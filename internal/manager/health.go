package manager

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
)

// Background checks that feed notifications:
//   - connections: every 5 minutes, the same test as the "Test connection"
//     button; reported after connDownAfter failures in a row, and again on
//     recovery;
//   - cloud destinations: every 30 minutes, the same probe as "Test";
//   - backup disk space, every 10 minutes (DISK_ALERT_PERCENT);
//   - schedules without a successful backup for too long (BACKUP_MAX_AGE).
// Backup size drops, restores and login lockouts are reported where they
// happen.

const (
	connCheckSpec    = "*/5 * * * *"
	destCheckSpec    = "*/30 * * * *"
	connDownAfter    = 2
	connCheckTimeout = 30 * time.Second
	destCheckTimeout = 2 * time.Minute
	// diskRecoverMargin keeps a disk near the threshold from flapping: it is
	// "fine" again only this many percentage points above it.
	diskRecoverMargin = 2.0
	// minShrinkBase skips the size comparison for tiny backups, whose
	// compressed size varies a lot.
	minShrinkBase = 1 << 20
)

// Variables so tests can stub the database, the cloud and the disk.
var (
	testConnection = func(ctx context.Context, target *Manager) error {
		return target.engine().test(ctx, target)
	}
	probeDestination = func(ctx context.Context, m *Manager, d Destination) error {
		return m.testDestination(ctx, d)
	}
	diskUsage = func(path string) (free, total uint64, err error) {
		var st syscall.Statfs_t
		if err = syscall.Statfs(path, &st); err != nil {
			return 0, 0, err
		}
		return uint64(st.Bavail) * uint64(st.Bsize), uint64(st.Blocks) * uint64(st.Bsize), nil
	}
)

// outage is the check state of one connection or destination.
type outage struct {
	Fails int
	Down  bool
	Since string // first failed check of the current outage
	Error string
}

type outageChange struct {
	ID        string
	Recovered bool
	State     outage
}

// trackOutages records one round of checks of the ids under prefix
// (errs[i] is "" when ids[i] passed) and forgets ids no longer checked. It
// returns the ids that just went down and those that just recovered.
func (m *Manager) trackOutages(prefix string, ids, errs []string) []outageChange {
	m.healthMu.Lock()
	defer m.healthMu.Unlock()
	checked := map[string]bool{}
	var changes []outageChange
	for i, id := range ids {
		key := prefix + id
		checked[key] = true
		h := m.health[key]
		if errs[i] != "" {
			if h.Fails == 0 {
				h.Since = now()
			}
			h.Fails++
			h.Error = errs[i]
			if !h.Down && h.Fails >= connDownAfter {
				h.Down = true
				changes = append(changes, outageChange{ID: id, State: h})
			}
		} else {
			if h.Down {
				changes = append(changes, outageChange{ID: id, Recovered: true, State: h})
			}
			h = outage{}
		}
		m.health[key] = h
	}
	for key := range m.health {
		if strings.HasPrefix(key, prefix) && !checked[key] {
			delete(m.health, key)
		}
	}
	return changes
}

// healthSnapshot returns the last check result of each id under prefix.
func (m *Manager) healthSnapshot(prefix string) ([]string, map[string]outage) {
	m.healthMu.Lock()
	defer m.healthMu.Unlock()
	ids := []string{}
	out := map[string]outage{}
	for key, h := range m.health {
		if id, ok := strings.CutPrefix(key, prefix); ok {
			ids = append(ids, id)
			out[id] = h
		}
	}
	sort.Strings(ids)
	return ids, out
}

// runChecks runs check(i) for every index in parallel; check returns the
// error text, "" on success.
func runChecks(n int, check func(i int) string) []string {
	errs := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = check(i)
		}()
	}
	wg.Wait()
	return errs
}

func errText(err error, redact func(string) string) string {
	if err == nil {
		return ""
	}
	if msg := redact(err.Error()); msg != "" {
		return msg
	}
	return "check failed"
}

// ---- connections ------------------------------------------------------------

// healthTargets lists the connections to check; "" is the connection from
// the environment, when one is configured.
func (m *Manager) healthTargets() ([]string, error) {
	rows, err := m.db.Query(`SELECT id FROM connections ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	if m.cfg.PGHost != "" {
		ids = append(ids, "")
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (m *Manager) checkConnections() {
	ids, err := m.healthTargets()
	if err != nil {
		m.logger.Print(err)
		return
	}
	errs := runChecks(len(ids), func(i int) string {
		target, err := m.forConnection(ids[i])
		if err != nil {
			return err.Error()
		}
		ctx, cancel := context.WithTimeout(m.ctx, connCheckTimeout)
		defer cancel()
		return errText(testConnection(ctx, target), target.redact)
	})
	if m.ctx.Err() != nil {
		return // shutting down; failures are not the database's fault
	}
	for _, c := range m.trackOutages("conn:", ids, errs) {
		ev := Event{Kind: eventConnection, ConnectionID: c.ID, Since: c.State.Since, Recovered: c.Recovered}
		if c.Recovered {
			m.audit("connection.up", m.connectionLabel(c.ID))
		} else {
			ev.Detail = c.State.Error
			m.audit("connection.down", m.connectionLabel(c.ID)+": "+truncate(c.State.Error, 300))
		}
		m.notify(ev)
	}
}

// ---- cloud destinations -----------------------------------------------------

func (m *Manager) checkDestinations() {
	list, err := m.destinations()
	if err != nil {
		m.logger.Print(err)
		return
	}
	ids := make([]string, len(list))
	for i, d := range list {
		ids[i] = d.ID
	}
	errs := runChecks(len(ids), func(i int) string {
		d, err := m.destination(ids[i])
		if err != nil {
			return err.Error()
		}
		ctx, cancel := context.WithTimeout(m.ctx, destCheckTimeout)
		defer cancel()
		return errText(probeDestination(ctx, m, d), func(s string) string { return s })
	})
	if m.ctx.Err() != nil {
		return
	}
	names := map[string]string{}
	for _, d := range list {
		names[d.ID] = d.Name
	}
	for _, c := range m.trackOutages("dest:", ids, errs) {
		ev := Event{Kind: eventDestination, DestinationID: c.ID, Target: names[c.ID], Since: c.State.Since, Recovered: c.Recovered}
		if c.Recovered {
			m.audit("destination.up", names[c.ID])
		} else {
			ev.Detail = c.State.Error
			m.audit("destination.down", names[c.ID]+": "+truncate(c.State.Error, 300))
		}
		m.notify(ev)
	}
}

// ---- disk space -------------------------------------------------------------

func (m *Manager) checkDisk() {
	if m.cfg.DiskAlertPercent <= 0 {
		return
	}
	free, total, err := diskUsage(m.cfg.BackupDir)
	if err != nil || total == 0 {
		return
	}
	pct := float64(free) * 100 / float64(total)
	m.healthMu.Lock()
	low := m.diskLow
	switch {
	case !low && pct < m.cfg.DiskAlertPercent:
		m.diskLow = true
	case low && pct >= m.cfg.DiskAlertPercent+diskRecoverMargin:
		m.diskLow = false
	}
	changed := low != m.diskLow
	m.healthMu.Unlock()
	if !changed {
		return
	}
	detail := fmt.Sprintf("%s / %s (%.1f%%)", formatBytes(clampInt64(free)), formatBytes(clampInt64(total)), pct)
	m.audit(map[bool]string{true: "disk.low", false: "disk.ok"}[!low], detail)
	m.notify(Event{Kind: eventDisk, Recovered: low, Detail: detail, Target: m.cfg.BackupDir})
}

// ---- backups that are too old -----------------------------------------------

// checkOverdue warns once when an enabled schedule has had no successful
// backup for longer than BACKUP_MAX_AGE, or one schedule interval plus
// JOB_TIMEOUT when that is longer (daily or weekly schedules: the last run had
// all its time and still produced nothing). Unlike "stale", it also catches
// jobs that run and keep failing.
func (m *Manager) checkOverdue() {
	if m.cfg.BackupMaxAge <= 0 {
		return
	}
	m.schedMu.Lock()
	ss, err := m.schedules()
	m.schedMu.Unlock()
	if err != nil {
		m.logger.Print(err)
		return
	}
	nowT := time.Now()
	current := map[string]bool{}
	var events []Event
	for _, s := range ss {
		if !s.Enabled {
			continue
		}
		key := s.ConnectionID + "|" + s.Database
		if _, seen := current[key]; seen {
			continue
		}
		spec, e := cronParse(m.timezone(), s.Cron)
		if e != nil {
			continue
		}
		next := spec.Next(nowT)
		limit := max(m.cfg.BackupMaxAge, spec.Next(next).Sub(next)+m.cfg.JobTimeout)
		var last string
		query, args := `SELECT COALESCE(MAX(created_at),'') FROM backups WHERE connection_id=? AND source IN ('scheduled','manual')`, []any{s.ConnectionID}
		if s.Database != "*" {
			query, args = query+` AND database_name=?`, append(args, s.Database)
		}
		if err = m.db.QueryRow(query, args...).Scan(&last); err != nil {
			continue
		}
		// A new or changed schedule gets a full period before it is overdue.
		ref := time.Time{}
		for _, v := range []string{last, s.UpdatedAt} {
			if t, e := time.Parse(time.RFC3339Nano, v); e == nil && t.After(ref) {
				ref = t
			}
		}
		overdue := !ref.IsZero() && nowT.Sub(ref) > limit
		current[key] = overdue
		if !overdue {
			continue
		}
		m.overdueMu.Lock()
		reported := m.overdue[key]
		m.overdueMu.Unlock()
		if !reported {
			events = append(events, Event{Kind: eventOverdue, ConnectionID: s.ConnectionID, Database: s.Database, Since: last, Detail: shortDuration(limit)})
		}
	}
	m.overdueMu.Lock()
	m.overdue = map[string]bool{}
	for key, overdue := range current {
		if overdue {
			m.overdue[key] = true
		}
	}
	m.overdueMu.Unlock()
	for _, ev := range events {
		m.audit("backup.overdue", ev.Database+" ("+m.connectionLabel(ev.ConnectionID)+")")
		m.notify(ev)
	}
}

// ---- backup size ------------------------------------------------------------

// checkBackupSize warns when a new backup is much smaller than the previous
// one of the same database: the dump "succeeded" but data may be missing.
func (m *Manager) checkBackupSize(j Job, backupID string) {
	if m.cfg.BackupShrinkPercent <= 0 {
		return
	}
	b, err := m.backup(backupID)
	if err != nil {
		return
	}
	var prev int64
	err = m.db.QueryRow(`SELECT size FROM backups WHERE connection_id=? AND database_name=? AND id<>? AND source IN ('scheduled','manual') ORDER BY created_at DESC LIMIT 1`, b.ConnectionID, b.Database, b.ID).Scan(&prev)
	if err != nil || prev < minShrinkBase {
		return
	}
	if float64(b.Size) >= float64(prev)*(1-float64(m.cfg.BackupShrinkPercent)/100) {
		return
	}
	m.audit("backup.shrunk", fmt.Sprintf("%s: %d -> %d bytes", b.Database, prev, b.Size))
	m.notify(Event{Kind: eventBackupSize, Job: j, ConnectionID: b.ConnectionID, Database: b.Database, Size: b.Size, PrevSize: prev})
}

func cronParse(tz, spec string) (cron.Schedule, error) {
	return cron.ParseStandard("CRON_TZ=" + tz + " " + spec)
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// shortDuration prints 24h instead of 24h0m0s.
func shortDuration(d time.Duration) string {
	s := d.Round(time.Minute).String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

// clampInt64 converts a byte count without wrapping to a negative number.
func clampInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

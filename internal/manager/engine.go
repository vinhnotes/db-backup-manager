package manager

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"golang.org/x/crypto/bcrypt"
)

var dbName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]{0,62}$`)
var idPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type Manager struct {
	cfg                   Config
	db                    *sql.DB
	logger                *log.Logger
	cron                  *cron.Cron
	ctx                   context.Context
	cancel                context.CancelFunc
	wg                    sync.WaitGroup
	queue                 chan Job
	opMu                  sync.Mutex
	queueMu               sync.Mutex
	schedMu               sync.Mutex
	tzMu                  sync.RWMutex
	tz                    string
	adminHash, viewerHash []byte
	loginMu               sync.Mutex
	attempts              map[string]attempt
	secretKey             []byte
	connectionID          string
	connectionMu          sync.Mutex
	pullMu                sync.Mutex
	pullSecrets           map[string]string
	googleOAuthMu         sync.Mutex
	googleOAuth           map[string]*googleOAuthFlow
	allowLocalRemote      bool
	started               time.Time
	staleMu               sync.Mutex
	stale                 map[string]staleState
	healthMu              sync.Mutex
	health                map[string]outage // "conn:<id>" and "dest:<id>"
	diskLow               bool              // guarded by healthMu
	overdueMu             sync.Mutex
	overdue               map[string]bool
	notifyFailures        atomic.Int64
	// notifyHook, when set (tests), receives every notification instead of
	// the configured channels.
	notifyHook func(Event)
}
type attempt struct {
	count int
	until time.Time
}
type Database struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func New(c Config, logger *log.Logger) (*Manager, error) {
	if err := os.MkdirAll(c.BackupDir, 0700); err != nil {
		return nil, err
	}
	db, err := openStore(c.MetadataDB)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	loc, _ := time.LoadLocation(c.Timezone)
	m := &Manager{cfg: c, tz: c.Timezone, db: db, logger: logger, ctx: ctx, cancel: cancel, queue: make(chan Job, 64), attempts: map[string]attempt{}, pullSecrets: map[string]string{}, started: time.Now(), stale: map[string]staleState{}, health: map[string]outage{}, overdue: map[string]bool{}, cron: cron.New(cron.WithLocation(loc), cron.WithChain(cron.Recover(cron.DefaultLogger), cron.SkipIfStillRunning(cron.DefaultLogger)))}
	if _, keyErr := os.Stat(filepath.Join(filepath.Dir(c.MetadataDB), "connections.key")); os.IsNotExist(keyErr) {
		var count int
		if err = db.QueryRow(`SELECT COUNT(*) FROM connections`).Scan(&count); err == nil && count > 0 {
			err = fmt.Errorf("connections.key is missing; restore the original key alongside metadata before starting")
		}
		if err != nil {
			cancel()
			db.Close()
			return nil, err
		}
	}
	m.secretKey, err = loadConnectionKey(c.MetadataDB)
	if err != nil {
		cancel()
		db.Close()
		return nil, err
	}
	m.adminHash, err = bcrypt.GenerateFromPassword([]byte(c.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		cancel()
		db.Close()
		return nil, err
	}
	if c.ViewerPassword != "" {
		m.viewerHash, _ = bcrypt.GenerateFromPassword([]byte(c.ViewerPassword), bcrypt.DefaultCost)
	}
	m.loadTimezoneSetting()
	if err = m.loadSchedules(); err != nil {
		cancel()
		db.Close()
		return nil, err
	}
	m.wg.Add(1)
	go m.worker()
	m.cron.Start()
	if c.CatchUp {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.catchUp()
		}()
	}
	return m, nil
}
func (m *Manager) Close() { m.cancel(); <-m.cron.Stop().Done(); m.wg.Wait(); m.db.Close() }
func token() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func (m *Manager) path(id string) string { return filepath.Join(m.cfg.BackupDir, id+".dump") }
func (m *Manager) pgEnv(restore bool) []string {
	out := baseEnv()
	user, password, passfile := m.cfg.PGUser, m.cfg.PGPassword, m.cfg.PGPassFile
	if restore {
		user, password, passfile = m.cfg.RestoreUser, m.cfg.RestorePassword, m.cfg.RestorePassFile
	}
	out = append(out, "PGHOST="+m.cfg.PGHost, "PGPORT="+m.cfg.PGPort, "PGUSER="+user, "PGSSLMODE="+m.cfg.PGSSLMode, "PGCONNECT_TIMEOUT=10", "PGAPPNAME=db-backup-manager")
	if password != "" {
		out = append(out, "PGPASSWORD="+password)
	}
	if passfile != "" {
		out = append(out, "PGPASSFILE="+passfile)
	}
	return out
}
func (m *Manager) connection(database string, restore bool) string {
	u := &url.URL{Scheme: "postgresql", Host: net.JoinHostPort(m.cfg.PGHost, m.cfg.PGPort), Path: "/" + database}
	user, password, passfile := m.cfg.PGUser, m.cfg.PGPassword, m.cfg.PGPassFile
	if restore {
		user, password, passfile = m.cfg.RestoreUser, m.cfg.RestorePassword, m.cfg.RestorePassFile
	}
	if password == "" && passfile != "" {
		password, _ = readPgpass(passfile, m.cfg.PGHost, m.cfg.PGPort, database, user)
	}
	if password != "" {
		u.User = url.UserPassword(user, password)
	} else {
		u.User = url.User(user)
	}
	q := u.Query()
	q.Set("sslmode", m.cfg.PGSSLMode)
	q.Set("connect_timeout", "10")
	u.RawQuery = q.Encode()
	return u.String()
}
func (m *Manager) connect(restore bool) (*sql.DB, error) {
	return openPostgres(m.connection(m.cfg.PGDatabase, restore))
}
func (m *Manager) databases(ctx context.Context) ([]Database, string, error) {
	if m.cfg.PGHost == "" {
		return []Database{}, "", nil
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	return m.engine().databases(ctx, m)
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len() < 16384 {
		left := 16384 - b.Len()
		if len(p) > left {
			p = p[:left]
		}
		b.Buffer.Write(p)
	}
	return n, nil
}
func (m *Manager) command(ctx context.Context, restore bool, name string, args ...string) error {
	return m.run(ctx, m.pgEnv(restore), nil, nil, name, args...)
}
func checksum(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, err
}
func (m *Manager) enqueue(kind, database, backupID string) (Job, error) {
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	return m.enqueueForLocked(kind, database, backupID, "")
}
func (m *Manager) enqueueLocked(kind, database, backupID string) (Job, error) {
	return m.enqueueForLocked(kind, database, backupID, "")
}
func (m *Manager) enqueueForLocked(kind, database, backupID, connectionID string) (Job, error) {
	return m.enqueueJobLocked(Job{ConnectionID: connectionID, Kind: kind, Database: database, BackupID: backupID})
}

// enqueueJobLocked persists and queues j; the caller holds queueMu.
func (m *Manager) enqueueJobLocked(j Job) (Job, error) {
	if j.ID == "" {
		j.ID = token()
	}
	j.Status, j.CreatedAt = "queued", now()
	if len(m.queue) == cap(m.queue) {
		return j, fmt.Errorf("job queue is full")
	}
	_, err := m.db.Exec(`INSERT INTO jobs(connection_id,id,kind,database_name,backup_id,destination_id,remote_path,status,created_at,attempt) VALUES(?,?,?,?,?,?,?,?,?,?)`, j.ConnectionID, j.ID, j.Kind, j.Database, j.BackupID, j.DestinationID, j.RemotePath, j.Status, j.CreatedAt, j.Attempt)
	if err != nil {
		return j, err
	}
	m.queue <- j
	m.audit("job.queued", j.Kind+" "+j.Database)
	return j, nil
}
func (m *Manager) worker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case j := <-m.queue:
			m.opMu.Lock()
			j.StartedAt = now()
			_, err := m.db.Exec(`UPDATE jobs SET status='running',started_at=? WHERE id=?`, j.StartedAt, j.ID)
			if err == nil {
				ctx, cancel := context.WithTimeout(m.ctx, m.cfg.JobTimeout)
				err = m.runJob(ctx, j)
				cancel()
			}
			m.pullMu.Lock()
			delete(m.pullSecrets, j.ID)
			m.pullMu.Unlock()
			if err != nil && m.retryable(j) {
				m.retryLater(j, err)
				m.opMu.Unlock()
				continue
			}
			status, message := "completed", ""
			if err != nil {
				status = "failed"
				message = err.Error()
				m.logger.Printf("job %s failed: %s", j.ID, message)
			}
			j.Status, j.Error, j.FinishedAt = status, message, now()
			if _, saveErr := m.db.Exec(`UPDATE jobs SET status=?,error=?,finished_at=? WHERE id=?`, status, message, j.FinishedAt, j.ID); saveErr != nil {
				m.logger.Printf("job result persistence failed: %v", saveErr)
			}
			m.audit("job."+status, j.Kind+" "+j.Database)
			m.notifyJob(j)
			m.opMu.Unlock()
		}
	}
}

// retryable reports whether a failed job is tried again. Restores are never
// retried (the target may be half-created), nor are cloud downloads (their
// passphrase is gone) or restore tests (a failure is the result).
func (m *Manager) retryable(j Job) bool {
	if m.ctx.Err() != nil || j.Attempt >= m.cfg.Retries {
		return false
	}
	switch j.Kind {
	case "backup", "scheduled", "offsite":
		return true
	}
	return false
}

// retryLater puts j back in the queue after a growing delay. The job stays
// "queued" meanwhile, so it still blocks deleting what it refers to.
func (m *Manager) retryLater(j Job, cause error) {
	delay := m.cfg.RetryDelay << j.Attempt
	j.Attempt++
	message := fmt.Sprintf("Lần thử %d lỗi, sẽ thử lại sau %s: %s", j.Attempt, delay, cause.Error())
	m.logger.Printf("job %s failed, retrying in %s: %v", j.ID, delay, cause)
	if _, err := m.db.Exec(`UPDATE jobs SET status='queued',error=?,attempt=? WHERE id=?`, message, j.Attempt, j.ID); err != nil {
		m.logger.Printf("job retry persistence failed: %v", err)
	}
	m.audit("job.retry", j.Kind+" "+j.Database)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-m.ctx.Done():
			return // startup marks the job failed
		case <-timer.C:
		}
		m.queueMu.Lock()
		defer m.queueMu.Unlock()
		if len(m.queue) == cap(m.queue) {
			j.Status, j.Error, j.FinishedAt = "failed", "Không thể thử lại: hàng đợi job đã đầy; "+message, now()
			m.db.Exec(`UPDATE jobs SET status='failed',error=?,finished_at=? WHERE id=?`, j.Error, j.FinishedAt, j.ID)
			m.notifyJob(j)
			return
		}
		m.queue <- j
	}()
}
func (m *Manager) runJob(ctx context.Context, j Job) error {
	switch j.Kind {
	case "retention":
		if err := m.retention(); err != nil {
			return err
		}
		return m.remoteRetention(ctx)
	case "offsite":
		return m.uploadOffsite(ctx, j)
	case "offsite_pull":
		return m.pullOffsite(ctx, j)
	}
	target, err := m.forConnection(j.ConnectionID)
	if err != nil {
		return err
	}
	switch j.Kind {
	case "backup", "scheduled":
		source := "manual"
		if j.Kind == "scheduled" {
			source = "scheduled"
		}
		id, err := target.createBackup(ctx, j.Database, source)
		if err == nil {
			m.checkBackupSize(j, id)
		}
		if err == nil && j.DestinationID != "" {
			m.chainOffsite(j, id)
		}
		if err == nil && j.Verify {
			m.chainVerify(j, id)
		}
		return err
	case "restore":
		return target.restore(ctx, j)
	case "verify":
		return target.verifyRestore(ctx, j)
	}
	return fmt.Errorf("unknown job kind %q", j.Kind)
}
func (m *Manager) createBackup(ctx context.Context, database, source string) (string, error) {
	if !dbName.MatchString(database) {
		return "", fmt.Errorf("unsupported database name")
	}
	if err := m.checkSpace(0); err != nil {
		return "", err
	}
	e := m.engine()
	id := token()
	partial := m.path(id) + ".partial"
	defer os.Remove(partial)
	if err := e.dump(ctx, m, database, partial); err != nil {
		return "", err
	}
	if err := e.verify(ctx, m, partial); err != nil {
		return "", err
	}
	sum, size, err := checksum(partial)
	if err != nil {
		return "", err
	}
	if size == 0 {
		return "", fmt.Errorf("empty backup")
	}
	if err = os.Rename(partial, m.path(id)); err != nil {
		return "", err
	}
	if err = m.saveBackup(Backup{m.connectionID, id, database, size, sum, now(), source, e.name()}); err != nil {
		os.Remove(m.path(id))
		return "", err
	}
	return id, nil
}
func (m *Manager) saveBackup(b Backup) error {
	b.Engine = engineName(b.Engine)
	_, err := m.db.Exec(`INSERT INTO backups(connection_id,id,database_name,size,checksum,created_at,source,engine) VALUES(?,?,?,?,?,?,?,?)`, b.ConnectionID, b.ID, b.Database, b.Size, b.Checksum, b.CreatedAt, b.Source, b.Engine)
	return err
}
func (m *Manager) restore(ctx context.Context, j Job) error {
	if m.cfg.RestoreUser == "" {
		return fmt.Errorf("RESTORE_PGUSER is not configured")
	}
	b, err := m.backup(j.BackupID)
	if err != nil {
		return err
	}
	e := m.engine()
	if engineName(b.Engine) != e.name() {
		return fmt.Errorf("Không thể khôi phục bản backup %s vào máy chủ %s", engineLabel(b.Engine), e.label())
	}
	sum, _, err := checksum(m.path(b.ID))
	if err != nil {
		return err
	}
	if sum != b.Checksum {
		return fmt.Errorf("backup checksum mismatch")
	}
	if err = e.verify(ctx, m, m.path(b.ID)); err != nil {
		return err
	}
	return e.restore(ctx, m, j.Database, b.Database, m.path(b.ID))
}
func (m *Manager) retention() error {
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	schedules, err := m.schedules()
	if err != nil {
		return err
	}
	backups, err := m.backups()
	if err != nil {
		return err
	}
	for _, b := range backups {
		if b.Source != "scheduled" {
			continue
		}
		days := 0
		for _, s := range schedules {
			if s.ConnectionID == b.ConnectionID && s.Enabled && (s.Database == "*" || s.Database == b.Database) && s.RetentionDays > days {
				days = s.RetentionDays
			}
		}
		created, err := time.Parse(time.RFC3339Nano, b.CreatedAt)
		if err != nil {
			return err
		}
		if days > 0 && time.Since(created) > time.Duration(days)*24*time.Hour {
			var pending int
			if err = m.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE backup_id=? AND status IN ('queued','running')`, b.ID).Scan(&pending); err != nil {
				return err
			}
			if pending == 0 {
				if err = m.removeBackup(b); err != nil {
					return err
				}
				m.audit("backup.retention_deleted", b.ID)
			}
		}
	}
	return nil
}
func (m *Manager) removeBackup(b Backup) error {
	if err := os.Remove(m.path(b.ID)); err != nil && !os.IsNotExist(err) {
		return err
	}
	_, err := m.db.Exec(`DELETE FROM backups WHERE id=?`, b.ID)
	return err
}
func (m *Manager) loadSchedules() error {
	for _, e := range m.cron.Entries() {
		m.cron.Remove(e.ID)
	}
	ss, err := m.schedules()
	if err != nil {
		return err
	}
	// CRON_TZ lets the timezone change at runtime without rebuilding the cron.
	prefix := "CRON_TZ=" + m.timezone() + " "
	for _, s := range ss {
		if !s.Enabled {
			continue
		}
		schedule := s
		if _, err = m.cron.AddFunc(prefix+s.Cron, func() { m.runSchedule(schedule) }); err != nil {
			return err
		}
	}
	_, err = m.cron.AddFunc(prefix+"17 3 * * *", func() {
		if _, e := m.enqueue("retention", "", ""); e != nil {
			m.logger.Print(e)
		}
	})
	if err != nil {
		return err
	}
	for spec, check := range map[string]func(){"*/10 * * * *": func() { m.checkStale(); m.checkOverdue(); m.checkDisk() }, connCheckSpec: m.checkConnections, destCheckSpec: m.checkDestinations} {
		if _, err = m.cron.AddFunc(spec, check); err != nil {
			return err
		}
	}
	return nil
}

// catchUp runs, once, every enabled schedule whose next run fell while the
// manager was stopped. Several missed runs still give a single backup.
func (m *Manager) catchUp() {
	m.schedMu.Lock()
	ss, err := m.schedules()
	m.schedMu.Unlock()
	if err != nil {
		m.logger.Print(err)
		return
	}
	for _, s := range ss {
		if m.ctx.Err() != nil {
			return
		}
		if !s.Enabled {
			continue
		}
		if next, ok := m.nextRunAfterLast(s); ok && next.Before(m.started) {
			m.audit("schedule.catch_up", s.Database+" "+s.Cron)
			m.runSchedule(s)
		}
	}
}

// nextRunAfterLast is the first run of s after its last run (or after it was
// last saved, if it never ran). ok is false when neither is known.
func (m *Manager) nextRunAfterLast(s Schedule) (time.Time, bool) {
	base := s.LastRunAt
	if base == "" {
		base = s.UpdatedAt
	}
	last, err := time.Parse(time.RFC3339Nano, base)
	if err != nil {
		return time.Time{}, false
	}
	spec, err := cron.ParseStandard("CRON_TZ=" + m.timezone() + " " + s.Cron)
	if err != nil {
		return time.Time{}, false
	}
	return spec.Next(last), true
}
func (m *Manager) runSchedule(s Schedule) {
	m.connectionMu.Lock()
	defer m.connectionMu.Unlock()
	if s.ID != 0 {
		if _, err := m.db.Exec(`UPDATE schedules SET last_run_at=? WHERE id=?`, now(), s.ID); err != nil {
			m.logger.Print(err)
		}
	}
	ctx, cancel := context.WithTimeout(m.ctx, 15*time.Second)
	defer cancel()
	target, err := m.forConnection(s.ConnectionID)
	if err != nil {
		m.recordScheduleFailure(s.Database, err, s.ConnectionID)
		return
	}
	names := []string{s.Database}
	if s.Database == "*" {
		dbs, _, err := target.databases(ctx)
		if err != nil {
			m.recordScheduleFailure(s.Database, err, s.ConnectionID)
			return
		}
		names = nil
		for _, d := range dbs {
			names = append(names, d.Name)
		}
	}
	for _, name := range names {
		m.queueMu.Lock()
		j := Job{ConnectionID: s.ConnectionID, Kind: "scheduled", Database: name, DestinationID: s.DestinationID, Verify: s.Verify, VerifyConnectionID: s.VerifyConnectionID}
		if j.VerifyConnectionID == "" {
			j.VerifyConnectionID = s.ConnectionID
		}
		_, err := m.enqueueJobLocked(j)
		m.queueMu.Unlock()
		if err != nil {
			m.recordScheduleFailure(name, err, s.ConnectionID)
		}
	}
	if _, err := m.enqueue("retention", "", ""); err != nil {
		m.logger.Print(err)
	}
}
func (m *Manager) recordScheduleFailure(database string, err error, connectionID string) {
	j := Job{ConnectionID: connectionID, ID: token(), Kind: "scheduled", Database: database, Status: "failed", Error: err.Error(), CreatedAt: now()}
	j.FinishedAt = j.CreatedAt
	_, e := m.db.Exec(`INSERT INTO jobs(connection_id,id,kind,database_name,backup_id,status,error,created_at,finished_at) VALUES(?,?,'scheduled',?,'','failed',?,?,?)`, connectionID, j.ID, database, j.Error, j.CreatedAt, j.FinishedAt)
	if e != nil {
		m.logger.Print(e)
	}
	m.logger.Print(err)
	m.notifyJob(j)
}
func writeJSON(w io.Writer, v any) { _ = json.NewEncoder(w).Encode(v) }

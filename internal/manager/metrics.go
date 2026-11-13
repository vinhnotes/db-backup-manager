package manager

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"syscall"
	"time"
)

// metrics serves GET /metrics in the Prometheus text format. It is off unless
// METRICS_TOKEN is set, and then needs "Authorization: Bearer <token>".
func (m *Manager) metrics(w http.ResponseWriter, r *http.Request) {
	if m.cfg.MetricsToken == "" {
		http.NotFound(w, r)
		return
	}
	got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(m.cfg.MetricsToken)) != 1 {
		w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
		http.Error(w, "Unauthorized", 401)
		return
	}
	var out metricWriter
	if err := m.writeMetrics(&out); err != nil {
		m.logger.Printf("metrics: %v", err)
		http.Error(w, "Metrics unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Write([]byte(out.String()))
}

type metricWriter struct{ strings.Builder }

// family writes the HELP and TYPE lines of a metric.
func (w *metricWriter) family(name, kind, help string) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, kind)
}

// sample writes one value; labels are name/value pairs.
func (w *metricWriter) sample(name string, value float64, labels ...string) {
	w.WriteString(name)
	if len(labels) > 0 {
		w.WriteByte('{')
		for i := 0; i+1 < len(labels); i += 2 {
			if i > 0 {
				w.WriteByte(',')
			}
			fmt.Fprintf(w, "%s=%q", labels[i], escapeLabel(labels[i+1]))
		}
		w.WriteByte('}')
	}
	fmt.Fprintf(w, " %g\n", value)
}

// escapeLabel leaves only what %q and Prometheus agree on: printable text.
func escapeLabel(v string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, v)
}

func unix(s string) (float64, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0, false
	}
	return float64(t.UnixNano()) / 1e9, true
}

func (m *Manager) writeMetrics(w *metricWriter) error {
	type conn struct{ name, engine string }
	conns := map[string]conn{}
	rows, err := m.db.Query(`SELECT id,name,engine FROM connections`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		var c conn
		if err = rows.Scan(&id, &c.name, &c.engine); err != nil {
			rows.Close()
			return err
		}
		c.engine = engineName(c.engine)
		conns[id] = c
	}
	rows.Close()
	labels := func(connID, database string) []string {
		return []string{"connection_id", connID, "connection", conns[connID].name, "database", database}
	}

	w.family("dbbm_up", "gauge", "1 when the manager and its metadata database respond.")
	w.sample("dbbm_up", 1)

	// Backups per connection and database.
	backups, err := m.backupViews()
	if err != nil {
		return err
	}
	type agg struct {
		count, bytes   float64
		last, lastSize float64
		verified       float64
		verifyOK       float64
		hasVerify      bool
	}
	per := map[[2]string]*agg{}
	for _, b := range backups { // newest first
		k := [2]string{b.ConnectionID, b.Database}
		a := per[k]
		if a == nil {
			a = &agg{}
			per[k] = a
			if ts, ok := unix(b.CreatedAt); ok {
				a.last, a.lastSize = ts, float64(b.Size)
			}
		}
		a.count++
		a.bytes += float64(b.Size)
		if ts, ok := unix(b.VerifiedAt); ok && ts > a.verified {
			a.verified, a.hasVerify = ts, true
			a.verifyOK = 0
			if b.VerifyStatus == "passed" {
				a.verifyOK = 1
			}
		}
	}
	keys := make([][2]string, 0, len(per))
	for k := range per {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i][0]+"|"+keys[i][1] < keys[j][0]+"|"+keys[j][1] })
	w.family("dbbm_backup_last_timestamp_seconds", "gauge", "Creation time of the newest stored backup.")
	for _, k := range keys {
		w.sample("dbbm_backup_last_timestamp_seconds", per[k].last, labels(k[0], k[1])...)
	}
	w.family("dbbm_backup_last_size_bytes", "gauge", "Size of the newest stored backup.")
	for _, k := range keys {
		w.sample("dbbm_backup_last_size_bytes", per[k].lastSize, labels(k[0], k[1])...)
	}
	w.family("dbbm_backups", "gauge", "Number of stored backups.")
	for _, k := range keys {
		w.sample("dbbm_backups", per[k].count, labels(k[0], k[1])...)
	}
	w.family("dbbm_backups_bytes", "gauge", "Total size of stored backups.")
	for _, k := range keys {
		w.sample("dbbm_backups_bytes", per[k].bytes, labels(k[0], k[1])...)
	}
	w.family("dbbm_verify_last_timestamp_seconds", "gauge", "Time of the latest restore test.")
	for _, k := range keys {
		if per[k].hasVerify {
			w.sample("dbbm_verify_last_timestamp_seconds", per[k].verified, labels(k[0], k[1])...)
		}
	}
	w.family("dbbm_verify_last_success", "gauge", "1 if the latest restore test passed, 0 if it failed.")
	for _, k := range keys {
		if per[k].hasVerify {
			w.sample("dbbm_verify_last_success", per[k].verifyOK, labels(k[0], k[1])...)
		}
	}

	// Backup jobs: last success, last failure and duration.
	type jobAgg struct{ success, failure, duration float64 }
	jobs := map[[2]string]*jobAgg{}
	rows, err = m.db.Query(`SELECT connection_id,database_name,status,started_at,finished_at FROM jobs WHERE kind IN ('backup','scheduled') AND status IN ('completed','failed') ORDER BY finished_at`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var c, d, status, started, finished string
		if err = rows.Scan(&c, &d, &status, &started, &finished); err != nil {
			rows.Close()
			return err
		}
		k := [2]string{c, d}
		a := jobs[k]
		if a == nil {
			a = &jobAgg{}
			jobs[k] = a
		}
		end, ok := unix(finished)
		if !ok {
			continue
		}
		if status == "failed" {
			a.failure = end
		} else {
			a.success = end
			if start, ok := unix(started); ok {
				a.duration = end - start
			}
		}
	}
	rows.Close()
	jobKeys := make([][2]string, 0, len(jobs))
	for k := range jobs {
		jobKeys = append(jobKeys, k)
	}
	sort.Slice(jobKeys, func(i, j int) bool { return jobKeys[i][0]+"|"+jobKeys[i][1] < jobKeys[j][0]+"|"+jobKeys[j][1] })
	w.family("dbbm_backup_last_success_timestamp_seconds", "gauge", "Finish time of the latest successful backup job.")
	for _, k := range jobKeys {
		if jobs[k].success > 0 {
			w.sample("dbbm_backup_last_success_timestamp_seconds", jobs[k].success, labels(k[0], k[1])...)
		}
	}
	w.family("dbbm_backup_last_failure_timestamp_seconds", "gauge", "Finish time of the latest failed backup job.")
	for _, k := range jobKeys {
		if jobs[k].failure > 0 {
			w.sample("dbbm_backup_last_failure_timestamp_seconds", jobs[k].failure, labels(k[0], k[1])...)
		}
	}
	w.family("dbbm_backup_last_duration_seconds", "gauge", "Run time of the latest successful backup job.")
	for _, k := range jobKeys {
		if jobs[k].success > 0 {
			w.sample("dbbm_backup_last_duration_seconds", jobs[k].duration, labels(k[0], k[1])...)
		}
	}

	// Jobs by kind and status.
	w.family("dbbm_jobs", "gauge", "Jobs in the history by kind and status.")
	rows, err = m.db.Query(`SELECT kind,status,COUNT(*) FROM jobs GROUP BY kind,status ORDER BY kind,status`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var kind, status string
		var n float64
		if err = rows.Scan(&kind, &status, &n); err != nil {
			rows.Close()
			return err
		}
		w.sample("dbbm_jobs", n, "kind", kind, "status", status)
	}
	rows.Close()
	w.family("dbbm_queue_length", "gauge", "Jobs waiting in the worker queue.")
	w.sample("dbbm_queue_length", float64(len(m.queue)))

	// Schedules.
	ss, err := m.schedules()
	if err != nil {
		return err
	}
	enabled := 0
	for _, s := range ss {
		if s.Enabled {
			enabled++
		}
	}
	w.family("dbbm_schedules", "gauge", "Backup schedules by state.")
	w.sample("dbbm_schedules", float64(enabled), "enabled", "true")
	w.sample("dbbm_schedules", float64(len(ss)-enabled), "enabled", "false")
	w.family("dbbm_schedule_stale", "gauge", "1 for each schedule whose last due run created no job.")
	m.staleMu.Lock()
	stale := make([]staleState, 0, len(m.stale))
	for _, st := range m.stale {
		stale = append(stale, st)
	}
	m.staleMu.Unlock()
	sort.Slice(stale, func(i, j int) bool {
		return stale[i].ConnectionID+"|"+stale[i].Database < stale[j].ConnectionID+"|"+stale[j].Database
	})
	for _, st := range stale {
		w.sample("dbbm_schedule_stale", 1, labels(st.ConnectionID, st.Database)...)
	}
	w.family("dbbm_connection_up", "gauge", "1 when the last health check of a connection passed, 0 when it failed.")
	ids, health := m.healthSnapshot("conn:")
	for _, id := range ids {
		w.sample("dbbm_connection_up", upValue(health[id]), "connection_id", id, "connection", conns[id].name)
	}

	// Cloud copies.
	w.family("dbbm_offsite_last_upload_timestamp_seconds", "gauge", "Time of the latest upload to each cloud destination.")
	destNames := map[string]string{}
	rows, err = m.db.Query(`SELECT d.id,d.name,COALESCE(MAX(o.uploaded_at),'') FROM destinations d LEFT JOIN offsite o ON o.destination_id=d.id GROUP BY d.id,d.name ORDER BY d.name`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, name, last string
		if err = rows.Scan(&id, &name, &last); err != nil {
			rows.Close()
			return err
		}
		destNames[id] = name
		if ts, ok := unix(last); ok {
			w.sample("dbbm_offsite_last_upload_timestamp_seconds", ts, "destination_id", id, "destination", name)
		}
	}
	rows.Close()
	w.family("dbbm_destination_up", "gauge", "1 when the last check of a cloud destination passed, 0 when it failed.")
	destIDs, destHealth := m.healthSnapshot("dest:")
	for _, id := range destIDs {
		w.sample("dbbm_destination_up", upValue(destHealth[id]), "destination_id", id, "destination", destNames[id])
	}

	// Disk and notifications.
	var st syscall.Statfs_t
	if syscall.Statfs(m.cfg.BackupDir, &st) == nil {
		w.family("dbbm_backup_disk_free_bytes", "gauge", "Free space for BACKUP_DIR.")
		w.sample("dbbm_backup_disk_free_bytes", float64(statfsBytes(st.Bavail, int64(st.Bsize))))
		w.family("dbbm_backup_disk_size_bytes", "gauge", "Size of the file system holding BACKUP_DIR.")
		w.sample("dbbm_backup_disk_size_bytes", float64(statfsBytes(st.Blocks, int64(st.Bsize))))
	}
	w.family("dbbm_notification_failures_total", "counter", "Notifications that could not be delivered since start.")
	w.sample("dbbm_notification_failures_total", float64(m.notifyFailures.Load()))
	return nil
}

func upValue(h outage) float64 {
	if h.Fails > 0 {
		return 0
	}
	return 1
}

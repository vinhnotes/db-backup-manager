package manager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
)

// Uses an isolated local PostgreSQL instance only. Never point at production.
func TestPostgresBackupRestoreIntegration(t *testing.T) {
	port := os.Getenv("TEST_PGPORT")
	if port == "" {
		t.Skip("set TEST_PGPORT to opt in to isolated PostgreSQL integration test")
	}
	m := testManager(t)
	m.cfg.PGPort = port
	m.cfg.PGPassword = "integration-only-password"
	m.cfg.RestoreUser = "postgres"
	m.cfg.RestorePassword = m.cfg.PGPassword
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := m.connect(true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	source := "test_source_" + token()[:8]
	target := "test_target_" + token()[:8]
	for _, name := range []string{source, target} {
		n := name
		t.Cleanup(func() { admin.Exec(`DROP DATABASE IF EXISTS ` + pq.QuoteIdentifier(n) + ` WITH (FORCE)`) })
	}
	if _, err = admin.ExecContext(ctx, `CREATE DATABASE `+pq.QuoteIdentifier(source)); err != nil {
		t.Fatal(err)
	}
	db, err := openPostgres(m.connection(source, false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `CREATE TABLE example(id int PRIMARY KEY, message text); INSERT INTO example VALUES(1,'xin chào'),(2,'backup verified')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	cookie := loginCookie(t, m, "admin", "test-admin-password")
	w := request(m, "POST", "/api/backups", `{"database":"`+source+`"}`, cookie)
	if w.Code != 202 {
		t.Fatalf("enqueue backup: %s", w.Body)
	}
	var js []Job
	if err = json.Unmarshal(w.Body.Bytes(), &js); err != nil {
		t.Fatal(err)
	}
	waitJob(t, m, js[0].ID)
	bb, err := m.backups()
	if err != nil || len(bb) != 1 {
		t.Fatalf("missing backup: %v", err)
	}
	b := bb[0]
	sum, n, err := checksum(m.path(b.ID))
	if err != nil || sum != b.Checksum || n < 1 {
		t.Fatal("invalid archive checksum")
	}
	w = request(m, "POST", "/api/restore", `{"backup_id":"`+b.ID+`","target":"`+target+`","confirmation":"`+target+`","trusted":true}`, cookie)
	if w.Code != 202 {
		t.Fatalf("enqueue restore: %s", w.Body)
	}
	var restoreJob Job
	json.Unmarshal(w.Body.Bytes(), &restoreJob)
	waitJob(t, m, restoreJob.ID)
	restored, err := openPostgres(m.connection(target, true))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var count int
	var message string
	if err = restored.QueryRowContext(ctx, `SELECT count(*),min(message) FROM example`).Scan(&count, &message); err != nil || count != 2 {
		t.Fatalf("restored data mismatch: %d %s %v", count, message, err)
	}
	err = m.restore(ctx, Job{BackupID: b.ID, Database: target})
	if err == nil || !strings.Contains(err.Error(), "cannot create target") {
		t.Fatalf("existing target was accepted: %v", err)
	}
	if err = restored.QueryRowContext(ctx, `SELECT count(*) FROM example`).Scan(&count); err != nil || count != 2 {
		t.Fatal("existing target changed")
	}
	// Restore test: restores into a temporary database, then drops it.
	w = request(m, "POST", "/api/backups/"+b.ID+"/verify", `{}`, cookie)
	if w.Code != 202 {
		t.Fatalf("enqueue restore test: %s", w.Body)
	}
	var verifyJob Job
	json.Unmarshal(w.Body.Bytes(), &verifyJob)
	waitJob(t, m, verifyJob.ID)
	views, _ := m.backupViews()
	if len(views) != 1 || views[0].VerifyStatus != "passed" || !strings.Contains(views[0].VerifyDetail, "1 ") {
		t.Fatalf("restore test result: %+v", views)
	}
	var leftovers int
	if err = admin.QueryRowContext(ctx, `SELECT count(*) FROM pg_database WHERE datname LIKE 'dbbm_verify_%'`).Scan(&leftovers); err != nil || leftovers != 0 {
		t.Fatalf("temporary database left behind: %d %v", leftovers, err)
	}
	// Exercise the raw upload path and downloads using a real pg_dump archive.
	f, err := os.Open(m.path(b.ID))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.Name())
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	m.cfg.MaxUpload = int64(len(data) + 1)
	r := newUploadRequest(cookie, source, string(data))
	ww := newRecorder()
	m.Handler().ServeHTTP(ww, r)
	if ww.Code != 201 {
		t.Fatalf("upload: %d %s", ww.Code, ww.Body)
	}
	var uploaded Backup
	json.Unmarshal(ww.Body.Bytes(), &uploaded)
	if uploaded.Checksum != b.Checksum {
		t.Fatal("uploaded checksum differs")
	}
	w = request(m, "GET", "/api/backups/"+uploaded.ID+"/download", "", cookie)
	if w.Code != 200 || w.Body.String() != string(data) {
		t.Fatal("download differs")
	}
	w = request(m, "DELETE", "/api/backups/"+uploaded.ID, "", cookie)
	if w.Code != 200 {
		t.Fatalf("delete: %s", w.Body)
	}
	// Scheduler callback produces real scheduled jobs and retention runs afterward.
	m.runSchedule(Schedule{Database: source})
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		jobs, _ := m.jobs()
		found := false
		for _, j := range jobs {
			if j.Kind == "scheduled" {
				waitJob(t, m, j.ID)
				found = true
				break
			}
		}
		if found {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("scheduler did not enqueue job")
}
func waitJob(t *testing.T, m *Manager, id string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		var status, message string
		if err := m.db.QueryRow(`SELECT status,error FROM jobs WHERE id=?`, id).Scan(&status, &message); err != nil {
			t.Fatal(err)
		}
		if status == "failed" {
			t.Fatalf("job failed: %s", message)
		}
		if status == "completed" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("job timed out")
}

func newUploadRequest(cookie *http.Cookie, database, data string) *http.Request {
	r := httptest.NewRequest("POST", "/api/upload", strings.NewReader(data))
	r.AddCookie(cookie)
	r.Header.Set("X-Requested-With", "DBBackupManager")
	r.Header.Set("X-Database-Name", database)
	return r
}
func newRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }

func TestUIConnectionPostgresIntegration(t *testing.T) {
	port := os.Getenv("TEST_PGPORT")
	if port == "" {
		t.Skip("set TEST_PGPORT to opt in to isolated PostgreSQL integration test")
	}
	m := testManager(t)
	cookie := loginCookie(t, m, "admin", "test-admin-password")
	payload := `{"name":"UI configured server","host":"127.0.0.1","port":"` + port + `","username":"postgres","password":"integration-only-password","database":"postgres","sslmode":"disable","restore_username":"postgres","restore_password":"integration-only-password"}`
	if w := request(m, "POST", "/api/connections/test", payload, cookie); w.Code != 200 {
		t.Fatalf("test connection: %s", w.Body)
	}
	w := request(m, "POST", "/api/connections", payload, cookie)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	var c Connection
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	// Deliberately keep the legacy port wrong: all operations must use the saved connection.
	target, err := m.forConnection(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	db, err := target.connect(false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	source := "ui_source_" + token()[:8]
	restoredName := "ui_restored_" + token()[:8]
	for _, name := range []string{source, restoredName} {
		n := name
		t.Cleanup(func() { db.Exec(`DROP DATABASE IF EXISTS ` + pq.QuoteIdentifier(n) + ` WITH (FORCE)`) })
	}
	if _, err = db.Exec(`CREATE DATABASE ` + pq.QuoteIdentifier(source)); err != nil {
		t.Fatal(err)
	}
	sourceDB, err := openPostgres(target.connection(source, false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sourceDB.Exec(`CREATE TABLE sample (value text); INSERT INTO sample VALUES ('saved connection works')`); err != nil {
		t.Fatal(err)
	}
	sourceDB.Close()
	w = request(m, "GET", "/api/databases?connection_id="+c.ID, "", cookie)
	if w.Code != 200 || !strings.Contains(w.Body.String(), source) {
		t.Fatalf("list scoped databases: %s", w.Body)
	}
	w = request(m, "POST", "/api/backups", `{"database":"`+source+`","connection_id":"`+c.ID+`"}`, cookie)
	if w.Code != 202 {
		t.Fatal(w.Body)
	}
	var jobs []Job
	if err = json.Unmarshal(w.Body.Bytes(), &jobs); err != nil || len(jobs) != 1 || jobs[0].ConnectionID != c.ID {
		t.Fatal("job lost connection")
	}
	waitJob(t, m, jobs[0].ID)
	backups, err := m.backups()
	if err != nil || len(backups) != 1 || backups[0].ConnectionID != c.ID {
		t.Fatal("backup lost connection")
	}
	w = request(m, "POST", "/api/restore", `{"backup_id":"`+backups[0].ID+`","connection_id":"`+c.ID+`","target":"`+restoredName+`","confirmation":"`+restoredName+`","trusted":true}`, cookie)
	if w.Code != 202 {
		t.Fatalf("restore via saved connection: %s", w.Body)
	}
	var restoreJob Job
	if err = json.Unmarshal(w.Body.Bytes(), &restoreJob); err != nil || restoreJob.ConnectionID != c.ID {
		t.Fatal("restore lost target connection")
	}
	waitJob(t, m, restoreJob.ID)
	restoredDB, err := openPostgres(target.connection(restoredName, true))
	if err != nil {
		t.Fatal(err)
	}
	defer restoredDB.Close()
	var value string
	if err = restoredDB.QueryRow("SELECT value FROM sample").Scan(&value); err != nil || value != "saved connection works" {
		t.Fatalf("restored data mismatch: %q %v", value, err)
	}
	m.runSchedule(Schedule{ConnectionID: c.ID, Database: source})
	all, err := m.jobs()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, scheduled := range all {
		if scheduled.Kind == "scheduled" {
			if scheduled.ConnectionID != c.ID {
				t.Fatal("scheduled job lost connection")
			}
			waitJob(t, m, scheduled.ID)
			found = true
		}
	}
	if !found {
		t.Fatal("no scheduled job")
	}
}

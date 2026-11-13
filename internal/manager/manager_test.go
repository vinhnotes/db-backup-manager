package manager

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	c := Config{BackupDir: filepath.Join(dir, "backups"), MetadataDB: filepath.Join(dir, "meta.db"), WebDir: dir, AdminUser: "admin", AdminPassword: "test-admin-password", ViewerPassword: "test-viewer-password", Timezone: "Asia/Ho_Chi_Minh", JobTimeout: time.Minute, MaxUpload: 1024, PGHost: "127.0.0.1", PGPort: "5432", PGDatabase: "postgres", PGUser: "postgres", PGSSLMode: "disable", RestoreUser: "restore_user"}
	m, err := New(c, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m
}
func request(m *Manager, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("X-Requested-With", "DBBackupManager")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, r)
	return w
}
func loginCookie(t *testing.T, m *Manager, user, password string) *http.Cookie {
	t.Helper()
	w := request(m, "POST", "/api/login", `{"username":"`+user+`","password":"`+password+`"}`, nil)
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body)
	}
	return w.Result().Cookies()[0]
}
func TestAuthenticationAndPermissions(t *testing.T) {
	m := testManager(t)
	if w := request(m, "GET", "/api/backups", "", nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("missing CSRF header accepted")
	}
	admin := loginCookie(t, m, "admin", "test-admin-password")
	viewer := loginCookie(t, m, "viewer", "test-viewer-password")
	if !admin.HttpOnly || admin.SameSite != http.SameSiteStrictMode {
		t.Fatal("insecure session cookie")
	}
	if w = request(m, "GET", "/api/backups", "", viewer); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = request(m, "POST", "/api/backups", `{"database":"*"}`, viewer); w.Code != 403 {
		t.Fatal("viewer mutation accepted")
	}
	r = httptest.NewRequest("POST", "/api/logout", nil)
	r.AddCookie(admin)
	r.Header.Set("X-Requested-With", "DBBackupManager")
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	m.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin request accepted")
	}
	if w = request(m, "POST", "/api/logout", "", admin); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = request(m, "GET", "/api/me", "", admin); w.Code != 401 {
		t.Fatal("revoked session accepted")
	}
}
func TestRestoreGuards(t *testing.T) {
	m := testManager(t)
	cookie := loginCookie(t, m, "admin", "test-admin-password")
	for _, body := range []string{`{"target":"prod","confirmation":"wrong","trusted":true}`, `{"target":"postgres","confirmation":"postgres","trusted":true}`, `{"target":"test; DROP DATABASE prod","confirmation":"test; DROP DATABASE prod","trusted":true}`, `{"target":"target","confirmation":"target","trusted":false}`} {
		if w := request(m, "POST", "/api/restore", body, cookie); w.Code != 400 {
			t.Fatalf("guard failed: %d %s", w.Code, body)
		}
	}
}
func TestSchedulePersistenceAndValidation(t *testing.T) {
	m := testManager(t)
	cookie := loginCookie(t, m, "admin", "test-admin-password")
	for _, body := range []string{`{"database":"*","cron":"@every 1s","retention_days":14,"enabled":true}`, `{"database":"*","cron":"0 2 * * *","retention_days":0,"enabled":true}`} {
		if w := request(m, "POST", "/api/schedules", body, cookie); w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
	w := request(m, "POST", "/api/schedules", `{"database":"*","cron":"0 2 * * *","retention_days":14,"enabled":true}`, cookie)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	w = request(m, "GET", "/api/schedules", "", cookie)
	var ss []Schedule
	if err := json.Unmarshal(w.Body.Bytes(), &ss); err != nil || len(ss) != 1 || ss[0].NextRun == "" {
		t.Fatalf("bad schedules %s", w.Body)
	}
	if w = request(m, "DELETE", "/api/schedules/1", "", cookie); w.Code != 200 {
		t.Fatal(w.Code)
	}
}
func TestRetentionPreservesManualAndPendingRestore(t *testing.T) {
	m := testManager(t)
	_, err := m.db.Exec(`INSERT INTO schedules(database_name,cron,retention_days,enabled) VALUES('app','0 2 * * *',14,1)`)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, source := range []string{"manual", "upload", "scheduled", "protected"} {
		id := token()
		ids[source] = id
		actual := source
		if source == "protected" {
			actual = "scheduled"
		}
		b := Backup{"", id, "app", 10, "hash", time.Now().Add(-40 * 24 * time.Hour).Format(time.RFC3339Nano), actual, ""}
		if err = m.saveBackup(b); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(m.path(id), []byte("backup"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, err = m.db.Exec(`INSERT INTO jobs(id,kind,database_name,backup_id,status,created_at) VALUES(?,'restore','newdb',?,'queued',?)`, token(), ids["protected"], now())
	if err != nil {
		t.Fatal(err)
	}
	if err = m.retention(); err != nil {
		t.Fatal(err)
	}
	bb, err := m.backups()
	if err != nil || len(bb) != 3 {
		t.Fatalf("retention deleted wrong backups: %v %v", bb, err)
	}
	if _, err = os.Stat(m.path(ids["scheduled"])); !os.IsNotExist(err) {
		t.Fatal("expired file remains")
	}
}
func TestInterruptedJobsAreMarkedFailed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.db")
	db, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO jobs(id,kind,database_name,backup_id,status,created_at) VALUES('interrupted','restore','newdb','','running',?)`, now())
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var status string
	if err = db.QueryRow(`SELECT status FROM jobs WHERE id='interrupted'`).Scan(&status); err != nil || status != "failed" {
		t.Fatal("interrupted job not recovered")
	}
}
func TestPgpassEscapingAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pgpass")
	os.WriteFile(path, []byte("# comment\nlocalhost:5432:*:backup:p\\:a\\\\ss\n"), 0600)
	pass, err := readPgpass(path, "localhost", "5432", "app", "backup")
	if err != nil || pass != `p:a\ss` {
		t.Fatalf("%q %v", pass, err)
	}
	os.Chmod(path, 0644)
	if _, err = readPgpass(path, "localhost", "5432", "app", "backup"); err == nil {
		t.Fatal("accepted world-readable pgpass")
	}
}
func TestChecksumTamperingBlocksRestore(t *testing.T) {
	m := testManager(t)
	id := token()
	os.WriteFile(m.path(id), []byte("tampered"), 0600)
	if err := m.saveBackup(Backup{"", id, "app", 5, "wrong", now(), "manual", ""}); err != nil {
		t.Fatal(err)
	}
	err := m.restore(context.Background(), Job{BackupID: id, Database: "target"})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("tampered backup not rejected: %v", err)
	}
}

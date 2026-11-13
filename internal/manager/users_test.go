package manager

import (
	"encoding/json"
	"io"
	"log"
	"strings"
	"testing"
	"time"
)

func TestUserPermissions(t *testing.T) {
	m := testManager(t)
	admin := loginCookie(t, m, "admin", "test-admin-password")
	for _, name := range []string{"app_db", "other_db"} {
		if err := m.saveBackup(Backup{"", token(), name, 1, "sum", now(), "manual", ""}); err != nil {
			t.Fatal(err)
		}
	}

	for _, body := range []string{
		`{"username":"admin","password":"long-enough-pass","role":"user","permissions":[]}`,
		`{"username":"Viewer","password":"long-enough-pass","role":"user","permissions":[]}`,
		`{"username":"alice","password":"short","role":"user","permissions":[]}`,
		`{"username":"alice","password":"long-enough-pass","role":"root","permissions":[]}`,
		`{"username":"alice","password":"long-enough-pass","role":"user","permissions":[{"connection_id":"ffffffffffffffffffffffffffffffff","database":"*","level":"view"}]}`,
	} {
		if w := request(m, "POST", "/api/users", body, admin); w.Code != 400 {
			t.Fatalf("invalid user accepted: %d %s", w.Code, body)
		}
	}
	w := request(m, "POST", "/api/users", `{"username":"alice","display_name":"Alice","password":"alice-password-1","role":"user","permissions":[{"connection_id":"","database":"app_db","level":"view"}]}`, admin)
	if w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var u User
	json.Unmarshal(w.Body.Bytes(), &u)
	if w = request(m, "POST", "/api/users", `{"username":"ALICE","password":"alice-password-1","role":"user","permissions":[]}`, admin); w.Code != 409 {
		t.Fatalf("duplicate username: %d", w.Code)
	}

	alice := loginCookie(t, m, "ALICE", "alice-password-1")
	if w = request(m, "GET", "/api/users", "", alice); w.Code != 403 {
		t.Fatal("non-admin listed users")
	}
	if w = request(m, "POST", "/api/connections", `{}`, alice); w.Code != 403 {
		t.Fatal("non-admin reached connection API")
	}
	w = request(m, "GET", "/api/backups", "", alice)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "app_db") || strings.Contains(w.Body.String(), "other_db") {
		t.Fatalf("backups not scoped: %s", w.Body)
	}
	var backups []Backup
	json.Unmarshal(w.Body.Bytes(), &backups)
	if w = request(m, "GET", "/api/backups/"+backups[0].ID+"/download", "", alice); w.Code != 403 {
		t.Fatalf("view-only download: %d", w.Code)
	}
	if w = request(m, "POST", "/api/backups", `{"database":"app_db","connection_id":""}`, alice); w.Code != 403 {
		t.Fatalf("view-only backup: %d", w.Code)
	}

	// Upgrade to operate on app_db only.
	if w = request(m, "PUT", "/api/users/"+u.ID, `{"username":"alice","password":"","role":"user","permissions":[{"connection_id":"","database":"app_db","level":"operate"}]}`, admin); w.Code != 200 {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	for _, body := range []string{`{"database":"other_db","connection_id":""}`, `{"database":"*","connection_id":""}`} {
		if w = request(m, "POST", "/api/backups", body, alice); w.Code != 403 {
			t.Fatalf("out-of-scope backup %s: %d", body, w.Code)
		}
	}
	if w = request(m, "POST", "/api/schedules", `{"database":"other_db","connection_id":"","cron":"0 2 * * *","retention_days":3,"enabled":true}`, alice); w.Code != 403 {
		t.Fatalf("out-of-scope schedule: %d", w.Code)
	}
	if w = request(m, "GET", "/api/backups/"+backups[0].ID+"/download", "", alice); w.Code == 403 || w.Code == 401 {
		t.Fatalf("operate download denied: %d", w.Code)
	}

	// Password change keeps the current session.
	if w = request(m, "POST", "/api/me/password", `{"current_password":"wrong-password","new_password":"alice-password-2"}`, alice); w.Code != 400 {
		t.Fatal("wrong current password accepted")
	}
	if w = request(m, "POST", "/api/me/password", `{"current_password":"alice-password-1","new_password":"alice-password-2"}`, alice); w.Code != 200 {
		t.Fatalf("change password: %d %s", w.Code, w.Body)
	}
	if w = request(m, "GET", "/api/me", "", alice); w.Code != 200 || !strings.Contains(w.Body.String(), `"username":"alice"`) {
		t.Fatalf("me: %d %s", w.Code, w.Body)
	}

	// Disabling revokes access immediately.
	if w = request(m, "PUT", "/api/users/"+u.ID, `{"username":"alice","password":"","role":"user","disabled":true,"permissions":[]}`, admin); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = request(m, "GET", "/api/me", "", alice); w.Code != 401 {
		t.Fatal("disabled user kept session")
	}
	if w = request(m, "POST", "/api/login", `{"username":"alice","password":"alice-password-2"}`, nil); w.Code != 401 {
		t.Fatal("disabled user logged in")
	}
	if w = request(m, "DELETE", "/api/users/"+u.ID, "", admin); w.Code != 200 {
		t.Fatal(w.Code)
	}
}

func TestAdminPasswordChange(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{BackupDir: dir + "/backups", MetadataDB: dir + "/meta.db", WebDir: dir, AdminUser: "admin", AdminPassword: "env-admin-password", Timezone: "Asia/Ho_Chi_Minh", JobTimeout: time.Minute, MaxUpload: 1024}
	start := func(c Config) *Manager {
		m, err := New(c, log.New(io.Discard, "", 0))
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	m := start(cfg)
	admin := loginCookie(t, m, "admin", "env-admin-password")
	other := loginCookie(t, m, "admin", "env-admin-password")
	if w := request(m, "POST", "/api/me/password", `{"current_password":"wrong-password!","new_password":"web-admin-password"}`, admin); w.Code != 400 {
		t.Fatalf("wrong current password: %d", w.Code)
	}
	if w := request(m, "POST", "/api/me/password", `{"current_password":"env-admin-password","new_password":"web-admin-password"}`, admin); w.Code != 200 {
		t.Fatalf("change: %d %s", w.Code, w.Body)
	}
	if w := request(m, "GET", "/api/me", "", admin); w.Code != 200 {
		t.Fatal("current session dropped")
	}
	if w := request(m, "GET", "/api/me", "", other); w.Code != 401 {
		t.Fatal("other session kept")
	}
	if w := request(m, "POST", "/api/login", `{"username":"admin","password":"env-admin-password"}`, nil); w.Code != 401 {
		t.Fatal("old password still works")
	}
	loginCookie(t, m, "admin", "web-admin-password")
	m.Close()

	// Survives a restart, even when ADMIN_PASSWORD in .env changes.
	cfg.AdminPassword = "new-env-admin-password"
	m = start(cfg)
	defer m.Close()
	if w := request(m, "POST", "/api/login", `{"username":"admin","password":"new-env-admin-password"}`, nil); w.Code != 401 {
		t.Fatal(".env change reset the password")
	}
	session := loginCookie(t, m, "admin", "web-admin-password")

	// The reset command applies while the server runs and signs admins out.
	if err := ResetAdminPassword(cfg.MetadataDB); err != nil {
		t.Fatal(err)
	}
	if w := request(m, "GET", "/api/me", "", session); w.Code != 401 {
		t.Fatal("admin session kept after reset")
	}
	if w := request(m, "POST", "/api/login", `{"username":"admin","password":"web-admin-password"}`, nil); w.Code != 401 {
		t.Fatal("web password still works after reset")
	}
	loginCookie(t, m, "admin", "new-env-admin-password")
	if err := ResetAdminPassword(dir + "/missing.db"); err == nil {
		t.Fatal("reset created a new metadata database")
	}
}

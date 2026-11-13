package manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConnectionCRUDAndSecrets(t *testing.T) {
	m := testManager(t)
	admin := loginCookie(t, m, "admin", "test-admin-password")
	viewer := loginCookie(t, m, "viewer", "test-viewer-password")
	body := `{"name":"Production","host":"db.internal","port":"5432","username":"backup","password":"private-secret-123","database":"postgres","sslmode":"require","restore_username":"restore_operator","restore_password":"restore-private-456"}`
	if w := request(m, "POST", "/api/connections", body, viewer); w.Code != 403 {
		t.Fatal("viewer can create connections")
	}
	w := request(m, "POST", "/api/connections", body, admin)
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	var c Connection
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if c.ID == "" || !c.HasPassword || (strings.Contains(w.Body.String(), "private-secret") || strings.Contains(w.Body.String(), "restore-private")) {
		t.Fatalf("unsafe response: %s", w.Body)
	}
	var stored string
	if err := m.db.QueryRow(`SELECT password FROM connections WHERE id=?`, c.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == "private-secret-123" || strings.Contains(stored, "private-secret") {
		t.Fatal("plaintext credential persisted")
	}
	key, err := loadConnectionKey(m.cfg.MetadataDB)
	if err != nil || string(key) != string(m.secretKey) {
		t.Fatal("encryption key did not persist")
	}
	c.Password = ""
	c.HasPassword = true
	c.Name = "Updated"
	c.RestorePassword = ""
	c.HasRestorePassword = true
	data, _ := json.Marshal(c)
	if w = request(m, "PUT", "/api/connections/"+c.ID, string(data), admin); w.Code != 200 {
		t.Fatal(w.Body)
	}
	saved, err := m.storedConnection(c.ID)
	if err != nil || saved.Password != "private-secret-123" || saved.Name != "Updated" || saved.RestorePassword != "restore-private-456" || !saved.RestoreEnabled {
		t.Fatal("blank password update lost credentials")
	}
	if w = request(m, "GET", "/api/connections", "", viewer); w.Code != 200 || (strings.Contains(w.Body.String(), "private-secret") || strings.Contains(w.Body.String(), "restore-private")) {
		t.Fatalf("list: %s", w.Body)
	}
	// Scoped manager must never inherit legacy passwords or pgpass paths.
	m.cfg.PGPassFile = "/legacy/pgpass"
	target, err := m.forConnection(c.ID)
	if err != nil || target.cfg.PGHost != "db.internal" || target.cfg.PGPassword != saved.Password || target.cfg.PGPassFile != "" || target.cfg.RestoreUser != "restore_operator" || target.cfg.RestorePassword != "restore-private-456" || target.cfg.RestorePassFile != "" {
		t.Fatal("wrong connection configuration")
	}
	schedule := `{"connection_id":"` + c.ID + `","database":"app","cron":"0 2 * * *","retention_days":14,"enabled":true}`
	if w = request(m, "POST", "/api/schedules", schedule, admin); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if w = request(m, "DELETE", "/api/connections/"+c.ID, "", admin); w.Code != 409 {
		t.Fatal("deleted scheduled connection")
	}
	request(m, "DELETE", "/api/schedules/1", "", admin)
	if w = request(m, "DELETE", "/api/connections/"+c.ID, "", admin); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if w = request(m, "GET", "/api/databases?connection_id="+c.ID, "", admin); w.Code != 404 {
		t.Fatal("missing connection fell back to env")
	}
	if w = request(m, "POST", "/api/schedules", schedule, admin); w.Code != 404 {
		t.Fatal("missing connection scheduled")
	}
	for _, invalid := range []string{
		`{"name":"bad","host":"host","port":"0","username":"user"}`,
		`{"name":"bad","host":"host","username":"user","sslmode":"invalid"}`,
		`{"name":"bad","host":"","username":"user"}`,
	} {
		if w = request(m, "POST", "/api/connections", invalid, admin); w.Code != 400 {
			t.Fatalf("invalid accepted: %s", w.Body)
		}
	}
}

func TestJobsAndRetentionStayOnTheirConnection(t *testing.T) {
	m := testManager(t)
	dir := t.TempDir()
	// Stub only the PostgreSQL binaries. Exercise real queue, files and SQLite metadata.
	dump := `#!/bin/sh
for arg in "$@"; do
 case "$arg" in --file=*) target="${arg#--file=}";; esac
done
printf '%s:%s' "$PGHOST" "$PGUSER" > "$target"
`
	for name, script := range map[string]string{"pg_dump": dump, "pg_restore": "#!/bin/sh\nexit 0\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, id := range []string{strings.Repeat("a", 32), strings.Repeat("b", 32)} {
		_, err := m.db.Exec(`INSERT INTO connections(id,name,host,port,username,password,database_name,sslmode) VALUES(?,?,?,?,?,?,?,?)`, id, id, id+".internal", "5432", "backup_"+id[:1], m.sealPassword("secret"), "postgres", "disable")
		if err != nil {
			t.Fatal(err)
		}
		m.queueMu.Lock()
		j, err := m.enqueueForLocked("backup", "same_name", "", id)
		m.queueMu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		waitJob(t, m, j.ID)
	}
	backups, err := m.backups()
	if err != nil || len(backups) != 2 {
		t.Fatal("missing backups", err)
	}
	for _, b := range backups {
		data, err := os.ReadFile(m.path(b.ID))
		if err != nil || string(data) != b.ConnectionID+".internal:backup_"+b.ConnectionID[:1] {
			t.Fatalf("wrong host used: %s %v", data, err)
		}
	}
	// An expired backup on A must not inherit the retention policy from B.
	old := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano)
	if _, err = m.db.Exec(`UPDATE backups SET source='scheduled',created_at=?`, old); err != nil {
		t.Fatal(err)
	}
	idB := strings.Repeat("b", 32)
	if _, err = m.db.Exec(`INSERT INTO schedules(connection_id,database_name,cron,retention_days,enabled) VALUES(?,'same_name','0 2 * * *',1,1)`, idB); err != nil {
		t.Fatal(err)
	}
	if err = m.retention(); err != nil {
		t.Fatal(err)
	}
	backups, err = m.backups()
	if err != nil || len(backups) != 1 || backups[0].ConnectionID != strings.Repeat("a", 32) {
		t.Fatal("retention crossed connections")
	}
}

func TestRestoreDestinationGuards(t *testing.T) {
	m := testManager(t)
	cookie := loginCookie(t, m, "admin", "test-admin-password")
	b := Backup{ID: token(), Database: "app", CreatedAt: now(), Source: "manual"}
	if err := m.saveBackup(b); err != nil {
		t.Fatal(err)
	}
	w := request(m, "POST", "/api/connections", `{"name":"No restore","host":"db.internal","username":"backup"}`, cookie)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	var c Connection
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id     string
		status int
	}{{c.ID, 400}, {token(), 404}} {
		payload := `{"backup_id":"` + b.ID + `","connection_id":"` + tc.id + `","target":"new_db","confirmation":"new_db","trusted":true}`
		if w = request(m, "POST", "/api/restore", payload, cookie); w.Code != tc.status {
			t.Fatalf("destination guard: %d %s", w.Code, w.Body)
		}
	}
	jobs, err := m.jobs()
	if err != nil || len(jobs) != 0 {
		t.Fatal("invalid destination queued a job")
	}
}

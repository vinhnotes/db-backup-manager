package manager

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeGzip(t *testing.T, path string, data []byte) {
	t.Helper()
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	z.Write(data)
	z.Close()
	if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestEngineVerify(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	dump := "/*M!999999\\- enable the sandbox mode */\n-- MariaDB dump 10.19  Distrib 11.8.3-MariaDB\nCREATE TABLE t(id int);\n-- Dump completed on 2026-11-04\n"
	writeGzip(t, file, []byte(dump))
	if err := (mysqlEngine{}).verify(ctx, nil, file); err != nil {
		t.Fatal(err)
	}
	writeGzip(t, file, []byte(strings.TrimSuffix(dump, "-- Dump completed on 2026-11-04\n")))
	if (mysqlEngine{}).verify(ctx, nil, file) == nil {
		t.Fatal("truncated SQL dump accepted")
	}
	writeGzip(t, file, append(append([]byte{}, mongoArchiveMagic...), "rest"...))
	if err := (mongoEngine{}).verify(ctx, nil, file); err != nil {
		t.Fatal(err)
	}
	writeGzip(t, file, []byte("not an archive"))
	if (mongoEngine{}).verify(ctx, nil, file) == nil {
		t.Fatal("bad mongo archive accepted")
	}
	os.WriteFile(file, []byte("TAPE\x00\x00rest"), 0600)
	if err := (mssqlEngine{}).verify(ctx, nil, file); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(file, []byte("MSSQLBAK\x02\x00"), 0600)
	if err := (mssqlEngine{}).verify(ctx, nil, file); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(file, []byte("PGDMP"), 0600)
	if (mssqlEngine{}).verify(ctx, nil, file) == nil {
		t.Fatal("non-.bak accepted")
	}
}

func TestDefinerFilter(t *testing.T) {
	in := "/*!50013 DEFINER=`ro``ot`@`%` SQL SECURITY DEFINER */\n" +
		"CREATE DEFINER=`app`@`localhost` PROCEDURE p() SELECT 1;\n" +
		"INSERT INTO t VALUES ('DEFINER=`x`@`y`');\n"
	out, err := io.ReadAll(&definerFilter{r: bufioReader(in)})
	if err != nil {
		t.Fatal(err)
	}
	want := "/*!50013  SQL SECURITY DEFINER */\nCREATE  PROCEDURE p() SELECT 1;\nINSERT INTO t VALUES ('DEFINER=`x`@`y`');\n"
	if string(out) != want {
		t.Fatalf("got %q", out)
	}
}

func TestOptionValueAndServerPath(t *testing.T) {
	if got := optionValue("a\"b\\c\nd"); got != `"a\"b\\c\nd"` {
		t.Fatal(got)
	}
	if serverPath(`D:\Backup\`, "x.bak") != `D:\Backup\x.bak` || serverPath("/var/opt/mssql/backup/", "x.bak") != "/var/opt/mssql/backup/x.bak" {
		t.Fatal("wrong server path")
	}
}

func TestConnectionValidationPerEngine(t *testing.T) {
	c := Connection{Engine: "mysql", Name: "m", Host: "db", Username: "u", Database: "ignored"}
	if err := validateConnection(&c); err != nil || c.Port != "3306" || c.Database != "" || c.SSLMode != "prefer" {
		t.Fatalf("mysql defaults: %v %+v", err, c)
	}
	c = Connection{Engine: "mongodb", Name: "m", Host: "db"}
	if err := validateConnection(&c); err != nil || c.Port != "27017" || c.Database != "admin" || c.SSLMode != "disable" {
		t.Fatalf("mongo defaults: %v %+v", err, c)
	}
	c = Connection{Engine: "mongodb", Name: "m", Host: "db", SSLMode: "prefer"}
	if validateConnection(&c) == nil {
		t.Fatal("mongo accepted an unsupported SSL mode")
	}
	c = Connection{Engine: "mssql", Name: "s", Host: "db", Username: "sa"}
	if validateConnection(&c) == nil {
		t.Fatal("mssql accepted without shared directory")
	}
	c = Connection{Engine: "mssql", Name: "s", Host: "db", Username: "sa", ServerDir: "/share", LocalDir: "/mnt/share"}
	if err := validateConnection(&c); err != nil || c.Database != "master" || c.Port != "1433" {
		t.Fatalf("mssql: %v %+v", err, c)
	}
	c = Connection{Engine: "oracle", Name: "o", Host: "db", Username: "u"}
	if validateConnection(&c) == nil {
		t.Fatal("unknown engine accepted")
	}
	c = Connection{Name: "p", Host: "db", Username: "u", ServerDir: "/x", LocalDir: "/y"}
	if err := validateConnection(&c); err != nil || c.Engine != "postgres" || c.ServerDir != "" || c.LocalDir != "" {
		t.Fatalf("postgres default: %v %+v", err, c)
	}
}

func TestEngineSpecificRestoreAndUpload(t *testing.T) {
	m := testManager(t)
	admin := loginCookie(t, m, "admin", "test-admin-password")
	body := `{"engine":"mysql","name":"My","host":"mysql.internal","username":"backup","password":"secret-1","restore_username":"root","restore_password":"secret-2"}`
	w := request(m, "POST", "/api/connections", body, admin)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	var c Connection
	json.Unmarshal(w.Body.Bytes(), &c)
	if c.Engine != "mysql" {
		t.Fatalf("engine not saved: %s", w.Body)
	}
	if w = request(m, "GET", "/api/connections", "", admin); !strings.Contains(w.Body.String(), `"engine":"mysql"`) {
		t.Fatal(w.Body)
	}
	// A PostgreSQL backup cannot be restored into a MySQL server.
	pg := Backup{ID: token(), Database: "app", CreatedAt: now(), Source: "manual", Checksum: "x"}
	if err := m.saveBackup(pg); err != nil {
		t.Fatal(err)
	}
	restore := `{"backup_id":"` + pg.ID + `","connection_id":"` + c.ID + `","target":"copy","confirmation":"copy","trusted":true}`
	if w = request(m, "POST", "/api/restore", restore, admin); w.Code != 400 || !strings.Contains(w.Body.String(), "PostgreSQL") {
		t.Fatalf("cross-engine restore: %d %s", w.Code, w.Body)
	}
	// System databases of the target engine are refused.
	restore = `{"backup_id":"` + pg.ID + `","connection_id":"` + c.ID + `","target":"mysql","confirmation":"mysql","trusted":true}`
	if w = request(m, "POST", "/api/restore", restore, admin); w.Code != 400 {
		t.Fatal("restore into mysql system database accepted")
	}
	// A plain .sql upload is compressed and stored as a MySQL backup.
	dump := "-- MySQL dump 10.13\nCREATE TABLE t(id int);\n-- Dump completed on 2026-11-04 10:00:00\n"
	r := uploadRequest(m, c.ID, "app", dump, admin)
	if r.Code != 201 {
		t.Fatalf("upload: %d %s", r.Code, r.Body)
	}
	var b Backup
	json.Unmarshal(r.Body.Bytes(), &b)
	if b.Engine != "mysql" {
		t.Fatalf("upload engine: %s", r.Body)
	}
	if gz, _ := isGzip(m.path(b.ID)); !gz {
		t.Fatal("upload not compressed")
	}
	if r = uploadRequest(m, c.ID, "app", "PGDMP not sql", admin); r.Code != 400 {
		t.Fatalf("invalid upload accepted: %d", r.Code)
	}
	if w = request(m, "GET", "/api/backups/"+b.ID+"/download", "", admin); w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), ".sql.gz") {
		t.Fatalf("download: %d %v", w.Code, w.Header())
	}
}

func bufioReader(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }

func uploadRequest(m *Manager, connectionID, database, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/upload", strings.NewReader(body))
	r.Header.Set("X-Requested-With", "DBBackupManager")
	r.Header.Set("X-Connection-ID", connectionID)
	r.Header.Set("X-Database-Name", database)
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, r)
	return w
}

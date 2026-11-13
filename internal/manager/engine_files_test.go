package manager

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTarGz(t *testing.T, path string, entries []tar.Header, content map[string]string) {
	t.Helper()
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	tw := tar.NewWriter(z)
	for _, h := range entries {
		h := h
		body := content[h.Name]
		if h.Typeflag == tar.TypeReg {
			h.Size = int64(len(body))
		}
		if h.Mode == 0 {
			h.Mode = 0644
		}
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(body))
	}
	tw.Close()
	z.Close()
	if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func filesManager(t *testing.T) (*Manager, string, string) {
	t.Helper()
	dir := t.TempDir()
	src, restore := filepath.Join(dir, "src"), filepath.Join(dir, "restore")
	os.MkdirAll(src, 0755)
	os.MkdirAll(restore, 0755)
	c := Connection{Engine: engineFiles, Name: "uploads", LocalDir: src, ServerDir: restore, Database: "files"}
	if err := validateConnection(&c); err != nil {
		t.Fatal(err)
	}
	cfg := Config{BackupDir: filepath.Join(dir, "backups")}.withConnection(c)
	os.MkdirAll(cfg.BackupDir, 0700)
	return &Manager{cfg: cfg}, src, restore
}

func TestDirectoryProbesValidateRuntimePaths(t *testing.T) {
	for _, dir := range []string{"", "/", "relative", "/tmp/../etc", "/tmp\nforged"} {
		if err := checkSharedDir(dir); err == nil {
			t.Errorf("SQL Server probe accepted %q", dir)
		}
		m, _, _ := filesManager(t)
		m.cfg.LocalDir = dir
		if _, err := (filesEngine{}).source(m); err == nil {
			t.Errorf("files source accepted %q", dir)
		}
		if dir != "" { // An empty restore path intentionally disables restore.
			m, _, _ = filesManager(t)
			m.cfg.ServerDir = dir
			if err := (filesEngine{}).test(t.Context(), m); err == nil {
				t.Errorf("files restore probe accepted %q", dir)
			}
		}
	}
}

func TestFilesBackupRestoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	m, src, restore := filesManager(t)
	e := filesEngine{}
	mtime := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	os.MkdirAll(filepath.Join(src, "2025", "01"), 0755)
	os.MkdirAll(filepath.Join(src, "empty"), 0755)
	os.WriteFile(filepath.Join(src, "2025", "01", "photo.jpg"), []byte("jpeg-bytes"), 0640)
	os.WriteFile(filepath.Join(src, "readme.txt"), []byte("hello"), 0644)
	os.Chtimes(filepath.Join(src, "readme.txt"), mtime, mtime)
	os.Symlink("2025/01/photo.jpg", filepath.Join(src, "latest.jpg"))
	os.MkdirAll(filepath.Join(src, "locked"), 0755)
	os.WriteFile(filepath.Join(src, "locked", "a"), []byte("a"), 0644)
	os.Chmod(filepath.Join(src, "locked"), 0555)
	t.Cleanup(func() { os.Chmod(filepath.Join(src, "locked"), 0755) })

	if err := e.test(ctx, m); err != nil {
		t.Fatal(err)
	}
	dbs, _, err := e.databases(ctx, m)
	if err != nil || len(dbs) != 1 || dbs[0].Name != "files" || dbs[0].Size != int64(len("jpeg-bytes")+len("hello")+1) {
		t.Fatalf("databases: %v %+v", err, dbs)
	}
	if e.dump(ctx, m, "other", filepath.Join(t.TempDir(), "x")) == nil {
		t.Fatal("dump of an unknown item accepted")
	}
	archive := filepath.Join(t.TempDir(), "a.tar.gz")
	if err = e.dump(ctx, m, "files", archive); err != nil {
		t.Fatal(err)
	}
	if err = e.verify(ctx, m, archive); err != nil {
		t.Fatal(err)
	}
	if err = e.restore(ctx, m, "copy", "files", archive); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(restore, "copy")
	if b, _ := os.ReadFile(filepath.Join(out, "2025", "01", "photo.jpg")); string(b) != "jpeg-bytes" {
		t.Fatalf("photo: %q", b)
	}
	if info, err := os.Stat(filepath.Join(out, "2025", "01", "photo.jpg")); err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("mode: %v %v", err, info)
	}
	if info, err := os.Stat(filepath.Join(out, "readme.txt")); err != nil || !info.ModTime().Equal(mtime) {
		t.Fatalf("mtime: %v %v", err, info)
	}
	if link, err := os.Readlink(filepath.Join(out, "latest.jpg")); err != nil || link != "2025/01/photo.jpg" {
		t.Fatalf("symlink: %v %q", err, link)
	}
	if info, err := os.Stat(filepath.Join(out, "empty")); err != nil || !info.IsDir() {
		t.Fatal("empty directory not restored")
	}
	if err = e.restore(ctx, m, "copy", "files", archive); err == nil || !strings.Contains(err.Error(), "đã tồn tại") {
		t.Fatalf("restore over an existing folder: %v", err)
	}
	// Restore test helpers: count files, then remove even read-only folders.
	if n, err := e.inspect(ctx, m, "copy"); err != nil || n != 3 {
		t.Fatalf("inspect: %d %v", n, err)
	}
	if err = e.drop(ctx, m, "copy"); err != nil {
		t.Fatal(err)
	}
	if found, err := e.exists(ctx, m, "copy"); err != nil || found {
		t.Fatalf("exists after drop: %v %v", found, err)
	}
}

func TestFilesArchiveSafety(t *testing.T) {
	ctx := context.Background()
	m, _, restore := filesManager(t)
	e := filesEngine{}
	file := filepath.Join(t.TempDir(), "a.tar.gz")
	bad := map[string][]tar.Header{
		"traversal": {{Name: "../evil", Typeflag: tar.TypeReg}},
		"absolute":  {{Name: "/etc/evil", Typeflag: tar.TypeReg}},
		"hardlink":  {{Name: "a", Typeflag: tar.TypeReg}, {Name: "b", Typeflag: tar.TypeLink, Linkname: "a"}},
		"device":    {{Name: "dev", Typeflag: tar.TypeChar}},
	}
	for name, entries := range bad {
		writeTarGz(t, file, entries, nil)
		if e.verify(ctx, m, file) == nil {
			t.Errorf("%s archive accepted", name)
		}
	}
	writeGzip(t, file, []byte("not a tar archive at all, just some bytes that are long enough"))
	if e.verify(ctx, m, file) == nil {
		t.Error("non-tar gzip accepted")
	}
	os.WriteFile(file, []byte("plain"), 0600)
	if e.verify(ctx, m, file) == nil {
		t.Error("non-gzip accepted")
	}
	// GNU tar style "./" entries are fine.
	writeTarGz(t, file, []tar.Header{{Name: "./", Typeflag: tar.TypeDir, Mode: 0755}, {Name: "./x.txt", Typeflag: tar.TypeReg}}, map[string]string{"./x.txt": "x"})
	if err := e.verify(ctx, m, file); err != nil {
		t.Fatal(err)
	}
	// A symlink to a folder outside, then an entry below it: nothing may be
	// written through the link.
	outside := t.TempDir()
	writeTarGz(t, file, []tar.Header{
		{Name: "a", Typeflag: tar.TypeSymlink, Linkname: outside},
		{Name: "a/b", Typeflag: tar.TypeSymlink, Linkname: "x"},
	}, nil)
	if err := e.restore(ctx, m, "attack", "files", file); err == nil {
		t.Fatal("conflicting symlinks restored")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("restore wrote outside the target: %v", entries)
	}
	writeTarGz(t, file, []tar.Header{{Name: "a", Typeflag: tar.TypeSymlink, Linkname: outside}, {Name: "a/f", Typeflag: tar.TypeReg}}, map[string]string{"a/f": "x"})
	e.restore(ctx, m, "attack2", "files", file)
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("restore wrote outside the target: %v", entries)
	}
	if e.restore(ctx, m, "../escape", "files", file) == nil {
		t.Fatal("invalid target name accepted")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(restore), "escape")); err == nil {
		t.Fatal("restore escaped the restore folder")
	}
}

func TestFilesConnectionValidation(t *testing.T) {
	c := Connection{Engine: engineFiles, Name: "u", LocalDir: "/sources/uploads", Host: "ignored", Username: "x", Password: "y"}
	if err := validateConnection(&c); err != nil || c.Database != "uploads" || c.Host != "/sources/uploads" || c.Username != "" || c.Password != "" || c.RestoreUsername != "" {
		t.Fatalf("defaults: %v %+v", err, c)
	}
	for dir, want := range map[string]string{"/sources/uploads/": "uploads", "/data/2024 ảnh": "_2024__nh", "/x/" + strings.Repeat("a", 70): strings.Repeat("a", 63), "/": "files", "": "files"} {
		if got := filesItemName(dir); got != want || !dbName.MatchString(got) {
			t.Errorf("filesItemName(%q) = %q, want %q", dir, got, want)
		}
	}
	c = Connection{Engine: engineFiles, Name: "u", LocalDir: "/sources/uploads", ServerDir: "/restore", Database: "uploads"}
	if err := validateConnection(&c); err != nil || c.RestoreUsername != "/restore" || c.Database != "uploads" {
		t.Fatalf("restore dir: %v %+v", err, c)
	}
	for _, bad := range []Connection{
		{Engine: engineFiles, Name: "u", LocalDir: "relative/path"},
		{Engine: engineFiles, Name: "u", LocalDir: "/"},
		{Engine: engineFiles, Name: "u", LocalDir: "/a/../b"},
		{Engine: engineFiles, Name: "u", LocalDir: "/src", ServerDir: "/src/restore"},
		{Engine: engineFiles, Name: "u", LocalDir: "/data/src", ServerDir: "/data"},
		{Engine: engineFiles, Name: "u", LocalDir: "/src", Database: "bad name"},
		{Engine: engineFiles, LocalDir: "/src"},
	} {
		if validateConnection(&bad) == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	// The source must not contain the backup folder (the archive would
	// include itself).
	dir := t.TempDir()
	m := &Manager{cfg: Config{BackupDir: filepath.Join(dir, "backups"), LocalDir: dir}}
	if (filesEngine{}).test(context.Background(), m) == nil {
		t.Fatal("source containing the backup folder accepted")
	}
}

func TestFilesConnectionAPIBackupUploadAndRestore(t *testing.T) {
	m := testManager(t)
	admin := loginCookie(t, m, "admin", "test-admin-password")
	dir := t.TempDir()
	src, restore := filepath.Join(dir, "src"), filepath.Join(dir, "restore")
	os.MkdirAll(src, 0755)
	os.MkdirAll(restore, 0755)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("A"), 0644)
	body, _ := json.Marshal(map[string]string{"engine": "files", "name": "Uploads", "local_dir": src, "server_dir": restore, "database": "uploads"})
	if w := request(m, "POST", "/api/connections/test", string(body), admin); w.Code != 200 {
		t.Fatalf("test: %d %s", w.Code, w.Body)
	}
	w := request(m, "POST", "/api/connections", string(body), admin)
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	var c Connection
	json.Unmarshal(w.Body.Bytes(), &c)
	if !c.RestoreEnabled || c.Engine != "files" {
		t.Fatalf("saved: %s", w.Body)
	}
	if w = request(m, "GET", "/api/databases?connection_id="+c.ID, "", admin); w.Code != 200 || !strings.Contains(w.Body.String(), `"uploads"`) {
		t.Fatalf("databases: %d %s", w.Code, w.Body)
	}
	target, err := m.forConnection(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	id, err := target.createBackup(context.Background(), "uploads", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if w = request(m, "GET", "/api/backups/"+id+"/download", "", admin); w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), ".tar.gz") {
		t.Fatalf("download: %d %v", w.Code, w.Header())
	}
	var got events
	m.notifyHook = got.add
	// Restore goes through the normal API checks, then the job.
	restoreBody := `{"backup_id":"` + id + `","connection_id":"` + c.ID + `","target":"uploads_copy","confirmation":"uploads_copy","trusted":true}`
	if w = request(m, "POST", "/api/restore", restoreBody, admin); w.Code != 202 {
		t.Fatalf("restore: %d %s", w.Code, w.Body)
	}
	if ev := got.all(); len(ev) == 0 || ev[0].Kind != eventRestore || ev[0].Actor != "admin" || ev[0].Database != "uploads_copy" || ev[0].Source.ID != id {
		t.Fatalf("restore notification: %+v", ev)
	}
	if _, msg := m.message("vi", got.all()[0]); !strings.Contains(msg, "Người thực hiện: admin") || !strings.Contains(msg, "Database đích: uploads_copy") {
		t.Fatalf("restore message: %s", msg)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if b, err := os.ReadFile(filepath.Join(restore, "uploads_copy", "a.txt")); err == nil && string(b) == "A" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("restore job did not finish")
		}
		time.Sleep(50 * time.Millisecond)
	}
	// An uploaded .tar.gz is verified and stored as a files backup.
	upload := filepath.Join(t.TempDir(), "u.tar.gz")
	writeTarGz(t, upload, []tar.Header{{Name: "x.txt", Typeflag: tar.TypeReg}}, map[string]string{"x.txt": "x"})
	data, _ := os.ReadFile(upload)
	if r := uploadRequest(m, c.ID, "uploads", string(data), admin); r.Code != 201 {
		t.Fatalf("upload: %d %s", r.Code, r.Body)
	}
	writeTarGz(t, upload, []tar.Header{{Name: "../x", Typeflag: tar.TypeReg}}, nil)
	data, _ = os.ReadFile(upload)
	if r := uploadRequest(m, c.ID, "uploads", string(data), admin); r.Code != 400 {
		t.Fatalf("unsafe upload accepted: %d %s", r.Code, r.Body)
	}
}

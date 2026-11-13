package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
)

// The test binary doubles as a fake `rclone` (backed by a local directory
// named by `root = …` in the config) and a no-op `pg_restore` when it is
// invoked through a symlink with that name.
func TestMain(m *testing.M) {
	switch filepath.Base(os.Args[0]) {
	case "rclone":
		os.Exit(fakeRclone(os.Args[1:]))
	case "pg_restore":
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeRclone(args []string) int {
	conf, rest := "", []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			conf = args[i+1]
			i++
		case "--stats", "--retries", "--low-level-retries", "--contimeout", "--timeout", "--include":
			i++
		default:
			if !strings.HasPrefix(args[i], "--") || args[i] == "--stat" {
				rest = append(rest, args[i])
			}
		}
	}
	data, err := os.ReadFile(conf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	root := ""
	for _, line := range strings.Split(string(data), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == "root" {
			root = strings.TrimSpace(v)
		}
	}
	// Simulate an OAuth token refresh rewriting the config file.
	if strings.Contains(string(data), "token = old") {
		os.WriteFile(conf, []byte(strings.Replace(string(data), "token = old", "token = refreshed", 1)), 0600)
	}
	local := func(target string) string { return filepath.Join(root, strings.TrimPrefix(target, "dest:")) }
	copyFile := func(src, dst string) error {
		in, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(dst), 0700)
		return os.WriteFile(dst, in, 0600)
	}
	switch rest[0] {
	case "rcat":
		os.MkdirAll(filepath.Dir(local(rest[1])), 0700)
		in, _ := io.ReadAll(os.Stdin)
		err = os.WriteFile(local(rest[1]), in, 0600)
	case "copyto":
		if strings.HasPrefix(rest[2], "dest:") {
			err = copyFile(rest[1], local(rest[2]))
		} else {
			err = copyFile(local(rest[1]), rest[2])
		}
	case "deletefile":
		if err = os.Remove(local(rest[1])); os.IsNotExist(err) {
			return 4
		}
	case "lsjson":
		if rest[1] == "--stat" {
			info, e := os.Stat(local(rest[2]))
			if e != nil {
				fmt.Fprintln(os.Stderr, "object not found")
				return 3
			}
			json.NewEncoder(os.Stdout).Encode(rcloneEntry{Path: info.Name(), Size: info.Size(), ModTime: info.ModTime().Format(time.RFC3339)})
			return 0
		}
		base := local(rest[2])
		if _, e := os.Stat(base); e != nil {
			fmt.Fprintln(os.Stderr, "directory not found")
			return 3
		}
		entries := []rcloneEntry{}
		filepath.WalkDir(base, func(p string, d fs.DirEntry, e error) error {
			if e == nil && !d.IsDir() && strings.HasSuffix(p, encryptedSuffix) {
				info, _ := d.Info()
				rel, _ := filepath.Rel(base, p)
				entries = append(entries, rcloneEntry{Path: filepath.ToSlash(rel), Size: info.Size(), ModTime: info.ModTime().Format(time.RFC3339)})
			}
			return nil
		})
		json.NewEncoder(os.Stdout).Encode(entries)
		return 0
	default:
		fmt.Fprintln(os.Stderr, "unsupported command", rest[0])
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func fakeTools(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"rclone", "pg_restore"} {
		if err = os.Symlink(self, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestParseRcloneConfig(t *testing.T) {
	opts, typ, err := parseRcloneConfig("[gdrive]\ntype = drive\nscope = drive\ntoken = {\"access_token\":\"a=b\"}\n# comment\n", false)
	if err != nil || typ != "drive" || len(opts) != 3 || opts[2].value != `{"access_token":"a=b"}` {
		t.Fatalf("%v %q %v", opts, typ, err)
	}
	for _, bad := range []string{"", "scope = drive", "type = local", "type = alias\nremote = x:", "[a]\ntype = drive\n[b]\ntype = s3", "type = sftp\nssh = /bin/sh -c id", "type = drive\ntype = s3", "type = drive\nnot a pair"} {
		if _, _, err := parseRcloneConfig(bad, false); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	for _, bad := range []string{"../x", "a/../b", "a//b", "a\nb"} {
		if _, err := cleanRemotePath(bad); err == nil {
			t.Fatalf("accepted path %q", bad)
		}
	}
	if p, err := cleanRemotePath(" /bucket/db-backups/ "); err != nil || p != "bucket/db-backups" {
		t.Fatal(p, err)
	}
}

func TestOffsiteUploadListPullAndRetention(t *testing.T) {
	m := testManager(t)
	tools := fakeTools(t)
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	m.cfg.RcloneBinary = filepath.Join(tools, "rclone")
	m.allowLocalRemote = true
	remote := t.TempDir()
	admin := loginCookie(t, m, "admin", "test-admin-password")
	viewer := loginCookie(t, m, "viewer", "test-viewer-password")

	w := request(m, "POST", "/api/destinations/keygen", "", admin)
	var keys struct{ PublicKey, PrivateKey string }
	json.Unmarshal(w.Body.Bytes(), &struct {
		PublicKey  *string `json:"public_key"`
		PrivateKey *string `json:"private_key"`
	}{&keys.PublicKey, &keys.PrivateKey})
	if w.Code != 200 || !strings.HasPrefix(keys.PublicKey, "age1") || !strings.HasPrefix(keys.PrivateKey, "AGE-SECRET-KEY-") {
		t.Fatalf("keygen: %d %s", w.Code, w.Body)
	}

	body := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	dest := map[string]any{"name": "Drive", "config": "[x]\ntype = local\nroot = " + remote + "\ntoken = old", "remote_path": "/db-backups/", "encryption": "recipient", "recipients": keys.PublicKey, "retention_days": 7}
	if w = request(m, "POST", "/api/destinations", body(dest), viewer); w.Code != 403 {
		t.Fatal("viewer created destination")
	}
	bad := map[string]any{"name": "x", "config": dest["config"], "encryption": "recipient", "recipients": "not-a-key"}
	if w = request(m, "POST", "/api/destinations", body(bad), admin); w.Code != 400 {
		t.Fatalf("invalid key accepted: %d", w.Code)
	}
	w = request(m, "POST", "/api/destinations", body(dest), admin)
	var d Destination
	if json.Unmarshal(w.Body.Bytes(), &d); w.Code != 200 || d.Config != "" || !d.HasConfig || d.RemotePath != "db-backups" {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var rawConfig string
	m.db.QueryRow(`SELECT config FROM destinations WHERE id=?`, d.ID).Scan(&rawConfig)
	if strings.Contains(rawConfig, "token") || strings.Contains(rawConfig, remote) {
		t.Fatal("rclone config stored in plain text")
	}
	if w = request(m, "GET", "/api/destinations", "", viewer); w.Code != 200 || strings.Contains(w.Body.String(), "token") {
		t.Fatalf("list leaked config: %s", w.Body)
	}
	if w = request(m, "POST", "/api/destinations/test", body(map[string]any{"id": d.ID, "name": "Drive", "encryption": "recipient", "recipients": keys.PublicKey}), admin); w.Code != 200 {
		t.Fatalf("test: %d %s", w.Code, w.Body)
	}
	if stored, _ := m.destination(d.ID); !strings.Contains(stored.Config, "token = refreshed") {
		t.Fatalf("refreshed token not persisted: %q", stored.Config)
	}

	// A local archive to upload.
	plain := []byte("PGDMP fake custom archive " + strings.Repeat("x", 100000))
	b := Backup{"", token(), "app_db", int64(len(plain)), "", now(), "manual", ""}
	os.WriteFile(m.path(b.ID), plain, 0600)
	b.Checksum, _, _ = checksum(m.path(b.ID))
	if err := m.saveBackup(b); err != nil {
		t.Fatal(err)
	}
	w = request(m, "POST", "/api/backups/"+b.ID+"/offsite", body(map[string]string{"destination_id": d.ID}), admin)
	var j Job
	if json.Unmarshal(w.Body.Bytes(), &j); w.Code != 202 {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	waitJob(t, m, j.ID)
	copies, _ := m.offsiteCopies()
	if len(copies) != 1 || copies[0].BackupID != b.ID || !strings.HasPrefix(copies[0].RemotePath, "env/app_db/app_db_") {
		t.Fatalf("offsite rows: %+v", copies)
	}
	encrypted, err := os.ReadFile(filepath.Join(remote, "db-backups", copies[0].RemotePath))
	if err != nil || bytes.Contains(encrypted, []byte("PGDMP")) {
		t.Fatal("remote file missing or not encrypted", err)
	}
	identity, _ := age.ParseX25519Identity(keys.PrivateKey)
	r, err := age.Decrypt(bytes.NewReader(encrypted), identity)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := io.ReadAll(r); !bytes.Equal(got, plain) {
		t.Fatal("decrypted content differs")
	}
	if leftovers, _ := filepath.Glob(filepath.Join(m.cfg.BackupDir, "*.partial")); len(leftovers) > 0 {
		t.Fatal("staging files left behind", leftovers)
	}

	// Listing and importing back.
	w = request(m, "GET", "/api/destinations/"+d.ID+"/files", "", viewer)
	var files []RemoteFile
	if json.Unmarshal(w.Body.Bytes(), &files); w.Code != 200 || len(files) != 1 || files[0].Database != "app_db" || files[0].BackupID != b.ID {
		t.Fatalf("files: %d %s", w.Code, w.Body)
	}
	pull := map[string]string{"remote_path": files[0].Path, "database": "app_db", "connection_id": ""}
	if w = request(m, "POST", "/api/destinations/"+d.ID+"/pull", body(pull), admin); w.Code != 400 {
		t.Fatalf("pull without private key: %d", w.Code)
	}
	pull["secret"] = keys.PrivateKey
	w = request(m, "POST", "/api/destinations/"+d.ID+"/pull", body(pull), admin)
	if json.Unmarshal(w.Body.Bytes(), &j); w.Code != 202 {
		t.Fatalf("pull: %d %s", w.Code, w.Body)
	}
	waitJob(t, m, j.ID)
	if strings.Contains(fmt.Sprint(m.pullSecrets), "AGE-SECRET") {
		t.Fatal("private key kept in memory after job")
	}
	backups, _ := m.backups()
	var imported *Backup
	for i := range backups {
		if backups[i].Source == "cloud" {
			imported = &backups[i]
		}
	}
	if imported == nil || imported.Checksum != b.Checksum || imported.Database != "app_db" {
		t.Fatalf("imported: %+v", backups)
	}

	// Retention keeps the newest copy per database and deletes expired ones.
	old := Offsite{token(), d.ID, "", "app_db", "env/app_db/old.dump.age", 1, "x", time.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339Nano), now()}
	os.WriteFile(filepath.Join(remote, "db-backups", old.RemotePath), []byte("old"), 0600)
	gone := old
	gone.BackupID, gone.RemotePath = token(), "env/app_db/already-gone.dump.age"
	m.saveOffsite(old)
	m.saveOffsite(gone)
	if err = m.remoteRetention(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(remote, "db-backups", old.RemotePath)); !os.IsNotExist(err) {
		t.Fatal("expired remote file kept")
	}
	if _, err = os.Stat(filepath.Join(remote, "db-backups", copies[0].RemotePath)); err != nil {
		t.Fatal("newest remote file deleted")
	}
	if copies, _ = m.offsiteCopies(); len(copies) != 2 {
		t.Fatalf("offsite rows after retention: %+v", copies)
	}

	// Destinations used by schedules cannot be deleted.
	sched := map[string]any{"database": "*", "cron": "0 2 * * *", "retention_days": 7, "enabled": true, "destination_id": d.ID}
	if w = request(m, "POST", "/api/schedules", body(sched), admin); w.Code != 200 {
		t.Fatalf("schedule: %d %s", w.Code, w.Body)
	}
	if ss, _ := m.schedules(); len(ss) != 1 || ss[0].DestinationID != d.ID {
		t.Fatalf("schedule destination not saved: %+v", ss)
	}
	if w = request(m, "DELETE", "/api/destinations/"+d.ID, "", admin); w.Code != 409 {
		t.Fatalf("delete in-use destination: %d", w.Code)
	}
}

func TestPassphraseEncryptionRoundTrip(t *testing.T) {
	m := testManager(t)
	d := Destination{Name: "S3", Config: "type = s3\nprovider = AWS", Encryption: "passphrase", Passphrase: "short"}
	if err := m.validateDestination(&d, nil); err == nil {
		t.Fatal("short passphrase accepted")
	}
	d.Passphrase = "correct horse battery staple"
	if err := m.validateDestination(&d, nil); err != nil {
		t.Fatal(err)
	}
	// Editing without re-entering the passphrase keeps the stored one.
	edit := Destination{Name: "S3", Encryption: "passphrase", HasPassphrase: true}
	if err := m.validateDestination(&edit, &d); err != nil || edit.Passphrase != d.Passphrase || edit.Config != d.Config {
		t.Fatal("edit lost secrets", err)
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "a.dump")
	os.WriteFile(src, []byte("PGDMP data"), 0600)
	sum, _, _ := checksum(src)
	recipients, _ := d.recipients()
	if _, _, err := encryptFile(src, filepath.Join(dir, "a.age"), sum, recipients); err != nil {
		t.Fatal(err)
	}
	if _, _, err := encryptFile(src, filepath.Join(dir, "b.age"), "tampered", recipients); err == nil {
		t.Fatal("checksum mismatch not detected")
	}
	f, _ := os.Open(filepath.Join(dir, "a.age"))
	defer f.Close()
	id, _ := age.NewScryptIdentity(d.Passphrase)
	r, err := age.Decrypt(f, id)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := io.ReadAll(r); string(got) != "PGDMP data" {
		t.Fatal("round trip failed")
	}
}

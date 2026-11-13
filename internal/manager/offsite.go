package manager

// Offsite copies: encrypt archives with age and send them to any rclone
// remote (Google Drive, OneDrive, Dropbox, S3, B2, SFTP, WebDAV…).
//
// The rclone remote definition (including OAuth tokens) is stored sealed with
// connections.key. Each rclone run gets a private temporary config file; if
// rclone refreshes a token during the run, the new config is sealed again and
// saved, so rotating refresh tokens (OneDrive) keep working.

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"filippo.io/age"
)

type Destination struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Config        string `json:"config,omitempty"`
	GoogleOAuthID string `json:"google_oauth_id,omitempty"`
	HasConfig     bool   `json:"has_config"`
	RemotePath    string `json:"remote_path"`
	Encryption    string `json:"encryption"`
	Recipients    string `json:"recipients"`
	Passphrase    string `json:"passphrase,omitempty"`
	HasPassphrase bool   `json:"has_passphrase"`
	RetentionDays int    `json:"retention_days"`
	CreatedAt     string `json:"created_at"`
}

type Offsite struct {
	BackupID        string `json:"backup_id"`
	DestinationID   string `json:"destination_id"`
	ConnectionID    string `json:"connection_id"`
	Database        string `json:"database"`
	RemotePath      string `json:"remote_path"`
	Size            int64  `json:"size"`
	Checksum        string `json:"checksum"`
	BackupCreatedAt string `json:"backup_created_at"`
	UploadedAt      string `json:"uploaded_at"`
}

type RemoteFile struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	ModTime  string `json:"mod_time"`
	Database string `json:"database"`
	BackupID string `json:"backup_id"`
}

const encryptedSuffix = ".dump.age"

var (
	rcloneType    = regexp.MustCompile(`^[a-z0-9]{1,32}$`)
	rcloneKey     = regexp.MustCompile(`^[a-zA-Z0-9_]{1,64}$`)
	remoteFileRe  = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_-]{0,62})_(\d{8}T\d{6}Z)_[a-f0-9]{8}\.dump\.age$`)
	unsafePathRun = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	// Backends that would touch the container filesystem or only wrap other remotes.
	blockedTypes = map[string]bool{"local": true, "alias": true, "memory": true, "union": true, "combine": true, "crypt": true, "chunker": true, "hasher": true, "compress": true, "cache": true}
	// Options that make rclone execute external programs.
	blockedKeys = map[string]bool{"ssh": true}
)

// ---- secrets -------------------------------------------------------------

func (m *Manager) openSealed(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	gcm := m.passwordCipher()
	raw, err := decodeBase64(value)
	if err != nil || len(raw) < gcm.NonceSize() {
		return "", errors.New("cannot decrypt stored secret")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("cannot decrypt stored secret")
	}
	return string(plain), nil
}

// ---- rclone config -------------------------------------------------------

type rcloneOption struct{ key, value string }

// parseRcloneConfig accepts the output of `rclone config show <remote>` (one
// section) or bare `key = value` lines and returns normalized options.
func parseRcloneConfig(text string, allowLocal bool) ([]rcloneOption, string, error) {
	if len(text) > 65536 {
		return nil, "", errors.New("Cấu hình rclone quá dài")
	}
	opts := []rcloneOption{}
	seen := map[string]bool{}
	sections := 0
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 65536), 65536)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sections++
			if sections > 1 {
				return nil, "", errors.New("Chỉ dán cấu hình của một remote rclone")
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || !rcloneKey.MatchString(key) {
			return nil, "", fmt.Errorf("Dòng cấu hình rclone không hợp lệ: %q", truncate(line, 40))
		}
		key = strings.ToLower(key)
		if seen[key] {
			return nil, "", fmt.Errorf("Khóa %q bị lặp", key)
		}
		if blockedKeys[key] {
			return nil, "", fmt.Errorf("Không hỗ trợ tùy chọn %q", key)
		}
		seen[key] = true
		opts = append(opts, rcloneOption{key, value})
	}
	if err := sc.Err(); err != nil {
		return nil, "", errors.New("Cấu hình rclone không hợp lệ")
	}
	typ := ""
	for _, o := range opts {
		if o.key == "type" {
			typ = o.value
		}
	}
	if !rcloneType.MatchString(typ) {
		return nil, "", errors.New("Cấu hình rclone cần dòng type = … (ví dụ drive, onedrive, s3)")
	}
	if blockedTypes[typ] && !(allowLocal && typ == "local") {
		return nil, "", fmt.Errorf("Không hỗ trợ remote loại %q", typ)
	}
	return opts, typ, nil
}

func renderRcloneConfig(opts []rcloneOption) string {
	var b strings.Builder
	for _, o := range opts {
		b.WriteString(o.key + " = " + o.value + "\n")
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func cleanRemotePath(p string) (string, error) {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if len(p) > 512 || strings.ContainsAny(p, "\x00\r\n\t") {
		return "", errors.New("Thư mục trên cloud không hợp lệ")
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." || part == "." || (part == "" && p != "") {
			return "", errors.New("Thư mục trên cloud không hợp lệ")
		}
	}
	return p, nil
}

// ---- persistence ---------------------------------------------------------

func (m *Manager) destinations() ([]Destination, error) {
	rows, err := m.db.Query(`SELECT id,name,type,config,remote_path,encryption,recipients,passphrase,retention_days,created_at FROM destinations ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Destination{}
	for rows.Next() {
		var d Destination
		var config, pass string
		if err = rows.Scan(&d.ID, &d.Name, &d.Type, &config, &d.RemotePath, &d.Encryption, &d.Recipients, &pass, &d.RetentionDays, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.HasConfig, d.HasPassphrase = config != "", pass != ""
		out = append(out, d)
	}
	return out, rows.Err()
}

// destination returns a destination with decrypted config and passphrase.
func (m *Manager) destination(id string) (Destination, error) {
	var d Destination
	var config, pass string
	err := m.db.QueryRow(`SELECT id,name,type,config,remote_path,encryption,recipients,passphrase,retention_days,created_at FROM destinations WHERE id=?`, id).Scan(&d.ID, &d.Name, &d.Type, &config, &d.RemotePath, &d.Encryption, &d.Recipients, &pass, &d.RetentionDays, &d.CreatedAt)
	if err != nil {
		return d, errors.New("Đích cloud không tồn tại")
	}
	if d.Config, err = m.openSealed(config); err != nil {
		return d, err
	}
	if d.Passphrase, err = m.openSealed(pass); err != nil {
		return d, err
	}
	d.HasConfig, d.HasPassphrase = d.Config != "", d.Passphrase != ""
	return d, nil
}

func (m *Manager) offsiteCopies() ([]Offsite, error) {
	rows, err := m.db.Query(`SELECT backup_id,destination_id,connection_id,database_name,remote_path,size,checksum,backup_created_at,uploaded_at FROM offsite ORDER BY backup_created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Offsite{}
	for rows.Next() {
		var o Offsite
		if err = rows.Scan(&o.BackupID, &o.DestinationID, &o.ConnectionID, &o.Database, &o.RemotePath, &o.Size, &o.Checksum, &o.BackupCreatedAt, &o.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (m *Manager) saveOffsite(o Offsite) error {
	_, err := m.db.Exec(`INSERT INTO offsite(backup_id,destination_id,connection_id,database_name,remote_path,size,checksum,backup_created_at,uploaded_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(backup_id,destination_id) DO UPDATE SET connection_id=excluded.connection_id,database_name=excluded.database_name,remote_path=excluded.remote_path,size=excluded.size,checksum=excluded.checksum,backup_created_at=excluded.backup_created_at,uploaded_at=excluded.uploaded_at`, o.BackupID, o.DestinationID, o.ConnectionID, o.Database, o.RemotePath, o.Size, o.Checksum, o.BackupCreatedAt, o.UploadedAt)
	return err
}

// validateDestination normalizes input. old carries stored secrets when editing.
func (m *Manager) validateDestination(d *Destination, old *Destination) error {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" || len(d.Name) > 100 {
		return errors.New("Tên đích cloud không hợp lệ")
	}
	if strings.TrimSpace(d.Config) == "" && old != nil {
		d.Config = old.Config
	}
	opts, typ, err := parseRcloneConfig(d.Config, m.allowLocalRemote)
	if err != nil {
		return err
	}
	d.Config, d.Type = renderRcloneConfig(opts), typ
	if d.RemotePath, err = cleanRemotePath(d.RemotePath); err != nil {
		return err
	}
	if d.RetentionDays < 0 || d.RetentionDays > 3650 {
		return errors.New("Retention trên cloud phải từ 0 đến 3650 ngày (0 = giữ vĩnh viễn)")
	}
	switch d.Encryption {
	case "recipient":
		d.Recipients = strings.TrimSpace(d.Recipients)
		if _, err = age.ParseRecipients(strings.NewReader(d.Recipients)); err != nil || len(d.Recipients) > 16384 {
			return errors.New("Public key age không hợp lệ (dạng age1…, mỗi dòng một khóa)")
		}
		d.Passphrase = ""
	case "passphrase":
		d.Recipients = ""
		if d.Passphrase == "" && old != nil && old.Encryption == "passphrase" && d.HasPassphrase {
			d.Passphrase = old.Passphrase
		}
		if len(d.Passphrase) < 16 || len(d.Passphrase) > 1024 {
			return errors.New("Passphrase mã hóa cần 16–1024 ký tự")
		}
	default:
		return errors.New("Chọn phương thức mã hóa")
	}
	return nil
}

func (d Destination) recipients() ([]age.Recipient, error) {
	if d.Encryption == "passphrase" {
		r, err := age.NewScryptRecipient(d.Passphrase)
		if err != nil {
			return nil, err
		}
		return []age.Recipient{r}, nil
	}
	return age.ParseRecipients(strings.NewReader(d.Recipients))
}

// ---- rclone execution ----------------------------------------------------

type rcloneSession struct {
	m        *Manager
	dest     Destination
	dir      string
	conf     string
	original []byte
	secrets  []string
	// quick fails fast for interactive checks instead of retrying for minutes.
	quick bool
}

func (m *Manager) rclone(d Destination) (*rcloneSession, error) {
	if d.Config == "" {
		return nil, errors.New("Đích cloud chưa có cấu hình rclone")
	}
	dir, err := os.MkdirTemp("", "dbbm-rclone-")
	if err != nil {
		return nil, err
	}
	conf := filepath.Join(dir, "rclone.conf")
	content := []byte("[dest]\n" + d.Config)
	if err = os.WriteFile(conf, content, 0600); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	s := &rcloneSession{m: m, dest: d, dir: dir, conf: conf, original: content}
	for _, line := range strings.Split(d.Config, "\n") {
		if _, v, ok := strings.Cut(line, "="); ok && len(strings.TrimSpace(v)) >= 8 {
			s.secrets = append(s.secrets, strings.TrimSpace(v))
		}
	}
	return s, nil
}

func (s *rcloneSession) target(rel string) string {
	return "dest:" + path.Join(s.dest.RemotePath, rel)
}

type rcloneError struct {
	code int
	msg  string
}

func (e *rcloneError) Error() string { return e.msg }

func (s *rcloneSession) run(ctx context.Context, stdin io.Reader, stdout io.Writer, args ...string) error {
	base := []string{"--config", s.conf, "--stats", "0", "--retries", "3", "--low-level-retries", "10", "--contimeout", "30s", "--timeout", "5m", "--ask-password=false"}
	if s.quick {
		base = []string{"--config", s.conf, "--stats", "0", "--retries", "1", "--low-level-retries", "2", "--contimeout", "15s", "--timeout", "1m", "--ask-password=false"}
	}
	cmd := exec.CommandContext(ctx, s.m.cfg.RcloneBinary, append(base, args...)...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + s.dir, "TMPDIR=" + s.dir, "XDG_CACHE_HOME=" + s.dir, "XDG_CONFIG_HOME=" + s.dir, "TZ=UTC"}
	cmd.Stdin, cmd.Stdout = stdin, stdout
	var stderr limitedBuffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}
	msg := stderr.String()
	for _, secret := range s.secrets {
		msg = strings.ReplaceAll(msg, secret, "[redacted]")
	}
	if ctx.Err() != nil {
		return &rcloneError{-1, fmt.Sprintf("rclone %s: hết thời gian chờ (%v): %s", args[0], ctx.Err(), truncate(strings.TrimSpace(msg), 2000))}
	}
	code := -1
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if errors.Is(err, exec.ErrNotFound) {
		return &rcloneError{code, "rclone chưa được cài đặt trên máy chủ backup"}
	}
	return &rcloneError{code, fmt.Sprintf("rclone %s failed: %v: %s", args[0], err, strings.TrimSpace(msg))}
}

// close persists refreshed OAuth tokens and removes the temporary config.
func (s *rcloneSession) close() {
	defer os.RemoveAll(s.dir)
	current, err := os.ReadFile(s.conf)
	if err != nil || string(current) == string(s.original) {
		return
	}
	opts, _, err := parseRcloneConfig(string(current), true)
	if err != nil {
		s.m.logger.Printf("destination %s: cannot parse refreshed rclone config: %v", s.dest.ID, err)
		return
	}
	var stored string
	if err = s.m.db.QueryRow(`SELECT config FROM destinations WHERE id=?`, s.dest.ID).Scan(&stored); err != nil {
		return
	}
	// Only replace the config this session started from; an admin edit wins.
	if plain, e := s.m.openSealed(stored); e != nil || plain != s.dest.Config {
		return
	}
	if _, err = s.m.db.Exec(`UPDATE destinations SET config=? WHERE id=? AND config=?`, s.m.sealPassword(renderRcloneConfig(opts)), s.dest.ID, stored); err != nil {
		s.m.logger.Printf("destination %s: cannot save refreshed token: %v", s.dest.ID, err)
	}
}

type rcloneEntry struct {
	Path    string
	Size    int64
	ModTime string
	IsDir   bool
}

func (s *rcloneSession) stat(ctx context.Context, rel string) (rcloneEntry, error) {
	var out limitedBuffer
	var e rcloneEntry
	if err := s.run(ctx, nil, &out, "lsjson", "--stat", "--no-mimetype", s.target(rel)); err != nil {
		return e, err
	}
	err := json.Unmarshal(out.Bytes(), &e)
	return e, err
}

// testDestination writes, reads back and deletes a small probe file.
func (m *Manager) testDestination(ctx context.Context, d Destination) error {
	if _, err := d.recipients(); err != nil {
		return errors.New("Khóa mã hóa không hợp lệ")
	}
	s, err := m.rclone(d)
	if err != nil {
		return err
	}
	defer s.close()
	s.quick = true
	probe := ".dbbm-test-" + token()
	payload := "db-backup-manager connectivity test " + now()
	if err = s.run(ctx, strings.NewReader(payload), nil, "rcat", s.target(probe)); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_ = s.run(cleanup, nil, nil, "deletefile", s.target(probe))
	}()
	e, err := s.stat(ctx, probe)
	if err != nil {
		return err
	}
	if e.Size != int64(len(payload)) {
		return errors.New("File kiểm tra trên cloud có dung lượng không khớp")
	}
	return nil
}

// ---- upload --------------------------------------------------------------

func (m *Manager) connectionLabel(id string) string {
	if id == "" {
		return "env"
	}
	var name string
	_ = m.db.QueryRow(`SELECT name FROM connections WHERE id=?`, id).Scan(&name)
	label := strings.Trim(unsafePathRun.ReplaceAllString(name, "-"), "-.")
	if label == "" {
		return id[:8]
	}
	return truncate(label, 60)
}

func remoteFileName(b Backup, connection string) string {
	created, err := time.Parse(time.RFC3339Nano, b.CreatedAt)
	if err != nil {
		created = time.Now()
	}
	return path.Join(connection, b.Database, fmt.Sprintf("%s_%s_%s%s", b.Database, created.UTC().Format("20060102T150405Z"), b.ID[:8], encryptedSuffix))
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// encryptFile encrypts src into dst, verifies the plaintext SHA-256 and
// returns the ciphertext SHA-256 and size.
func encryptFile(src, dst, plainSum string, recipients []age.Recipient) (string, int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", 0, err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", 0, err
	}
	defer out.Close()
	cipherHash, plainHash := sha256.New(), sha256.New()
	counter := &countingWriter{w: io.MultiWriter(out, cipherHash)}
	w, err := age.Encrypt(counter, recipients...)
	if err != nil {
		return "", 0, err
	}
	if _, err = io.Copy(w, io.TeeReader(in, plainHash)); err != nil {
		return "", 0, err
	}
	if err = w.Close(); err != nil {
		return "", 0, err
	}
	if err = out.Sync(); err != nil {
		return "", 0, err
	}
	if hex.EncodeToString(plainHash.Sum(nil)) != plainSum {
		return "", 0, errors.New("backup checksum mismatch; local archive changed since it was created")
	}
	return hex.EncodeToString(cipherHash.Sum(nil)), counter.n, out.Close()
}

func (m *Manager) uploadOffsite(ctx context.Context, j Job) error {
	b, err := m.backup(j.BackupID)
	if err != nil {
		return errors.New("Bản backup không còn trên máy chủ")
	}
	d, err := m.destination(j.DestinationID)
	if err != nil {
		return err
	}
	recipients, err := d.recipients()
	if err != nil {
		return errors.New("Khóa mã hóa của đích cloud không hợp lệ")
	}
	if err = m.checkSpace(b.Size + 1<<20); err != nil {
		return err
	}
	staging := m.path(b.ID) + "." + d.ID[:8] + ".age.partial"
	defer os.Remove(staging)
	sum, size, err := encryptFile(m.path(b.ID), staging, b.Checksum, recipients)
	if err != nil {
		return err
	}
	s, err := m.rclone(d)
	if err != nil {
		return err
	}
	defer s.close()
	rel := remoteFileName(b, m.connectionLabel(b.ConnectionID))
	if err = s.run(ctx, nil, nil, "copyto", staging, s.target(rel)); err != nil {
		return err
	}
	e, err := s.stat(ctx, rel)
	if err != nil {
		return fmt.Errorf("uploaded but cannot verify remote file: %w", err)
	}
	if e.Size != size {
		return fmt.Errorf("remote size %d does not match encrypted size %d", e.Size, size)
	}
	if err = m.saveOffsite(Offsite{b.ID, d.ID, b.ConnectionID, b.Database, rel, size, sum, b.CreatedAt, now()}); err != nil {
		return err
	}
	m.audit("offsite.uploaded", b.ID+" → "+d.Name+":"+rel)
	return nil
}

// chainOffsite queues an upload after a successful backup job.
func (m *Manager) chainOffsite(parent Job, backupID string) {
	m.queueMu.Lock()
	_, err := m.enqueueJobLocked(Job{ConnectionID: parent.ConnectionID, Kind: "offsite", Database: parent.Database, BackupID: backupID, DestinationID: parent.DestinationID})
	m.queueMu.Unlock()
	if err != nil {
		_, e := m.db.Exec(`INSERT INTO jobs(connection_id,id,kind,database_name,backup_id,destination_id,status,error,created_at,finished_at) VALUES(?,?,'offsite',?,?,?,'failed',?,?,?)`, parent.ConnectionID, token(), parent.Database, backupID, parent.DestinationID, "Cannot queue cloud upload: "+err.Error(), now(), now())
		if e != nil {
			m.logger.Print(e)
		}
	}
}

// ---- download / import ---------------------------------------------------

func (m *Manager) listRemote(ctx context.Context, d Destination) ([]RemoteFile, error) {
	s, err := m.rclone(d)
	if err != nil {
		return nil, err
	}
	defer s.close()
	s.quick = true
	out := &cappedBuffer{limit: 32 << 20}
	if err = s.run(ctx, nil, out, "lsjson", "-R", "--files-only", "--no-mimetype", "--include", "*"+encryptedSuffix, s.target("")); err != nil {
		var re *rcloneError
		if errors.As(err, &re) && re.code == 3 {
			return []RemoteFile{}, nil // folder not created yet
		}
		return nil, err
	}
	if out.overflow {
		return nil, errors.New("Quá nhiều file trên cloud để liệt kê")
	}
	var entries []rcloneEntry
	if err = json.Unmarshal(out.Bytes(), &entries); err != nil {
		return nil, errors.New("Không đọc được danh sách file từ rclone")
	}
	known := map[string]string{}
	rows, err := m.db.Query(`SELECT remote_path,backup_id FROM offsite WHERE destination_id=?`, d.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p, id string
		if rows.Scan(&p, &id) == nil {
			known[p] = id
		}
	}
	rows.Close()
	files := []RemoteFile{}
	for _, e := range entries {
		f := RemoteFile{Path: e.Path, Size: e.Size, ModTime: e.ModTime}
		if match := remoteFileRe.FindStringSubmatch(path.Base(e.Path)); match != nil {
			f.Database = match[1]
		}
		if id := known[e.Path]; id != "" {
			if _, err := m.backup(id); err == nil {
				f.BackupID = id
			}
		}
		files = append(files, f)
	}
	sort.Slice(files, func(i, k int) bool { return files[i].ModTime > files[k].ModTime })
	return files, nil
}

type cappedBuffer struct {
	limitedBuffer
	limit    int
	overflow bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if c.Len()+len(p) > c.limit {
		c.overflow = true
		return len(p), nil
	}
	return c.Buffer.Write(p)
}

func (m *Manager) pullOffsite(ctx context.Context, j Job) error {
	m.pullMu.Lock()
	secret := m.pullSecrets[j.ID]
	delete(m.pullSecrets, j.ID)
	m.pullMu.Unlock()
	d, err := m.destination(j.DestinationID)
	if err != nil {
		return err
	}
	var identities []age.Identity
	switch {
	case d.Encryption == "recipient" || strings.HasPrefix(strings.TrimSpace(secret), "AGE-SECRET-KEY-"):
		if secret == "" {
			return errors.New("Private key không còn trong bộ nhớ (máy chủ đã khởi động lại); tạo lại yêu cầu")
		}
		identities, err = age.ParseIdentities(strings.NewReader(secret))
		if err != nil {
			return errors.New("Private key age không hợp lệ")
		}
	default:
		pass := d.Passphrase
		if secret != "" {
			pass = secret
		}
		id, e := age.NewScryptIdentity(pass)
		if e != nil {
			return e
		}
		identities = []age.Identity{id}
	}
	if err = m.checkSpace(0); err != nil {
		return err
	}
	id := token()
	encrypted := m.path(id) + ".age.partial"
	partial := m.path(id) + ".partial"
	defer os.Remove(encrypted)
	defer os.Remove(partial)
	s, err := m.rclone(d)
	if err != nil {
		return err
	}
	defer s.close()
	e, err := s.stat(ctx, j.RemotePath)
	if err != nil {
		return err
	}
	if err = m.checkSpace(2 * e.Size); err != nil {
		return err
	}
	if err = s.run(ctx, nil, nil, "copyto", s.target(j.RemotePath), encrypted); err != nil {
		return err
	}
	cipherSum, cipherSize, err := checksum(encrypted)
	if err != nil {
		return err
	}
	in, err := os.Open(encrypted)
	if err != nil {
		return err
	}
	defer in.Close()
	r, err := age.Decrypt(in, identities...)
	if err != nil {
		return errors.New("Không giải mã được: sai private key/passphrase hoặc file không phải định dạng age")
	}
	out, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, r); err != nil {
		out.Close()
		return fmt.Errorf("Giải mã thất bại (file có thể bị hỏng hoặc bị sửa đổi): %w", err)
	}
	if err = out.Close(); err != nil {
		return err
	}
	target, err := m.forConnection(j.ConnectionID)
	if err != nil {
		return err
	}
	engine := target.engine()
	if err = engine.verify(ctx, target, partial); err != nil {
		return fmt.Errorf("File giải mã không phải bản backup %s hợp lệ", engine.label())
	}
	sum, size, err := checksum(partial)
	if err != nil {
		return err
	}
	if err = os.Rename(partial, m.path(id)); err != nil {
		return err
	}
	created := now()
	if match := remoteFileRe.FindStringSubmatch(path.Base(j.RemotePath)); match != nil {
		if t, e := time.Parse("20060102T150405Z", match[2]); e == nil {
			created = t.UTC().Format(time.RFC3339Nano)
		}
	}
	b := Backup{j.ConnectionID, id, j.Database, size, sum, created, "cloud", engine.name()}
	if err = m.saveBackup(b); err != nil {
		os.Remove(m.path(id))
		return err
	}
	if err = m.saveOffsite(Offsite{id, d.ID, j.ConnectionID, j.Database, j.RemotePath, cipherSize, cipherSum, created, now()}); err != nil {
		m.logger.Printf("offsite link for imported backup %s: %v", id, err)
	}
	m.audit("offsite.imported", d.Name+":"+j.RemotePath+" → "+id)
	return nil
}

// ---- remote retention ----------------------------------------------------

// remoteRetention deletes cloud copies older than the destination retention,
// always keeping the newest copy of every database on each destination.
func (m *Manager) remoteRetention(ctx context.Context) error {
	dests, err := m.destinations()
	if err != nil {
		return err
	}
	copies, err := m.offsiteCopies()
	if err != nil {
		return err
	}
	failures := []string{}
	for _, summary := range dests {
		if summary.RetentionDays <= 0 {
			continue
		}
		newest := map[string]string{}
		expired := []Offsite{}
		for _, o := range copies { // sorted newest first
			if o.DestinationID != summary.ID {
				continue
			}
			key := o.ConnectionID + "\x00" + o.Database
			if newest[key] == "" {
				newest[key] = o.BackupID
				continue
			}
			created, e := time.Parse(time.RFC3339Nano, o.BackupCreatedAt)
			if e == nil && time.Since(created) > time.Duration(summary.RetentionDays)*24*time.Hour {
				expired = append(expired, o)
			}
		}
		if len(expired) == 0 {
			continue
		}
		d, e := m.destination(summary.ID)
		if e != nil {
			failures = append(failures, summary.Name+": "+e.Error())
			continue
		}
		s, e := m.rclone(d)
		if e != nil {
			failures = append(failures, summary.Name+": "+e.Error())
			continue
		}
		for _, o := range expired {
			e = s.run(ctx, nil, nil, "deletefile", s.target(o.RemotePath))
			var re *rcloneError
			if e != nil && !(errors.As(e, &re) && (re.code == 3 || re.code == 4)) {
				failures = append(failures, summary.Name+":"+o.RemotePath+": "+e.Error())
				continue
			}
			if _, e = m.db.Exec(`DELETE FROM offsite WHERE backup_id=? AND destination_id=?`, o.BackupID, o.DestinationID); e != nil {
				failures = append(failures, e.Error())
				continue
			}
			m.audit("offsite.retention_deleted", d.Name+":"+o.RemotePath)
		}
		s.close()
	}
	if len(failures) > 0 {
		return fmt.Errorf("cloud retention: %s", strings.Join(failures, "; "))
	}
	return nil
}

func destinationInUse(db *sql.DB, id string) (int, error) {
	var n int
	err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM jobs WHERE destination_id=? AND status IN ('queued','running')) + (SELECT COUNT(*) FROM schedules WHERE destination_id=?)`, id, id).Scan(&n)
	return n, err
}

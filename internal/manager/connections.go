package manager

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Connection struct {
	Engine             string `json:"engine"`
	ServerDir          string `json:"server_dir"`
	LocalDir           string `json:"local_dir"`
	RestoreUsername    string `json:"restore_username"`
	RestorePassword    string `json:"restore_password,omitempty"`
	HasRestorePassword bool   `json:"has_restore_password"`
	RestoreEnabled     bool   `json:"restore_enabled"`
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Host               string `json:"host"`
	Port               string `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password,omitempty"`
	Database           string `json:"database"`
	SSLMode            string `json:"sslmode"`
	HasPassword        bool   `json:"has_password"`
}

func loadConnectionKey(metadata string) ([]byte, error) {
	path := filepath.Join(filepath.Dir(metadata), "connections.key")
	key, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return nil, e
		}
		_, err = f.Write(key)
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	} else if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid connections.key")
	}
	return key, nil
}
func (m *Manager) passwordCipher() cipher.AEAD {
	block, _ := aes.NewCipher(m.secretKey)
	gcm, _ := cipher.NewGCM(block)
	return gcm
}
func (m *Manager) sealPassword(password string) string {
	gcm := m.passwordCipher()
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(password), nil))
}
func decodeBase64(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }
func (m *Manager) storedConnection(id string) (Connection, error) {
	var c Connection
	var encrypted, restoreEncrypted string
	err := m.db.QueryRow(`SELECT id,name,host,port,username,password,database_name,sslmode,restore_username,restore_password,engine,server_dir,local_dir FROM connections WHERE id=?`, id).Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &encrypted, &c.Database, &c.SSLMode, &c.RestoreUsername, &restoreEncrypted, &c.Engine, &c.ServerDir, &c.LocalDir)
	if err != nil {
		return c, err
	}
	c.Engine = engineName(c.Engine)
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	gcm := m.passwordCipher()
	if err != nil || len(raw) < gcm.NonceSize() {
		return c, errors.New("cannot decrypt connection password")
	}
	value, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return c, errors.New("cannot decrypt connection password")
	}
	c.Password = string(value)
	c.HasPassword = c.Password != ""
	c.RestoreEnabled = c.RestoreUsername != ""
	if restoreEncrypted != "" {
		raw, err = base64.StdEncoding.DecodeString(restoreEncrypted)
		if err != nil || len(raw) < gcm.NonceSize() {
			return c, errors.New("cannot decrypt restore password")
		}
		value, err = gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
		if err != nil {
			return c, errors.New("cannot decrypt restore password")
		}
		c.RestorePassword = string(value)
	}
	c.HasRestorePassword = c.RestorePassword != ""
	return c, nil
}
func (m *Manager) forConnection(id string) (*Manager, error) {
	if id == "" {
		return m, nil
	}
	c, err := m.storedConnection(id)
	if err != nil {
		return nil, errors.New("Connection unavailable; select an existing connection")
	}
	return &Manager{cfg: m.cfg.withConnection(c), db: m.db, logger: m.logger, connectionID: id}, nil
}

// withConnection returns a copy of cfg pointing at connection c.
func (cfg Config) withConnection(c Connection) Config {
	cfg.Engine, cfg.ServerDir, cfg.LocalDir = engineName(c.Engine), c.ServerDir, c.LocalDir
	cfg.PGHost, cfg.PGPort, cfg.PGUser, cfg.PGPassword, cfg.PGDatabase, cfg.PGSSLMode = c.Host, c.Port, c.Username, c.Password, c.Database, c.SSLMode
	cfg.PGPassFile, cfg.RestorePassFile = "", ""
	cfg.RestoreUser, cfg.RestorePassword = c.RestoreUsername, c.RestorePassword
	return cfg
}
func validateConnection(c *Connection) error {
	c.Name, c.Host, c.Username, c.Database = strings.TrimSpace(c.Name), strings.TrimSpace(c.Host), strings.TrimSpace(c.Username), strings.TrimSpace(c.Database)
	c.Engine = engineName(c.Engine)
	e, ok := engineByName(c.Engine)
	if !ok {
		return errors.New("Loại database không được hỗ trợ")
	}
	if c.Engine == engineFiles {
		return validateFilesConnection(c)
	}
	port, database, sslmode := e.defaults()
	if c.Port == "" {
		c.Port = port
	}
	if c.Database == "" || database == "" {
		c.Database = database
	}
	if c.SSLMode == "" {
		c.SSLMode = sslmode
	}
	c.RestoreUsername = strings.TrimSpace(c.RestoreUsername)
	if len(c.RestoreUsername) > 128 || strings.ContainsAny(c.RestoreUsername, "\x00\r\n") || len(c.RestorePassword) > 4096 || strings.ContainsRune(c.RestorePassword, 0) {
		return errors.New("Tài khoản restore không hợp lệ")
	}
	// MongoDB servers without access control accept connections without a user.
	needUser := c.Engine != "mongodb"
	p, err := strconv.Atoi(c.Port)
	if c.Name == "" || len(c.Name) > 100 || c.Host == "" || len(c.Host) > 255 || strings.ContainsAny(c.Host, "\x00\r\n=/?#@") || (needUser && c.Username == "") || len(c.Username) > 128 || strings.ContainsAny(c.Username, "\x00\r\n") || (c.Database != "" && !dbName.MatchString(c.Database)) || err != nil || p < 1 || p > 65535 || len(c.Password) > 4096 || strings.ContainsRune(c.Password, 0) {
		return errors.New("Tên, host, port (1–65535), tài khoản hoặc database không hợp lệ")
	}
	valid := false
	for _, mode := range e.sslModes() {
		valid = valid || c.SSLMode == mode
	}
	if !valid {
		return errors.New("SSL mode không hợp lệ")
	}
	c.ServerDir, c.LocalDir = strings.TrimSpace(c.ServerDir), strings.TrimSpace(c.LocalDir)
	if c.Engine == "mssql" {
		if c.ServerDir == "" || len(c.ServerDir) > 1024 || strings.ContainsAny(c.ServerDir, "\x00\r\n'") || !validLocalDir(c.LocalDir) {
			return errors.New("Cần thư mục backup dùng chung: đường dẫn phía SQL Server và đường dẫn tuyệt đối trong container")
		}
	} else {
		c.ServerDir, c.LocalDir = "", ""
	}
	return nil
}
func (m *Manager) connectionAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" && r.URL.Path == "/api/connections" {
		rows, err := m.db.Query(`SELECT id,name,host,port,username,database_name,sslmode,engine,server_dir,local_dir FROM connections ORDER BY name,id`)
		if err != nil {
			fail(w, 500, "Cannot read connections")
			return
		}
		out := []Connection{}
		for rows.Next() {
			var c Connection
			if err = rows.Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.Database, &c.SSLMode, &c.Engine, &c.ServerDir, &c.LocalDir); err != nil {
				break
			}
			c.Engine = engineName(c.Engine)
			out = append(out, c)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			fail(w, 500, "Cannot read connections")
			return
		}
		me := principalOf(r)
		visible := out[:0]
		for _, c := range out {
			if me.canConnection(c.ID, "view") {
				visible = append(visible, c)
			}
		}
		out = visible
		// Never return credentials, including to administrators.
		for i := range out {
			c, e := m.storedConnection(out[i].ID)
			if e != nil {
				fail(w, 500, "Cannot read connection credentials")
				return
			}
			out[i].HasPassword = c.HasPassword
			out[i].RestoreUsername, out[i].RestoreEnabled, out[i].HasRestorePassword = c.RestoreUsername, c.RestoreEnabled, c.HasRestorePassword
		}
		respond(w, 200, out)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/connections/")
	if r.Method == "DELETE" && idPattern.MatchString(id) {
		m.connectionMu.Lock()
		defer m.connectionMu.Unlock()
		m.queueMu.Lock()
		defer m.queueMu.Unlock()
		m.schedMu.Lock()
		defer m.schedMu.Unlock()
		var count int
		if err := m.db.QueryRow(`SELECT (SELECT COUNT(*) FROM jobs WHERE connection_id=? AND status IN ('queued','running')) + (SELECT COUNT(*) FROM schedules WHERE connection_id=? OR verify_connection_id=?)`, id, id, id).Scan(&count); err != nil {
			fail(w, 500, "Cannot check connection usage")
			return
		}
		if count > 0 {
			fail(w, 409, "Xóa lịch backup và đợi job hoàn tất trước khi xóa kết nối")
			return
		}
		result, err := m.db.Exec(`DELETE FROM connections WHERE id=?`, id)
		if err != nil {
			fail(w, 500, "Cannot delete connection")
			return
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			fail(w, 404, "Connection not found")
			return
		}
		m.db.Exec(`DELETE FROM user_permissions WHERE connection_id=?`, id)
		m.auditReq(r, "connection.deleted", id)
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	test := r.URL.Path == "/api/connections/test"
	if (test && r.Method != "POST") || (!test && !((r.Method == "POST" && r.URL.Path == "/api/connections") || (r.Method == "PUT" && idPattern.MatchString(id)))) {
		fail(w, 405, "Method not allowed")
		return
	}
	m.connectionMu.Lock()
	defer m.connectionMu.Unlock()
	var c Connection
	if !decode(w, r, &c) {
		return
	}
	if r.Method == "PUT" {
		c.ID = id
		var active int
		if err := m.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE connection_id=? AND status IN ('queued','running')`, id).Scan(&active); err != nil {
			fail(w, 500, "Cannot check active jobs")
			return
		}
		if active > 0 {
			fail(w, 409, "Đợi job hoàn tất trước khi sửa kết nối")
			return
		}
	} else if !test {
		c.ID = ""
	}
	if c.ID != "" {
		old, err := m.storedConnection(c.ID)
		if err != nil {
			fail(w, 404, "Connection not found")
			return
		}
		if c.Password == "" && c.HasPassword {
			c.Password = old.Password
		}
		if c.RestorePassword == "" && c.HasRestorePassword {
			c.RestorePassword = old.RestorePassword
		}
	}
	if err := validateConnection(&c); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if test {
		target := &Manager{cfg: m.cfg.withConnection(c), logger: m.logger}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if err := target.engine().test(ctx, target); err != nil {
			fail(w, 400, err.Error())
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	if c.ID == "" {
		c.ID = token()
	}
	_, err := m.db.Exec(`INSERT INTO connections(id,name,host,port,username,password,database_name,sslmode,restore_username,restore_password,engine,server_dir,local_dir) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,host=excluded.host,port=excluded.port,username=excluded.username,password=excluded.password,database_name=excluded.database_name,sslmode=excluded.sslmode,restore_username=excluded.restore_username,restore_password=excluded.restore_password,engine=excluded.engine,server_dir=excluded.server_dir,local_dir=excluded.local_dir`, c.ID, c.Name, c.Host, c.Port, c.Username, m.sealPassword(c.Password), c.Database, c.SSLMode, c.RestoreUsername, m.sealPassword(c.RestorePassword), c.Engine, c.ServerDir, c.LocalDir)
	if err != nil {
		fail(w, 500, "Cannot save connection")
		return
	}
	m.auditReq(r, "connection.saved", c.ID+" "+c.Name)
	c.HasPassword = c.Password != ""
	c.HasRestorePassword = c.RestorePassword != ""
	c.RestoreEnabled = c.RestoreUsername != ""
	c.Password, c.RestorePassword = "", ""
	respond(w, 200, c)
}

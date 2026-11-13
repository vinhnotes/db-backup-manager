package manager

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Grant gives a user access to one database (or every database) on one
// connection (or every connection). ConnectionID "" is the env connection.
type Grant struct {
	ConnectionID string `json:"connection_id"`
	Database     string `json:"database"`
	Level        string `json:"level"` // "view" or "operate"
}

// Principal is the signed-in account behind a request.
type Principal struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	Role        string  `json:"role"` // "admin" or "user"
	Builtin     bool    `json:"builtin"`
	Grants      []Grant `json:"permissions"`
}

type User struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	Role        string  `json:"role"`
	Disabled    bool    `json:"disabled"`
	CreatedAt   string  `json:"created_at"`
	Password    string  `json:"password,omitempty"`
	Permissions []Grant `json:"permissions"`
}

type principalKey struct{}

const (
	envAdminID  = "env:admin"
	envViewerID = "env:viewer"
	anyValue    = "*"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{1,31}$`)

func (p *Principal) Admin() bool { return p.Role == "admin" }

// can reports whether the principal may act on database db of connection conn.
// db "*" means "every database", which needs a grant covering every database.
func (p *Principal) can(conn, db, level string) bool {
	if p.Admin() {
		return true
	}
	for _, g := range p.Grants {
		if (g.ConnectionID == anyValue || g.ConnectionID == conn) && (g.Database == anyValue || g.Database == db) && (level == "view" || g.Level == "operate") {
			return true
		}
	}
	return false
}

// canConnection reports whether the principal has any grant on conn.
func (p *Principal) canConnection(conn, level string) bool {
	if p.Admin() {
		return true
	}
	for _, g := range p.Grants {
		if (g.ConnectionID == anyValue || g.ConnectionID == conn) && (level == "view" || g.Level == "operate") {
			return true
		}
	}
	return false
}

func principalOf(r *http.Request) *Principal {
	p, _ := r.Context().Value(principalKey{}).(*Principal)
	if p == nil {
		return &Principal{Role: "user"}
	}
	return p
}

func (m *Manager) loadPrincipal(ctx context.Context, id string) (*Principal, error) {
	switch id {
	case envAdminID:
		return &Principal{ID: id, Username: m.cfg.AdminUser, DisplayName: "Administrator", Role: "admin", Builtin: true, Grants: []Grant{}}, nil
	case envViewerID:
		if len(m.viewerHash) == 0 {
			return nil, errors.New("viewer disabled")
		}
		return &Principal{ID: id, Username: "viewer", DisplayName: "Viewer", Role: "user", Builtin: true, Grants: []Grant{{anyValue, anyValue, "view"}}}, nil
	}
	p := &Principal{ID: id}
	var disabled bool
	if err := m.db.QueryRowContext(ctx, `SELECT username,display_name,role,disabled FROM users WHERE id=?`, id).Scan(&p.Username, &p.DisplayName, &p.Role, &disabled); err != nil {
		return nil, err
	}
	if disabled {
		return nil, errors.New("user disabled")
	}
	var err error
	p.Grants, err = m.grants(ctx, id)
	return p, err
}

func (m *Manager) grants(ctx context.Context, userID string) ([]Grant, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT connection_id,database_name,level FROM user_permissions WHERE user_id=? ORDER BY connection_id,database_name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Grant{}
	for rows.Next() {
		var g Grant
		if err = rows.Scan(&g.ConnectionID, &g.Database, &g.Level); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (m *Manager) users(ctx context.Context) ([]User, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT id,username,display_name,role,disabled,created_at FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	out := []User{}
	for rows.Next() {
		var u User
		if err = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Disabled, &u.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, u)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Permissions, err = m.grants(ctx, out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// cleanGrants validates grants and keeps the strongest level per scope.
func (m *Manager) cleanGrants(in []Grant) ([]Grant, error) {
	if len(in) > 500 {
		return nil, errors.New("Quá nhiều quyền")
	}
	seen := map[[2]string]int{}
	out := []Grant{}
	for _, g := range in {
		g.ConnectionID, g.Database = strings.TrimSpace(g.ConnectionID), strings.TrimSpace(g.Database)
		if g.Level != "view" && g.Level != "operate" {
			return nil, errors.New("Mức quyền không hợp lệ")
		}
		if g.Database == "" {
			g.Database = anyValue
		}
		if g.Database != anyValue && !dbName.MatchString(g.Database) {
			return nil, errors.New("Tên database trong phân quyền không hợp lệ: " + g.Database)
		}
		switch {
		case g.ConnectionID == anyValue:
		case g.ConnectionID == "" && m.cfg.PGHost != "":
		default:
			var n int
			if !idPattern.MatchString(g.ConnectionID) || m.db.QueryRow(`SELECT COUNT(*) FROM connections WHERE id=?`, g.ConnectionID).Scan(&n) != nil || n == 0 {
				return nil, errors.New("Kết nối trong phân quyền không tồn tại")
			}
		}
		key := [2]string{g.ConnectionID, g.Database}
		if i, ok := seen[key]; ok {
			if g.Level == "operate" {
				out[i].Level = "operate"
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, g)
	}
	return out, nil
}

func validPassword(p string) bool { return len(p) >= 12 && len(p) <= 72 }

func (m *Manager) userAPI(w http.ResponseWriter, r *http.Request) {
	me := principalOf(r)
	if !me.Admin() {
		fail(w, 403, "Admin role required")
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/users"), "/")
	switch {
	case r.Method == "GET" && id == "":
		list, err := m.users(r.Context())
		if err != nil {
			fail(w, 500, "Cannot read users")
			return
		}
		respond(w, 200, list)
	case r.Method == "POST" && id == "":
		m.saveUser(w, r, "")
	case r.Method == "PUT" && idPattern.MatchString(id):
		m.saveUser(w, r, id)
	case r.Method == "DELETE" && idPattern.MatchString(id):
		if id == me.ID {
			fail(w, 400, "Không thể xóa tài khoản đang đăng nhập")
			return
		}
		tx, err := m.db.Begin()
		if err != nil {
			fail(w, 500, "Cannot delete user")
			return
		}
		defer tx.Rollback()
		res, err := tx.Exec(`DELETE FROM users WHERE id=?`, id)
		if err != nil {
			fail(w, 500, "Cannot delete user")
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			fail(w, 404, "Người dùng không tồn tại")
			return
		}
		if _, err = tx.Exec(`DELETE FROM user_permissions WHERE user_id=?`, id); err == nil {
			_, err = tx.Exec(`DELETE FROM sessions WHERE user_id=?`, id)
		}
		if err != nil || tx.Commit() != nil {
			fail(w, 500, "Cannot delete user")
			return
		}
		m.auditReq(r, "user.deleted", id)
		respond(w, 200, map[string]bool{"ok": true})
	default:
		fail(w, 404, "Endpoint not found")
	}
}

func (m *Manager) saveUser(w http.ResponseWriter, r *http.Request, id string) {
	var u User
	if !decode(w, r, &u) {
		return
	}
	u.Username, u.DisplayName = strings.TrimSpace(u.Username), strings.TrimSpace(u.DisplayName)
	if !usernamePattern.MatchString(u.Username) {
		fail(w, 400, "Tên đăng nhập gồm 2–32 ký tự: chữ, số, dấu chấm, gạch dưới hoặc gạch ngang")
		return
	}
	if strings.EqualFold(u.Username, m.cfg.AdminUser) || strings.EqualFold(u.Username, "viewer") {
		fail(w, 400, "Tên đăng nhập này đã được dành cho tài khoản hệ thống")
		return
	}
	if u.DisplayName == "" {
		u.DisplayName = u.Username
	}
	if len(u.DisplayName) > 100 || strings.ContainsAny(u.DisplayName, "\x00\r\n") {
		fail(w, 400, "Tên hiển thị không hợp lệ")
		return
	}
	if u.Role != "admin" && u.Role != "user" {
		fail(w, 400, "Vai trò không hợp lệ")
		return
	}
	if (id == "" || u.Password != "") && !validPassword(u.Password) {
		fail(w, 400, "Mật khẩu cần 12–72 ký tự")
		return
	}
	me := principalOf(r)
	if id != "" && id == me.ID && (u.Role != "admin" || u.Disabled) {
		fail(w, 400, "Không thể tự hạ quyền hoặc khóa tài khoản đang đăng nhập")
		return
	}
	grants, err := m.cleanGrants(u.Permissions)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	if u.Role == "admin" {
		grants = []Grant{}
	}
	var hash []byte
	if u.Password != "" {
		if hash, err = bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost); err != nil {
			fail(w, 500, "Cannot hash password")
			return
		}
	}
	tx, err := m.db.Begin()
	if err != nil {
		fail(w, 500, "Cannot save user")
		return
	}
	defer tx.Rollback()
	var taken int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM users WHERE username=? COLLATE NOCASE AND id<>?`, u.Username, id).Scan(&taken); err != nil {
		fail(w, 500, "Cannot save user")
		return
	}
	if taken > 0 {
		fail(w, 409, "Tên đăng nhập đã tồn tại")
		return
	}
	if id == "" {
		u.ID, u.CreatedAt = token(), now()
		_, err = tx.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,disabled,created_at) VALUES(?,?,?,?,?,?,?)`, u.ID, u.Username, u.DisplayName, string(hash), u.Role, u.Disabled, u.CreatedAt)
	} else {
		u.ID = id
		var res sql.Result
		res, err = tx.Exec(`UPDATE users SET username=?,display_name=?,role=?,disabled=? WHERE id=?`, u.Username, u.DisplayName, u.Role, u.Disabled, id)
		if err == nil {
			if n, _ := res.RowsAffected(); n == 0 {
				fail(w, 404, "Người dùng không tồn tại")
				return
			}
		}
		if err == nil && hash != nil {
			if _, err = tx.Exec(`UPDATE users SET password_hash=? WHERE id=?`, string(hash), id); err == nil && id != me.ID {
				_, err = tx.Exec(`DELETE FROM sessions WHERE user_id=?`, id)
			}
		}
		if err == nil && u.Disabled {
			_, err = tx.Exec(`DELETE FROM sessions WHERE user_id=?`, id)
		}
		if err == nil {
			err = tx.QueryRow(`SELECT created_at FROM users WHERE id=?`, id).Scan(&u.CreatedAt)
		}
	}
	if err == nil {
		_, err = tx.Exec(`DELETE FROM user_permissions WHERE user_id=?`, u.ID)
	}
	for _, g := range grants {
		if err != nil {
			break
		}
		_, err = tx.Exec(`INSERT INTO user_permissions(user_id,connection_id,database_name,level) VALUES(?,?,?,?)`, u.ID, g.ConnectionID, g.Database, g.Level)
	}
	if err != nil || tx.Commit() != nil {
		fail(w, 500, "Cannot save user")
		return
	}
	m.auditReq(r, "user.saved", u.Username+" ("+u.Role+")")
	u.Password, u.Permissions = "", grants
	respond(w, 200, u)
}

// changeOwnPassword lets a database-backed user rotate their own password.
func (m *Manager) changeOwnPassword(w http.ResponseWriter, r *http.Request) {
	me := principalOf(r)
	var b struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if !decode(w, r, &b) {
		return
	}
	if me.Builtin && me.ID != envAdminID {
		fail(w, 400, "Mật khẩu của tài khoản viewer được cấu hình trong file .env")
		return
	}
	if !validPassword(b.New) {
		fail(w, 400, "Mật khẩu mới cần 12–72 ký tự")
		return
	}
	if me.ID == envAdminID {
		m.changeAdminPassword(w, r, b.Current, b.New)
		return
	}
	var hash string
	if err := m.db.QueryRow(`SELECT password_hash FROM users WHERE id=?`, me.ID).Scan(&hash); err != nil {
		fail(w, 404, "Người dùng không tồn tại")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(b.Current)) != nil {
		fail(w, 400, "Mật khẩu hiện tại không đúng")
		return
	}
	next, err := bcrypt.GenerateFromPassword([]byte(b.New), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 500, "Cannot hash password")
		return
	}
	// Keep the current session; sign out every other one.
	c, _ := r.Cookie("dbbm_session")
	if _, err = m.db.Exec(`UPDATE users SET password_hash=? WHERE id=?`, string(next), me.ID); err == nil {
		_, err = m.db.Exec(`DELETE FROM sessions WHERE user_id=? AND token<>?`, me.ID, hashToken(c.Value))
	}
	if err != nil {
		fail(w, 500, "Cannot change password")
		return
	}
	m.auditReq(r, "user.password_changed", me.Username)
	respond(w, 200, map[string]bool{"ok": true})
}

// nonAdminAllowed is the allow-list of routes a non-admin may call at all;
// handlers then check per-connection and per-database grants.
func nonAdminAllowed(r *http.Request) bool {
	p := r.URL.Path
	if r.Method == "GET" {
		return !strings.HasPrefix(p, "/api/users")
	}
	switch {
	case p == "/api/logout", p == "/api/me/password", p == "/api/backups", p == "/api/restore", p == "/api/upload", p == "/api/schedules":
		return r.Method == "POST"
	case strings.HasPrefix(p, "/api/schedules/"):
		return r.Method == "DELETE"
	case strings.HasPrefix(p, "/api/backups/"):
		return r.Method == "DELETE" || (r.Method == "POST" && (strings.HasSuffix(p, "/offsite") || strings.HasSuffix(p, "/verify")))
	}
	return false
}

func (m *Manager) auditReq(r *http.Request, action, detail string) {
	m.auditBy(principalOf(r).Username, action, detail)
}

// authenticate returns the principal id and role for valid credentials, or "".
// Every path runs one bcrypt comparison so timing does not reveal usernames.
func (m *Manager) authenticate(username, password string) (string, string) {
	if subtle.ConstantTimeCompare([]byte(username), []byte(m.cfg.AdminUser)) == 1 {
		if bcrypt.CompareHashAndPassword(m.currentAdminHash(), []byte(password)) == nil {
			return envAdminID, "admin"
		}
		return "", ""
	}
	if username == "viewer" && len(m.viewerHash) > 0 {
		if bcrypt.CompareHashAndPassword(m.viewerHash, []byte(password)) == nil {
			return envViewerID, "user"
		}
		return "", ""
	}
	var id, hash, role string
	var disabled bool
	err := m.db.QueryRow(`SELECT id,password_hash,role,disabled FROM users WHERE username=? COLLATE NOCASE`, username).Scan(&id, &hash, &role, &disabled)
	if err != nil {
		bcrypt.CompareHashAndPassword(m.currentAdminHash(), []byte(password))
		return "", ""
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil || disabled {
		return "", ""
	}
	return id, role
}

// The .env admin password can be replaced from the web UI. The replacement
// is stored in the settings table and read on every login; editing
// ADMIN_PASSWORD in .env does not override it. The reset-admin-password
// command removes it, which takes effect without a restart.
const (
	settingAdminHash = "admin_password_hash"
	// Written by an earlier version; removed whenever the password changes.
	settingAdminEnvHashLegacy = "admin_env_password_hash"
)

func (m *Manager) currentAdminHash() []byte {
	var hash string
	if err := m.db.QueryRow(`SELECT value FROM settings WHERE key=?`, settingAdminHash).Scan(&hash); err == nil && hash != "" {
		return []byte(hash)
	}
	return m.adminHash
}

func (m *Manager) changeAdminPassword(w http.ResponseWriter, r *http.Request, current, next string) {
	if bcrypt.CompareHashAndPassword(m.currentAdminHash(), []byte(current)) != nil {
		fail(w, 400, "Mật khẩu hiện tại không đúng")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 500, "Cannot hash password")
		return
	}
	c, _ := r.Cookie("dbbm_session")
	// Keep the current session; sign out every other admin session.
	if err = setAdminPassword(m.db, string(hash), hashToken(c.Value)); err != nil {
		fail(w, 500, "Cannot change password")
		return
	}
	m.auditReq(r, "user.password_changed", m.cfg.AdminUser)
	respond(w, 200, map[string]bool{"ok": true})
}

// setAdminPassword stores hash ("" = use ADMIN_PASSWORD from .env again) and
// signs out every admin session except keepToken.
func setAdminPassword(db *sql.DB, hash, keepToken string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if hash == "" {
		_, err = tx.Exec(`DELETE FROM settings WHERE key=?`, settingAdminHash)
	} else {
		_, err = tx.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, settingAdminHash, hash)
	}
	if err == nil {
		_, err = tx.Exec(`DELETE FROM settings WHERE key=?`, settingAdminEnvHashLegacy)
	}
	if err == nil {
		_, err = tx.Exec(`DELETE FROM sessions WHERE user_id=? AND token<>?`, envAdminID, keepToken)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ResetAdminPassword is the recovery path for a forgotten admin password: it
// drops the password set from the web UI so ADMIN_PASSWORD from .env applies
// again. It works on the metadata database directly and is safe while the
// server runs.
func ResetAdminPassword(metadataDB string) error {
	if _, err := os.Stat(metadataDB); err != nil {
		return fmt.Errorf("metadata database not found at %s (set METADATA_DB): %w", metadataDB, err)
	}
	db, err := sql.Open("sqlite", metadataDB)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err = db.Exec(`PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY, value TEXT NOT NULL); CREATE TABLE IF NOT EXISTS sessions(token TEXT PRIMARY KEY, role TEXT NOT NULL, expires_at INTEGER NOT NULL, user_id TEXT NOT NULL DEFAULT '')`); err != nil {
		return err
	}
	if err = setAdminPassword(db, "", ""); err != nil {
		return err
	}
	db.Exec(`INSERT INTO audit(action,detail,created_at,actor) VALUES(?,?,?,?)`, "auth.admin_password_reset", "back to ADMIN_PASSWORD", now(), "cli")
	return nil
}

package manager

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

func fail(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	writeJSON(w, map[string]string{"error": message})
}
func respond(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	writeJSON(w, v)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, 400, "Invalid request body")
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		fail(w, 400, "Expected a single JSON value")
		return false
	}
	return true
}
func hashToken(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func (m *Manager) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if err := m.db.PingContext(r.Context()); err != nil {
			fail(w, 503, "Metadata unavailable")
			return
		}
		respond(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /metrics", m.metrics)
	mux.HandleFunc("GET /api/branding", m.branding)
	mux.HandleFunc("GET /api/branding/logo", m.brandLogo)
	mux.HandleFunc("GET /api/branding/favicon", m.favicon)
	mux.HandleFunc("GET /favicon.ico", m.favicon)
	mux.HandleFunc("POST /api/login", m.login)
	// OAuth returns cross-site, so the Strict session cookie is unavailable.
	// The callback uses a separate Lax binding cookie and revalidates the
	// initiating admin session before accepting a one-time state.
	mux.HandleFunc("GET /api/destinations/google/callback", m.googleOAuthCallback)
	mux.Handle("/api/", m.protect(http.HandlerFunc(m.api)))
	fs := http.FileServer(http.Dir(m.cfg.WebDir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if r.URL.Path != "/" {
			if _, err := os.Stat(filepath.Join(m.cfg.WebDir, filepath.Clean("/"+r.URL.Path))); err != nil {
				http.NotFound(w, r)
				return
			}
		}
		fs.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; img-src 'self' data:; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if r.Header.Get("X-Requested-With") != "DBBackupManager" {
				fail(w, 403, "Missing request protection header")
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
					fail(w, 403, "Cross-origin request denied")
					return
				}
			}
		}
		mux.ServeHTTP(w, r)
	})
}

// loginLimit is how many logins an address may try per 15 minutes.
const loginLimit = 10

func (m *Manager) login(w http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	m.loginMu.Lock()
	a := m.attempts[ip]
	if time.Now().After(a.until) {
		a = attempt{until: time.Now().Add(15 * time.Minute)}
	}
	if a.count >= loginLimit {
		m.loginMu.Unlock()
		fail(w, 429, "Too many attempts. Try again in 15 minutes.")
		return
	}
	a.count++
	if len(m.attempts) > 4096 {
		for k, v := range m.attempts {
			if time.Now().After(v.until) {
				delete(m.attempts, k)
			}
		}
	}
	m.attempts[ip] = a
	m.loginMu.Unlock()
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	principalID, role := m.authenticate(body.Username, body.Password)
	if principalID == "" {
		// The 10th failure locks the address out for 15 minutes: say so once.
		m.loginMu.Lock()
		locked := m.attempts[ip].count == loginLimit
		m.loginMu.Unlock()
		if locked {
			m.audit("login.locked", ip)
			m.notify(Event{Kind: eventSecurity, Target: ip})
		}
		fail(w, 401, "Tên đăng nhập hoặc mật khẩu không đúng")
		return
	}
	m.loginMu.Lock()
	delete(m.attempts, ip)
	m.loginMu.Unlock()
	t := token() + token()
	expires := time.Now().Add(12 * time.Hour)
	if _, err := m.db.Exec(`INSERT INTO sessions(token,role,expires_at,user_id) VALUES(?,?,?,?)`, hashToken(t), role, expires.Unix(), principalID); err != nil {
		fail(w, 500, "Cannot create session")
		return
	}
	m.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().Unix())
	http.SetCookie(w, &http.Cookie{Name: "dbbm_session", Value: t, Path: "/", HttpOnly: true, Secure: m.cfg.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: 43200, Expires: expires})
	m.auditBy(body.Username, "auth.login", body.Username)
	respond(w, 200, map[string]string{"role": role})
}
func (m *Manager) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("dbbm_session")
		if err != nil {
			fail(w, 401, "Login required")
			return
		}
		var userID string
		if err = m.db.QueryRowContext(r.Context(), `SELECT user_id FROM sessions WHERE token=? AND expires_at>?`, hashToken(c.Value), time.Now().Unix()).Scan(&userID); err != nil {
			fail(w, 401, "Session expired")
			return
		}
		p, err := m.loadPrincipal(r.Context(), userID)
		if err != nil {
			m.db.Exec(`DELETE FROM sessions WHERE token=?`, hashToken(c.Value))
			fail(w, 401, "Session expired")
			return
		}
		if !p.Admin() && !nonAdminAllowed(r) {
			fail(w, 403, "Admin role required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}
func (m *Manager) api(w http.ResponseWriter, r *http.Request) {
	p := principalOf(r)
	switch {
	case r.URL.Path == "/api/users" || strings.HasPrefix(r.URL.Path, "/api/users/"):
		m.userAPI(w, r)
	case r.Method == "POST" && r.URL.Path == "/api/me/password":
		m.changeOwnPassword(w, r)
	case r.URL.Path == "/api/connections" || strings.HasPrefix(r.URL.Path, "/api/connections/"):
		m.connectionAPI(w, r)
	case r.URL.Path == "/api/destinations" || strings.HasPrefix(r.URL.Path, "/api/destinations/"):
		m.destinationAPI(w, r)
	case r.Method == "GET" && r.URL.Path == "/api/offsite":
		v, err := m.offsiteCopies()
		if err != nil {
			fail(w, 500, "Cannot read cloud copies")
			return
		}
		out := v[:0]
		for _, o := range v {
			if p.can(o.ConnectionID, o.Database, "view") {
				out = append(out, o)
			}
		}
		respond(w, 200, out)
	case r.Method == "GET" && r.URL.Path == "/api/me":
		respond(w, 200, map[string]any{"offsite_available": m.offsiteAvailable(), "role": p.Role, "user_id": p.ID, "username": p.Username, "display_name": p.DisplayName, "builtin": p.Builtin, "permissions": p.Grants, "timezone": m.timezone(), "default_timezone": m.cfg.Timezone, "restore_enabled": m.cfg.RestoreUser != "", "restore_host": m.cfg.PGHost, "max_upload_bytes": m.cfg.MaxUpload})
	case r.Method == "POST" && r.URL.Path == "/api/logout":
		c, _ := r.Cookie("dbbm_session")
		m.db.Exec(`DELETE FROM sessions WHERE token=?`, hashToken(c.Value))
		http.SetCookie(w, &http.Cookie{Name: "dbbm_session", Value: "", Path: "/", HttpOnly: true, Secure: m.cfg.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		respond(w, 200, map[string]bool{"ok": true})
	case r.Method == "GET" && r.URL.Path == "/api/databases":
		connectionID := r.URL.Query().Get("connection_id")
		if !p.canConnection(connectionID, "view") {
			fail(w, 403, "Bạn không có quyền với kết nối này")
			return
		}
		target, err := m.forConnection(connectionID)
		if err != nil {
			fail(w, 404, err.Error())
			return
		}
		dbs, version, err := target.databases(r.Context())
		if err != nil {
			fail(w, 503, err.Error())
			return
		}
		visible := []Database{}
		for _, d := range dbs {
			if p.can(connectionID, d.Name, "view") {
				visible = append(visible, d)
			}
		}
		respond(w, 200, map[string]any{"databases": visible, "version": version, "host": target.cfg.PGHost, "engine": target.engine().name()})
	case r.URL.Path == "/api/notifiers" || strings.HasPrefix(r.URL.Path, "/api/notifiers/"):
		m.notifierAPI(w, r)
	case r.Method == "GET" && r.URL.Path == "/api/backups":
		v, err := m.backupViews()
		if err != nil {
			fail(w, 500, "Cannot read backups")
			return
		}
		out := v[:0]
		for _, b := range v {
			if p.can(b.ConnectionID, b.Database, "view") {
				out = append(out, b)
			}
		}
		respond(w, 200, out)
	case r.Method == "GET" && r.URL.Path == "/api/audit":
		query, args := `SELECT action,detail,created_at,actor FROM audit ORDER BY id DESC LIMIT 100`, []any{}
		if !p.Admin() {
			query, args = `SELECT action,detail,created_at,actor FROM audit WHERE actor=? ORDER BY id DESC LIMIT 100`, []any{p.Username}
		}
		rows, err := m.db.Query(query, args...)
		if err != nil {
			fail(w, 500, "Cannot read audit log")
			return
		}
		defer rows.Close()
		events := []map[string]string{}
		for rows.Next() {
			var action, detail, created, actor string
			if err = rows.Scan(&action, &detail, &created, &actor); err != nil {
				fail(w, 500, "Cannot read audit log")
				return
			}
			events = append(events, map[string]string{"action": action, "detail": detail, "created_at": created, "actor": actor})
		}
		if rows.Err() != nil {
			fail(w, 500, "Cannot read audit log")
			return
		}
		respond(w, 200, events)
	case r.Method == "GET" && r.URL.Path == "/api/jobs":
		v, err := m.jobs()
		if err != nil {
			fail(w, 500, "Cannot read jobs")
			return
		}
		out := v[:0]
		for _, j := range v {
			if p.can(j.ConnectionID, j.Database, "view") {
				out = append(out, j)
			}
		}
		respond(w, 200, out)
	case r.Method == "GET" && r.URL.Path == "/api/schedules":
		m.schedMu.Lock()
		defer m.schedMu.Unlock()
		ss, err := m.schedules()
		if err != nil {
			fail(w, 500, "Cannot read schedules")
			return
		}
		visible := ss[:0]
		for _, s := range ss {
			if p.can(s.ConnectionID, s.Database, "view") {
				visible = append(visible, s)
			}
		}
		ss = visible
		loc, _ := time.LoadLocation(m.timezone())
		for i := range ss {
			if ss[i].Enabled {
				if s, e := cron.ParseStandard(ss[i].Cron); e == nil {
					ss[i].NextRun = s.Next(time.Now().In(loc)).Format(time.RFC3339)
				}
			}
		}
		respond(w, 200, ss)
	case r.Method == "POST" && r.URL.Path == "/api/backups":
		m.startBackup(w, r)
	case r.Method == "POST" && r.URL.Path == "/api/restore":
		m.startRestore(w, r)
	case r.Method == "POST" && r.URL.Path == "/api/upload":
		m.upload(w, r)
	case r.URL.Path == "/api/settings/branding" || r.URL.Path == "/api/settings/branding/logo":
		m.brandingAPI(w, r)
	case r.Method == "PUT" && r.URL.Path == "/api/settings/timezone":
		m.setTimezone(w, r)
	case r.Method == "POST" && r.URL.Path == "/api/schedules":
		m.saveSchedule(w, r)
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/schedules/"):
		m.deleteSchedule(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/backups/"):
		m.backupResource(w, r)
	default:
		fail(w, 404, "Endpoint not found")
	}
}
func (m *Manager) startBackup(w http.ResponseWriter, r *http.Request) {
	m.connectionMu.Lock()
	defer m.connectionMu.Unlock()
	var b struct {
		Database      string `json:"database"`
		ConnectionID  string `json:"connection_id"`
		DestinationID string `json:"destination_id"`
	}
	if !decode(w, r, &b) {
		return
	}
	if b.Database != "*" && !dbName.MatchString(b.Database) {
		fail(w, 400, "Invalid database name")
		return
	}
	if !principalOf(r).can(b.ConnectionID, b.Database, "operate") {
		fail(w, 403, "Bạn không có quyền backup database này")
		return
	}
	if b.DestinationID != "" {
		if _, err := m.destination(b.DestinationID); err != nil {
			fail(w, 404, err.Error())
			return
		}
	}
	target, err := m.forConnection(b.ConnectionID)
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	dbs, _, err := target.databases(r.Context())
	if err != nil {
		fail(w, 503, err.Error())
		return
	}
	names := []string{}
	for _, d := range dbs {
		if b.Database == "*" || d.Name == b.Database {
			if !dbName.MatchString(d.Name) {
				fail(w, 400, "Database names must use letters, digits, underscores or hyphens")
				return
			}
			names = append(names, d.Name)
		}
	}
	if len(names) == 0 {
		fail(w, 404, "Database not found")
		return
	}
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	if len(m.queue)+len(names) > cap(m.queue) {
		fail(w, 409, "Too many queued jobs")
		return
	}
	jobs := []Job{}
	for _, name := range names {
		j, err := m.enqueueJobLocked(Job{ConnectionID: b.ConnectionID, Kind: "backup", Database: name, DestinationID: b.DestinationID})
		if err != nil {
			fail(w, 500, "Could not persist backup job")
			return
		}
		jobs = append(jobs, j)
	}
	respond(w, 202, jobs)
}
func (m *Manager) startRestore(w http.ResponseWriter, r *http.Request) {
	m.connectionMu.Lock()
	defer m.connectionMu.Unlock()
	var b struct {
		ConnectionID string `json:"connection_id"`
		BackupID     string `json:"backup_id"`
		Target       string `json:"target"`
		Confirmation string `json:"confirmation"`
		Trusted      bool   `json:"trusted"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !dbName.MatchString(b.Target) || b.Target != b.Confirmation || !b.Trusted {
		fail(w, 400, "Use a new database name, repeat it exactly and confirm archive trust")
		return
	}
	// System databases of the target engine are never restore targets.
	if t, err := m.forConnection(b.ConnectionID); err == nil && t.engine().reserved(b.Target) {
		fail(w, 400, "Use a new database name, repeat it exactly and confirm archive trust")
		return
	}
	source, err := m.backup(b.BackupID)
	if err != nil {
		fail(w, 404, "Backup not found")
		return
	}
	if p := principalOf(r); !p.can(source.ConnectionID, source.Database, "operate") || !p.can(b.ConnectionID, "*", "operate") {
		fail(w, 403, "Restore cần quyền vận hành trên bản backup và trên toàn bộ kết nối đích")
		return
	}
	target, err := m.forConnection(b.ConnectionID)
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	if source.Engine != target.engine().name() {
		fail(w, 400, fmt.Sprintf("Không thể khôi phục bản backup %s vào máy chủ %s", engineLabel(source.Engine), target.engine().label()))
		return
	}
	if target.cfg.RestoreUser == "" || target.cfg.PGHost == "" {
		fail(w, 400, "Cấu hình tài khoản restore cho Server đích trước khi khôi phục")
		return
	}
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	j, err := m.enqueueForLocked("restore", b.Target, b.BackupID, b.ConnectionID)
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	m.notify(Event{Kind: eventRestore, Job: j, ConnectionID: b.ConnectionID, Database: b.Target, Actor: principalOf(r).Username, Source: source})
	respond(w, 202, j)
}
func (m *Manager) saveSchedule(w http.ResponseWriter, r *http.Request) {
	m.connectionMu.Lock()
	defer m.connectionMu.Unlock()
	var s Schedule
	if !decode(w, r, &s) {
		return
	}
	if _, err := m.forConnection(s.ConnectionID); err != nil {
		fail(w, 404, err.Error())
		return
	}
	if s.Database != "*" && !dbName.MatchString(s.Database) {
		fail(w, 400, "Invalid database name")
		return
	}
	if p := principalOf(r); !p.can(s.ConnectionID, s.Database, "operate") {
		fail(w, 403, "Bạn không có quyền tạo lịch cho database này")
		return
	} else if s.ID != 0 {
		var oldConn, oldDB string
		if err := m.db.QueryRow(`SELECT connection_id,database_name FROM schedules WHERE id=?`, s.ID).Scan(&oldConn, &oldDB); err != nil {
			fail(w, 404, "Schedule not found")
			return
		}
		if !p.can(oldConn, oldDB, "operate") {
			fail(w, 403, "Bạn không có quyền sửa lịch này")
			return
		}
	}
	if s.DestinationID != "" {
		if _, err := m.destination(s.DestinationID); err != nil {
			fail(w, 404, err.Error())
			return
		}
	}
	if strings.HasPrefix(s.Cron, "@") || len(strings.Fields(s.Cron)) != 5 {
		fail(w, 400, "Use a five-field cron expression")
		return
	}
	if _, err := cron.ParseStandard(s.Cron); err != nil {
		fail(w, 400, "Invalid cron expression")
		return
	}
	if s.RetentionDays < 1 || s.RetentionDays > 3650 {
		fail(w, 400, "Retention must be between 1 and 3650 days")
		return
	}
	if !s.Verify {
		s.VerifyConnectionID = ""
	} else {
		target := s.VerifyConnectionID
		if target == "" {
			target = s.ConnectionID
		}
		source, _ := m.forConnection(s.ConnectionID)
		if err := m.checkVerifyTarget(principalOf(r), target, source.engine().name()); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	m.schedMu.Lock()
	defer m.schedMu.Unlock()
	var err error
	if s.ID == 0 {
		_, err = m.db.Exec(`INSERT INTO schedules(connection_id,database_name,cron,retention_days,enabled,destination_id,verify,verify_connection_id,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, s.ConnectionID, s.Database, s.Cron, s.RetentionDays, s.Enabled, s.DestinationID, s.Verify, s.VerifyConnectionID, now())
	} else {
		var result sql.Result
		result, err = m.db.Exec(`UPDATE schedules SET connection_id=?,database_name=?,cron=?,retention_days=?,enabled=?,destination_id=?,verify=?,verify_connection_id=?,updated_at=? WHERE id=?`, s.ConnectionID, s.Database, s.Cron, s.RetentionDays, s.Enabled, s.DestinationID, s.Verify, s.VerifyConnectionID, now(), s.ID)
		if err == nil {
			n, _ := result.RowsAffected()
			if n == 0 {
				fail(w, 404, "Schedule not found")
				return
			}
		}
	}
	if err != nil {
		fail(w, 500, "Cannot save schedule")
		return
	}
	if err = m.loadSchedules(); err != nil {
		fail(w, 500, "Saved but scheduler reload failed")
		return
	}
	m.auditReq(r, "schedule.saved", s.Database+" "+s.Cron)
	respond(w, 200, map[string]bool{"ok": true})
}
func (m *Manager) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	m.schedMu.Lock()
	defer m.schedMu.Unlock()
	id := strings.TrimPrefix(r.URL.Path, "/api/schedules/")
	var conn, database string
	if err := m.db.QueryRow(`SELECT connection_id,database_name FROM schedules WHERE id=?`, id).Scan(&conn, &database); err != nil {
		fail(w, 404, "Schedule not found")
		return
	}
	if !principalOf(r).can(conn, database, "operate") {
		fail(w, 403, "Bạn không có quyền xóa lịch này")
		return
	}
	res, err := m.db.Exec(`DELETE FROM schedules WHERE id=?`, id)
	if err != nil {
		fail(w, 500, "Cannot delete schedule")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "Schedule not found")
		return
	}
	if err = m.loadSchedules(); err != nil {
		fail(w, 500, "Cannot reload scheduler")
		return
	}
	m.auditReq(r, "schedule.deleted", id)
	respond(w, 200, map[string]bool{"ok": true})
}
func (m *Manager) backupResource(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/backups/")
	download := strings.HasSuffix(id, "/download")
	offsite := strings.HasSuffix(id, "/offsite")
	verify := strings.HasSuffix(id, "/verify")
	id = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(id, "/download"), "/offsite"), "/verify")
	if !idPattern.MatchString(id) {
		fail(w, 404, "Backup not found")
		return
	}
	b, err := m.backup(id)
	if err != nil || !principalOf(r).can(b.ConnectionID, b.Database, "view") {
		fail(w, 404, "Backup not found")
		return
	}
	// Downloading, deleting and copying to cloud all need operate rights.
	if !principalOf(r).can(b.ConnectionID, b.Database, "operate") {
		fail(w, 403, "Bạn chỉ có quyền xem bản backup này")
		return
	}
	if offsite {
		if r.Method != "POST" {
			fail(w, 405, "Method not allowed")
			return
		}
		m.startOffsiteUpload(w, r, b)
		return
	}
	if verify {
		if r.Method != "POST" {
			fail(w, 405, "Method not allowed")
			return
		}
		m.startVerify(w, r, b)
		return
	}
	if download && r.Method == "GET" {
		f, err := os.Open(m.path(id))
		if err != nil {
			fail(w, 404, "Backup file missing")
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			fail(w, 500, "Cannot read backup")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		ext := postgresEngine{}.ext()
		if e, ok := engineByName(b.Engine); ok {
			ext = e.ext()
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s%s"`, b.Database, b.ID, ext))
		w.Header().Set("X-Checksum-SHA256", b.Checksum)
		m.auditReq(r, "backup.downloaded", id)
		http.ServeContent(w, r, b.Database+ext, info.ModTime(), f)
		return
	}
	if !download && r.Method == "DELETE" {
		if !m.opMu.TryLock() {
			fail(w, 409, "A job or upload is running; retry when finished")
			return
		}
		defer m.opMu.Unlock()
		m.queueMu.Lock()
		defer m.queueMu.Unlock()
		var count int
		if err = m.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE backup_id=? AND status IN ('queued','running')`, id).Scan(&count); err != nil {
			fail(w, 500, "Cannot check active jobs")
			return
		}
		if count > 0 {
			fail(w, 409, "Backup is referenced by an active restore")
			return
		}
		if err = m.removeBackup(b); err != nil {
			fail(w, 500, "Cannot delete backup")
			return
		}
		m.auditReq(r, "backup.deleted", id)
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	fail(w, 405, "Method not allowed")
}
func (m *Manager) upload(w http.ResponseWriter, r *http.Request) {
	m.connectionMu.Lock()
	defer m.connectionMu.Unlock()
	connectionID := r.Header.Get("X-Connection-ID")
	target, err := m.forConnection(connectionID)
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	engine := target.engine()
	database := r.Header.Get("X-Database-Name")
	if !dbName.MatchString(database) {
		fail(w, 400, "Invalid X-Database-Name")
		return
	}
	if !principalOf(r).can(connectionID, database, "operate") {
		fail(w, 403, "Bạn không có quyền upload cho database này")
		return
	}
	if !m.opMu.TryLock() {
		fail(w, 409, "A job or upload is running; retry when finished")
		return
	}
	defer m.opMu.Unlock()
	if err := m.checkSpace(r.ContentLength); err != nil {
		fail(w, 507, err.Error())
		return
	}
	id := token()
	partial := m.path(id) + ".partial"
	f, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		fail(w, 500, "Cannot create backup file")
		return
	}
	defer os.Remove(partial)
	r.Body = http.MaxBytesReader(w, r.Body, m.cfg.MaxUpload)
	_, err = io.Copy(f, r.Body)
	closeErr := f.Close()
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			fail(w, 413, "Upload exceeds configured limit")
		} else {
			fail(w, 400, "Upload interrupted")
		}
		return
	}
	if closeErr != nil {
		fail(w, 500, "Cannot write backup file")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	if n, ok := engine.(uploadPreparer); ok {
		if err = n.prepareUpload(partial); err != nil {
			fail(w, 500, "Cannot prepare upload")
			return
		}
	}
	if err = engine.verify(ctx, target, partial); err != nil {
		fail(w, 400, fmt.Sprintf("File không phải bản backup %s hợp lệ", engine.label()))
		return
	}
	sum, size, err := checksum(partial)
	if err != nil {
		fail(w, 500, "Cannot checksum upload")
		return
	}
	if err = os.Rename(partial, m.path(id)); err != nil {
		fail(w, 500, "Cannot finalize upload")
		return
	}
	b := Backup{connectionID, id, database, size, sum, now(), "upload", engine.name()}
	if err = m.saveBackup(b); err != nil {
		os.Remove(m.path(id))
		fail(w, 500, "Cannot save backup metadata")
		return
	}
	m.auditReq(r, "backup.uploaded", id)
	respond(w, 201, b)
}

package manager

import (
	"context"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"filippo.io/age"
)

func (m *Manager) offsiteAvailable() bool {
	_, err := exec.LookPath(m.cfg.RcloneBinary)
	return err == nil
}

// destinationAPI serves /api/destinations and its sub-resources.
func (m *Manager) destinationAPI(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/destinations"), "/")
	id, action, _ := strings.Cut(rest, "/")
	switch {
	case rest == "google/oauth" || rest == "google/oauth/start" || rest == "google/oauth/status":
		m.googleOAuthAPI(w, r, rest)
	case r.Method == "GET" && rest == "":
		list, err := m.destinations()
		if err != nil {
			fail(w, 500, "Cannot read destinations")
			return
		}
		respond(w, 200, list)
	case r.Method == "POST" && rest == "keygen":
		identity, err := age.GenerateX25519Identity()
		if err != nil {
			fail(w, 500, "Cannot generate key")
			return
		}
		m.auditReq(r, "destination.keygen", identity.Recipient().String())
		respond(w, 200, map[string]string{"public_key": identity.Recipient().String(), "private_key": identity.String()})
	case r.Method == "POST" && (rest == "" || rest == "test"):
		m.saveDestination(w, r, "", rest == "test")
	case r.Method == "PUT" && idPattern.MatchString(id) && action == "":
		m.saveDestination(w, r, id, false)
	case r.Method == "DELETE" && idPattern.MatchString(id) && action == "":
		m.deleteDestination(w, r, id)
	case r.Method == "GET" && idPattern.MatchString(id) && action == "files":
		// Remote files are not tied to a connection, so only global viewers may list them.
		if !principalOf(r).can(anyValue, anyValue, "view") {
			fail(w, 403, "Cần quyền xem toàn bộ kết nối")
			return
		}
		d, err := m.destination(id)
		if err != nil {
			fail(w, 404, err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		files, err := m.listRemote(ctx, d)
		if err != nil {
			fail(w, 502, err.Error())
			return
		}
		respond(w, 200, files)
	case r.Method == "POST" && idPattern.MatchString(id) && action == "pull":
		m.startPull(w, r, id)
	default:
		fail(w, 404, "Endpoint not found")
	}
}

func (m *Manager) saveDestination(w http.ResponseWriter, r *http.Request, id string, test bool) {
	var d Destination
	if !decode(w, r, &d) {
		return
	}
	if test {
		id = d.ID
	}
	var old *Destination
	if id != "" {
		if !idPattern.MatchString(id) {
			fail(w, 404, "Đích cloud không tồn tại")
			return
		}
		stored, err := m.destination(id)
		if err != nil {
			fail(w, 404, err.Error())
			return
		}
		old = &stored
		d.CreatedAt = stored.CreatedAt
	}
	if d.GoogleOAuthID != "" {
		config, err := m.googleOAuthConfig(r, d.GoogleOAuthID)
		if err != nil || d.Config != "" {
			fail(w, 400, "Kết nối Google OAuth đã hết hạn hoặc không hợp lệ. Hãy kết nối lại.")
			return
		}
		d.Config = config
		// Reconnecting changes credentials, not the existing Drive root.
		// Preserve Shared Drive / Folder ID settings from manual rclone setup.
		if old != nil && old.Type == "drive" {
			opts, _, err := parseRcloneConfig(old.Config, false)
			if err != nil {
				fail(w, 400, "Cấu hình Google Drive hiện tại không hợp lệ")
				return
			}
			for _, option := range opts {
				if option.key == "root_folder_id" || option.key == "team_drive" {
					d.Config += option.key + " = " + option.value + "\n"
				}
			}
		}
	}
	if err := m.validateDestination(&d, old); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if test {
		d.ID = id
		if d.ID == "" {
			d.ID = token()
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		if err := m.testDestination(ctx, d); err != nil {
			fail(w, 400, "Không ghi được lên cloud: "+err.Error())
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	// Recheck after serializing saves so a completed ticket is single-use,
	// even when two requests validate it concurrently.
	if d.GoogleOAuthID != "" {
		if _, err := m.googleOAuthConfig(r, d.GoogleOAuthID); err != nil {
			fail(w, 400, "Kết nối Google OAuth đã hết hạn hoặc không hợp lệ. Hãy kết nối lại.")
			return
		}
	}
	if id != "" {
		var active int
		if err := m.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE destination_id=? AND status IN ('queued','running')`, id).Scan(&active); err != nil {
			fail(w, 500, "Cannot check active jobs")
			return
		}
		if active > 0 {
			fail(w, 409, "Đợi job upload/tải về hoàn tất trước khi sửa đích cloud")
			return
		}
	} else {
		id, d.CreatedAt = token(), now()
	}
	d.ID = id
	_, err := m.db.Exec(`INSERT INTO destinations(id,name,type,config,remote_path,encryption,recipients,passphrase,retention_days,created_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,type=excluded.type,config=excluded.config,remote_path=excluded.remote_path,encryption=excluded.encryption,recipients=excluded.recipients,passphrase=excluded.passphrase,retention_days=excluded.retention_days`,
		d.ID, d.Name, d.Type, m.sealPassword(d.Config), d.RemotePath, d.Encryption, d.Recipients, sealIfSet(m, d.Passphrase), d.RetentionDays, d.CreatedAt)
	if err != nil {
		fail(w, 500, "Cannot save destination")
		return
	}
	m.auditReq(r, "destination.saved", d.ID+" "+d.Name+" ("+d.Type+", "+d.Encryption+")")
	if d.GoogleOAuthID != "" {
		m.googleOAuthMu.Lock()
		delete(m.googleOAuth, d.GoogleOAuthID)
		m.googleOAuthMu.Unlock()
		d.GoogleOAuthID = ""
	}
	d.HasConfig, d.HasPassphrase = true, d.Passphrase != ""
	d.Config, d.Passphrase = "", ""
	respond(w, 200, d)
}

func sealIfSet(m *Manager, v string) string {
	if v == "" {
		return ""
	}
	return m.sealPassword(v)
}

func (m *Manager) deleteDestination(w http.ResponseWriter, r *http.Request, id string) {
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	m.schedMu.Lock()
	defer m.schedMu.Unlock()
	n, err := destinationInUse(m.db, id)
	if err != nil {
		fail(w, 500, "Cannot check destination usage")
		return
	}
	if n > 0 {
		fail(w, 409, "Gỡ đích cloud khỏi các lịch backup và đợi job hoàn tất trước khi xóa")
		return
	}
	res, err := m.db.Exec(`DELETE FROM destinations WHERE id=?`, id)
	if err != nil {
		fail(w, 500, "Cannot delete destination")
		return
	}
	if c, _ := res.RowsAffected(); c == 0 {
		fail(w, 404, "Đích cloud không tồn tại")
		return
	}
	m.db.Exec(`DELETE FROM offsite WHERE destination_id=?`, id)
	m.auditReq(r, "destination.deleted", id)
	respond(w, 200, map[string]bool{"ok": true})
}

func (m *Manager) startPull(w http.ResponseWriter, r *http.Request, destinationID string) {
	m.connectionMu.Lock()
	defer m.connectionMu.Unlock()
	var b struct {
		RemotePath   string `json:"remote_path"`
		Database     string `json:"database"`
		ConnectionID string `json:"connection_id"`
		Secret       string `json:"secret"`
	}
	if !decode(w, r, &b) {
		return
	}
	d, err := m.destination(destinationID)
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	rel, err := cleanRemotePath(b.RemotePath)
	if err != nil || rel == "" || !strings.HasSuffix(rel, encryptedSuffix) {
		fail(w, 400, "Chọn file .dump.age trên cloud")
		return
	}
	if !dbName.MatchString(b.Database) {
		fail(w, 400, "Tên database gốc không hợp lệ")
		return
	}
	if _, err = m.forConnection(b.ConnectionID); err != nil {
		fail(w, 404, err.Error())
		return
	}
	b.Secret = strings.TrimSpace(b.Secret)
	if d.Encryption == "recipient" || b.Secret != "" {
		if strings.HasPrefix(b.Secret, "AGE-SECRET-KEY-") {
			if _, err = age.ParseIdentities(strings.NewReader(b.Secret)); err != nil {
				fail(w, 400, "Private key age không hợp lệ")
				return
			}
		} else if d.Encryption == "recipient" {
			fail(w, 400, "Nhập private key (AGE-SECRET-KEY-…) tương ứng với public key của đích này")
			return
		}
	}
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	// The secret stays in memory only and is registered before the worker can see the job.
	j := Job{ID: token(), ConnectionID: b.ConnectionID, Kind: "offsite_pull", Database: b.Database, DestinationID: destinationID, RemotePath: rel}
	m.pullMu.Lock()
	m.pullSecrets[j.ID] = b.Secret
	m.pullMu.Unlock()
	j, err = m.enqueueJobLocked(j)
	if err != nil {
		m.pullMu.Lock()
		delete(m.pullSecrets, j.ID)
		m.pullMu.Unlock()
		fail(w, 409, err.Error())
		return
	}
	respond(w, 202, j)
}

func (m *Manager) startOffsiteUpload(w http.ResponseWriter, r *http.Request, b Backup) {
	var body struct {
		DestinationID string `json:"destination_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !idPattern.MatchString(body.DestinationID) {
		fail(w, 400, "Chọn đích cloud")
		return
	}
	if _, err := m.destination(body.DestinationID); err != nil {
		fail(w, 404, err.Error())
		return
	}
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	j, err := m.enqueueJobLocked(Job{ConnectionID: b.ConnectionID, Kind: "offsite", Database: b.Database, BackupID: b.ID, DestinationID: body.DestinationID})
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	respond(w, 202, j)
}

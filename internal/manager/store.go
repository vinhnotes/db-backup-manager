package manager

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"
)

type Backup struct {
	ConnectionID string `json:"connection_id"`
	ID           string `json:"id"`
	Database     string `json:"database"`
	Size         int64  `json:"size"`
	Checksum     string `json:"checksum"`
	CreatedAt    string `json:"created_at"`
	Source       string `json:"source"`
	Engine       string `json:"engine"`
}
type Job struct {
	ConnectionID  string `json:"connection_id"`
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Database      string `json:"database"`
	BackupID      string `json:"backup_id"`
	DestinationID string `json:"destination_id"`
	RemotePath    string `json:"remote_path"`
	Status        string `json:"status"`
	Error         string `json:"error"`
	CreatedAt     string `json:"created_at"`
	StartedAt     string `json:"started_at"`
	FinishedAt    string `json:"finished_at"`
	// Attempt counts automatic retries already made (0 = first try).
	Attempt int `json:"attempt"`
	// Verify asks for a restore test of the new backup on VerifyConnectionID.
	// Only kept in memory: jobs are not resumed after a restart.
	Verify             bool   `json:"-"`
	VerifyConnectionID string `json:"-"`
}
type Schedule struct {
	ConnectionID  string `json:"connection_id"`
	ID            int64  `json:"id"`
	Database      string `json:"database"`
	Cron          string `json:"cron"`
	RetentionDays int    `json:"retention_days"`
	Enabled       bool   `json:"enabled"`
	DestinationID string `json:"destination_id"`
	// Verify runs a restore test after every backup of this schedule, on
	// VerifyConnectionID ("" = the schedule's own connection).
	Verify             bool   `json:"verify"`
	VerifyConnectionID string `json:"verify_connection_id"`
	NextRun            string `json:"next_run,omitempty"`
	LastRunAt          string `json:"last_run_at"`
	UpdatedAt          string `json:"-"`
}

// BackupView is a backup with the result of its latest restore test.
type BackupView struct {
	Backup
	VerifyStatus string `json:"verify_status"` // "", "passed" or "failed"
	VerifyDetail string `json:"verify_detail"`
	VerifiedAt   string `json:"verified_at"`
}

func openStore(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS backups(id TEXT PRIMARY KEY, database_name TEXT NOT NULL, size INTEGER NOT NULL, checksum TEXT NOT NULL, created_at TEXT NOT NULL, source TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS jobs(id TEXT PRIMARY KEY, kind TEXT NOT NULL, database_name TEXT NOT NULL, backup_id TEXT NOT NULL, status TEXT NOT NULL, error TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, finished_at TEXT NOT NULL DEFAULT '');
 CREATE TABLE IF NOT EXISTS schedules(id INTEGER PRIMARY KEY AUTOINCREMENT, database_name TEXT NOT NULL, cron TEXT NOT NULL, retention_days INTEGER NOT NULL, enabled INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS connections(id TEXT PRIMARY KEY, name TEXT NOT NULL, host TEXT NOT NULL, port TEXT NOT NULL, username TEXT NOT NULL, password TEXT NOT NULL, database_name TEXT NOT NULL, sslmode TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS destinations(id TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL, config TEXT NOT NULL, remote_path TEXT NOT NULL, encryption TEXT NOT NULL, recipients TEXT NOT NULL, passphrase TEXT NOT NULL, retention_days INTEGER NOT NULL, created_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS offsite(backup_id TEXT NOT NULL, destination_id TEXT NOT NULL, connection_id TEXT NOT NULL, database_name TEXT NOT NULL, remote_path TEXT NOT NULL, size INTEGER NOT NULL, checksum TEXT NOT NULL, backup_created_at TEXT NOT NULL, uploaded_at TEXT NOT NULL, PRIMARY KEY(backup_id, destination_id));
 CREATE TABLE IF NOT EXISTS sessions(token TEXT PRIMARY KEY, role TEXT NOT NULL, expires_at INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS audit(id INTEGER PRIMARY KEY AUTOINCREMENT, action TEXT NOT NULL, detail TEXT NOT NULL, created_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS users(id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE COLLATE NOCASE, display_name TEXT NOT NULL, password_hash TEXT NOT NULL, role TEXT NOT NULL, disabled INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY, value TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS user_permissions(user_id TEXT NOT NULL, connection_id TEXT NOT NULL, database_name TEXT NOT NULL, level TEXT NOT NULL, PRIMARY KEY(user_id, connection_id, database_name));
 CREATE TABLE IF NOT EXISTS notifiers(id TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL, config TEXT NOT NULL, events TEXT NOT NULL, lang TEXT NOT NULL, enabled INTEGER NOT NULL, created_at TEXT NOT NULL);
 DELETE FROM sessions;`)
	if err != nil {
		db.Close()
		return nil, err
	}
	const text, integer = "TEXT NOT NULL DEFAULT ''", "INTEGER NOT NULL DEFAULT 0"
	for _, migration := range [][3]string{{"backups", "connection_id", text}, {"jobs", "connection_id", text}, {"schedules", "connection_id", text}, {"connections", "restore_username", text}, {"connections", "restore_password", text}, {"schedules", "destination_id", text}, {"jobs", "destination_id", text}, {"jobs", "remote_path", text}, {"sessions", "user_id", text}, {"audit", "actor", text}, {"backups", "engine", text}, {"connections", "engine", text}, {"connections", "server_dir", text}, {"connections", "local_dir", text},
		{"jobs", "attempt", integer}, {"jobs", "started_at", text}, {"schedules", "verify", integer}, {"schedules", "verify_connection_id", text}, {"schedules", "updated_at", text}, {"schedules", "last_run_at", text}, {"backups", "verify_status", text}, {"backups", "verify_detail", text}, {"backups", "verified_at", text}} {
		table, column, definition := migration[0], migration[1], migration[2]
		rows, e := db.Query("PRAGMA table_info(" + table + ")")
		if e != nil {
			db.Close()
			return nil, e
		}
		found := false
		for rows.Next() {
			var cid, notnull, pk int
			var name, kind string
			var def any
			if e = rows.Scan(&cid, &name, &kind, &notnull, &def, &pk); e != nil {
				rows.Close()
				db.Close()
				return nil, e
			}
			if name == column {
				found = true
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			db.Close()
			return nil, e
		}
		if !found {
			if _, e = db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition); e != nil {
				db.Close()
				return nil, e
			}
		}
	}
	_, err = db.Exec(`UPDATE jobs SET status='failed', error='Server restarted before completion; inspect target before retrying restore', finished_at=? WHERE status IN ('queued','running')`, now())
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func (m *Manager) backups() ([]Backup, error) {
	rows, err := m.db.Query(`SELECT connection_id,id,database_name,size,checksum,created_at,source,engine FROM backups ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Backup{}
	for rows.Next() {
		var b Backup
		if err = rows.Scan(&b.ConnectionID, &b.ID, &b.Database, &b.Size, &b.Checksum, &b.CreatedAt, &b.Source, &b.Engine); err != nil {
			return nil, err
		}
		b.Engine = engineName(b.Engine)
		out = append(out, b)
	}
	return out, rows.Err()
}

// backupViews lists backups with their latest restore test result.
func (m *Manager) backupViews() ([]BackupView, error) {
	rows, err := m.db.Query(`SELECT connection_id,id,database_name,size,checksum,created_at,source,engine,verify_status,verify_detail,verified_at FROM backups ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BackupView{}
	for rows.Next() {
		var b BackupView
		if err = rows.Scan(&b.ConnectionID, &b.ID, &b.Database, &b.Size, &b.Checksum, &b.CreatedAt, &b.Source, &b.Engine, &b.VerifyStatus, &b.VerifyDetail, &b.VerifiedAt); err != nil {
			return nil, err
		}
		b.Engine = engineName(b.Engine)
		out = append(out, b)
	}
	return out, rows.Err()
}
func (m *Manager) backup(id string) (Backup, error) {
	var b Backup
	err := m.db.QueryRow(`SELECT connection_id,id,database_name,size,checksum,created_at,source,engine FROM backups WHERE id=?`, id).Scan(&b.ConnectionID, &b.ID, &b.Database, &b.Size, &b.Checksum, &b.CreatedAt, &b.Source, &b.Engine)
	b.Engine = engineName(b.Engine)
	return b, err
}
func (m *Manager) jobs() ([]Job, error) {
	rows, err := m.db.Query(`SELECT connection_id,id,kind,database_name,backup_id,destination_id,remote_path,status,error,created_at,started_at,finished_at,attempt FROM jobs ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		var j Job
		if err = rows.Scan(&j.ConnectionID, &j.ID, &j.Kind, &j.Database, &j.BackupID, &j.DestinationID, &j.RemotePath, &j.Status, &j.Error, &j.CreatedAt, &j.StartedAt, &j.FinishedAt, &j.Attempt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
func (m *Manager) schedules() ([]Schedule, error) {
	rows, err := m.db.Query(`SELECT connection_id,id,database_name,cron,retention_days,enabled,destination_id,verify,verify_connection_id,last_run_at,updated_at FROM schedules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() {
		var s Schedule
		if err = rows.Scan(&s.ConnectionID, &s.ID, &s.Database, &s.Cron, &s.RetentionDays, &s.Enabled, &s.DestinationID, &s.Verify, &s.VerifyConnectionID, &s.LastRunAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func (m *Manager) audit(action, detail string) { m.auditBy("", action, detail) }
func (m *Manager) auditBy(actor, action, detail string) {
	if _, err := m.db.Exec(`INSERT INTO audit(action,detail,created_at,actor) VALUES(?,?,?,?)`, action, detail, now(), actor); err != nil {
		m.logger.Printf("audit write failed: %v", err)
	}
}

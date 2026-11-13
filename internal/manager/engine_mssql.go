package manager

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/microsoft/go-mssqldb"
)

// SQL Server has no remote dump tool: BACKUP DATABASE writes a .bak file on
// the database server. The connection names a directory shared between SQL
// Server (ServerDir, as SQL Server sees it) and this container (LocalDir, the
// same directory as mounted here). Backups are written there and moved into
// BACKUP_DIR; restores copy the file there and run RESTORE DATABASE.
type mssqlEngine struct{}

func (mssqlEngine) name() string  { return "mssql" }
func (mssqlEngine) label() string { return "SQL Server" }
func (mssqlEngine) ext() string   { return ".bak" }
func (mssqlEngine) defaults() (string, string, string) {
	return "1433", "master", "prefer"
}
func (mssqlEngine) sslModes() []string {
	return []string{"disable", "prefer", "require", "verify-full"}
}
func (mssqlEngine) reserved(database string) bool {
	switch strings.ToLower(database) {
	case "master", "model", "msdb", "tempdb":
		return true
	}
	return false
}

func (mssqlEngine) open(m *Manager, restore bool, database string) (*sql.DB, error) {
	user, password := m.credentials(restore)
	u := &url.URL{Scheme: "sqlserver", User: url.UserPassword(user, password), Host: net.JoinHostPort(m.cfg.PGHost, m.cfg.PGPort)}
	q := url.Values{}
	if database == "" {
		database = m.cfg.PGDatabase
	}
	q.Set("database", database)
	q.Set("dial timeout", "10")
	q.Set("app name", "db-backup-manager")
	switch m.cfg.PGSSLMode {
	case "disable":
		q.Set("encrypt", "disable")
	case "require":
		q.Set("encrypt", "true")
		q.Set("TrustServerCertificate", "true")
	case "verify-full":
		q.Set("encrypt", "true")
	default: // prefer: encrypt the login only, as SQL Server clients do by default
		q.Set("encrypt", "false")
		q.Set("TrustServerCertificate", "true")
	}
	u.RawQuery = q.Encode()
	return sql.Open("sqlserver", u.String())
}

func (e mssqlEngine) databases(ctx context.Context, m *Manager) ([]Database, string, error) {
	db, err := e.open(m, false, "")
	if err != nil {
		return nil, "", errors.New("Cấu hình SQL Server không hợp lệ")
	}
	defer db.Close()
	var version, edition string
	if err = db.QueryRowContext(ctx, `SELECT CAST(SERVERPROPERTY('ProductVersion') AS nvarchar(128)), CAST(SERVERPROPERTY('Edition') AS nvarchar(128))`).Scan(&version, &edition); err != nil {
		return nil, "", errors.New("Không kết nối được SQL Server; kiểm tra host, tài khoản, SSL và mạng từ máy chủ backup")
	}
	version += " " + edition
	// sys.master_files hides rows without VIEW ANY DEFINITION: sizes are then 0.
	rows, err := db.QueryContext(ctx, `SELECT d.name, CAST(COALESCE(SUM(CAST(f.size AS bigint)), 0) * 8192 AS bigint)
		FROM sys.databases d LEFT JOIN sys.master_files f ON f.database_id = d.database_id
		WHERE d.database_id > 4 AND d.state = 0 GROUP BY d.name ORDER BY d.name`)
	if err != nil {
		return nil, version, fmt.Errorf("cannot list databases; check permissions")
	}
	defer rows.Close()
	out := []Database{}
	for rows.Next() {
		var d Database
		if err = rows.Scan(&d.Name, &d.Size); err != nil {
			return nil, version, err
		}
		out = append(out, d)
	}
	return out, version, rows.Err()
}

func (e mssqlEngine) test(ctx context.Context, m *Manager) error {
	db, err := e.open(m, false, "")
	if err != nil {
		return errors.New("Cấu hình SQL Server không hợp lệ")
	}
	defer db.Close()
	if err = db.PingContext(ctx); err != nil {
		return errors.New("Không kết nối được SQL Server; kiểm tra host, tài khoản, SSL và mạng từ máy chủ backup")
	}
	if err = checkSharedDir(m.cfg.LocalDir); err != nil {
		return err
	}
	if m.cfg.RestoreUser == "" {
		return nil
	}
	r, err := e.open(m, true, "")
	if err != nil {
		return errors.New("Cấu hình restore không hợp lệ")
	}
	defer r.Close()
	var canCreate int
	if err = r.QueryRowContext(ctx, `SELECT HAS_PERMS_BY_NAME(NULL, NULL, 'CREATE ANY DATABASE')`).Scan(&canCreate); err != nil || canCreate != 1 {
		return errors.New("Tài khoản restore cần đăng nhập được SQL Server và có quyền CREATE ANY DATABASE (vai trò dbcreator)")
	}
	return nil
}

// checkSharedDir verifies this container can write to the admin-selected shared
// mount. The root is intentionally configurable; the temporary name is generated.
func checkSharedDir(dir string) error {
	if !validLocalDir(dir) || dir == "/" {
		return errors.New("Thư mục dùng chung phải là đường dẫn tuyệt đối hợp lệ")
	}
	f, err := os.CreateTemp(dir, ".dbbm-check-*")
	if err != nil {
		return fmt.Errorf("Không ghi được vào thư mục dùng chung %s trong container", dir)
	}
	return errors.Join(f.Close(), os.Remove(f.Name()))
}

// serverPath joins the shared directory and name as SQL Server sees them.
func serverPath(dir, name string) string {
	sep := "/"
	if strings.Contains(dir, `\`) {
		sep = `\`
	}
	return strings.TrimRight(dir, `/\`) + sep + name
}

func (e mssqlEngine) dump(ctx context.Context, m *Manager, database, out string) error {
	db, err := e.open(m, false, "")
	if err != nil {
		return errors.New("Cấu hình SQL Server không hợp lệ")
	}
	defer db.Close()
	name := "dbbm-" + token() + ".bak"
	local := filepath.Join(m.cfg.LocalDir, name)
	defer os.Remove(local)
	options := "COPY_ONLY, INIT, CHECKSUM"
	var edition int
	// Express (EngineEdition 4) does not support backup compression.
	if err = db.QueryRowContext(ctx, `SELECT CAST(SERVERPROPERTY('EngineEdition') AS int)`).Scan(&edition); err == nil && edition != 4 {
		options += ", COMPRESSION"
	}
	remote := serverPath(m.cfg.ServerDir, name)
	if _, err = db.ExecContext(ctx, `BACKUP DATABASE @p1 TO DISK = @p2 WITH `+options, database, remote); err != nil {
		return fmt.Errorf("BACKUP DATABASE failed: %s", m.redact(err.Error()))
	}
	// RESTORE VERIFYONLY needs CREATE DATABASE, which a db_backupoperator
	// login usually lacks; BACKUP WITH CHECKSUM has already checked pages.
	var canVerify int
	if err = db.QueryRowContext(ctx, `SELECT HAS_PERMS_BY_NAME(NULL, NULL, 'CREATE ANY DATABASE')`).Scan(&canVerify); err == nil && canVerify == 1 {
		if _, err = db.ExecContext(ctx, `RESTORE VERIFYONLY FROM DISK = @p1 WITH CHECKSUM`, remote); err != nil {
			return fmt.Errorf("RESTORE VERIFYONLY failed: %s", m.redact(err.Error()))
		}
	}
	if _, err = os.Stat(local); err != nil {
		return fmt.Errorf("SQL Server wrote %s but it is not visible at %s; check that both paths are the same shared directory", remote, local)
	}
	if err = moveFile(local, out); err != nil {
		return fmt.Errorf("cannot read %s; SQL Server creates backups readable by its own group only (mssql, GID 10001 on Linux): add that group to this container: %w", local, err)
	}
	return nil
}

func (mssqlEngine) verify(_ context.Context, _ *Manager, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	// Uncompressed backups start with an MTF "TAPE" block; compressed ones
	// (SQL Server 2022+) with "MSSQLBAK".
	head := make([]byte, 8)
	n, _ := io.ReadFull(f, head)
	if !bytes.HasPrefix(head[:n], []byte("TAPE")) && !bytes.HasPrefix(head[:n], []byte("MSSQLBAK")) {
		return errors.New("not a SQL Server backup (.bak) file")
	}
	return nil
}

type mssqlFile struct{ logical, physical, kind string }

func mssqlFileList(ctx context.Context, db *sql.DB, remote string) ([]mssqlFile, error) {
	rows, err := db.QueryContext(ctx, `RESTORE FILELISTONLY FROM DISK = @p1`, remote)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	index := map[string]int{}
	for i, c := range cols {
		index[c] = i
	}
	for _, c := range []string{"LogicalName", "PhysicalName", "Type"} {
		if _, ok := index[c]; !ok {
			return nil, fmt.Errorf("unexpected RESTORE FILELISTONLY output")
		}
	}
	out := []mssqlFile{}
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err = rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		str := func(c string) string {
			switch v := values[index[c]].(type) {
			case string:
				return v
			case []byte:
				return string(v)
			}
			return ""
		}
		out = append(out, mssqlFile{str("LogicalName"), str("PhysicalName"), str("Type")})
	}
	return out, rows.Err()
}

func (e mssqlEngine) restore(ctx context.Context, m *Manager, target, _, file string) error {
	db, err := e.open(m, true, "")
	if err != nil {
		return fmt.Errorf("invalid restore configuration")
	}
	defer db.Close()
	var existing sql.NullInt64
	if err = db.QueryRowContext(ctx, `SELECT DB_ID(@p1)`, target).Scan(&existing); err != nil {
		return fmt.Errorf("cannot check target database: %s", m.redact(err.Error()))
	}
	if existing.Valid {
		return fmt.Errorf("target database %s already exists", target)
	}
	name := "dbbm-restore-" + token() + ".bak"
	local := filepath.Join(m.cfg.LocalDir, name)
	// SQL Server runs as another user and must be able to read the copy.
	if err = copyFile(file, local, 0644); err != nil {
		os.Remove(local)
		return fmt.Errorf("cannot copy backup to the shared directory %s: %w", m.cfg.LocalDir, err)
	}
	defer os.Remove(local)
	remote := serverPath(m.cfg.ServerDir, name)
	files, err := mssqlFileList(ctx, db, remote)
	if err != nil {
		return fmt.Errorf("RESTORE FILELISTONLY failed: %s", m.redact(err.Error()))
	}
	var dataDir, logDir string
	if err = db.QueryRowContext(ctx, `SELECT CAST(SERVERPROPERTY('InstanceDefaultDataPath') AS nvarchar(4000)), CAST(SERVERPROPERTY('InstanceDefaultLogPath') AS nvarchar(4000))`).Scan(&dataDir, &logDir); err != nil {
		return fmt.Errorf("cannot read SQL Server default file paths: %s", m.redact(err.Error()))
	}
	// Give every file a new physical name so the source database is untouched.
	query := `RESTORE DATABASE @p1 FROM DISK = @p2 WITH CHECKSUM, RECOVERY`
	args := []any{target, remote}
	for i, f := range files {
		dir, suffix := dataDir, "_"+strconv.Itoa(i)
		if f.kind == "L" {
			dir = logDir
		}
		if ext := strings.ToLower(filepath.Ext(strings.ReplaceAll(f.physical, `\`, "/"))); f.kind == "D" || f.kind == "L" {
			if len(ext) == 4 {
				suffix += ext
			}
		}
		args = append(args, f.logical, serverPath(dir, target+suffix))
		query += fmt.Sprintf(", MOVE @p%d TO @p%d", len(args)-1, len(args)) // #nosec G202 -- only @pN placeholders are appended; values are bound parameters.
	}
	if _, err = db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("restore into %s failed; inspect and remove it manually before retry: %s", target, m.redact(err.Error()))
	}
	// The restored database keeps its original users, so the restore login
	// may not be able to open it: check its state from master instead.
	var state string
	if err = db.QueryRowContext(ctx, `SELECT state_desc FROM sys.databases WHERE name = @p1`, target).Scan(&state); err != nil || state != "ONLINE" {
		return fmt.Errorf("restore finished but database %s is not ONLINE (%s)", target, state)
	}
	return nil
}

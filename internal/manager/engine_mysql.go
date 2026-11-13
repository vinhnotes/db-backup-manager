package manager

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// MySQL and MariaDB are backed up with mariadb-dump (logical SQL, gzipped)
// and restored with the mariadb client. Variables so tests can stub them.
var (
	mysqlDumpBinary   = "mariadb-dump"
	mysqlClientBinary = "mariadb"
)

type mysqlEngine struct{}

func (mysqlEngine) name() string  { return "mysql" }
func (mysqlEngine) label() string { return "MySQL/MariaDB" }
func (mysqlEngine) ext() string   { return ".sql.gz" }
func (mysqlEngine) defaults() (string, string, string) {
	return "3306", "", "prefer"
}
func (mysqlEngine) sslModes() []string {
	return []string{"disable", "prefer", "require", "verify-full"}
}
func (mysqlEngine) reserved(database string) bool {
	switch strings.ToLower(database) {
	case "mysql", "information_schema", "performance_schema", "sys":
		return true
	}
	return false
}

func quoteMySQL(name string) string { return "`" + strings.ReplaceAll(name, "`", "``") + "`" }

func (mysqlEngine) open(m *Manager, restore bool, database string) (*sql.DB, error) {
	user, password := m.credentials(restore)
	cfg := mysql.NewConfig()
	cfg.User, cfg.Passwd = user, password
	cfg.Net, cfg.Addr, cfg.DBName = "tcp", net.JoinHostPort(m.cfg.PGHost, m.cfg.PGPort), database
	cfg.Timeout = 10 * time.Second
	switch m.cfg.PGSSLMode {
	case "disable":
		cfg.TLSConfig = "false"
	case "require":
		cfg.TLSConfig = "skip-verify"
	case "verify-full":
		cfg.TLSConfig = "true"
	default:
		cfg.TLSConfig = "preferred"
	}
	c, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(c), nil
}

func (e mysqlEngine) databases(ctx context.Context, m *Manager) ([]Database, string, error) {
	db, err := e.open(m, false, "")
	if err != nil {
		return nil, "", errors.New("Cấu hình MySQL/MariaDB không hợp lệ")
	}
	defer db.Close()
	var version string
	if err = db.QueryRowContext(ctx, `SELECT VERSION()`).Scan(&version); err != nil {
		return nil, "", errors.New("Không kết nối được MySQL/MariaDB; kiểm tra host, tài khoản, SSL và mạng từ máy chủ backup")
	}
	rows, err := db.QueryContext(ctx, `SELECT s.SCHEMA_NAME, CAST(COALESCE(SUM(t.DATA_LENGTH + t.INDEX_LENGTH), 0) AS SIGNED)
		FROM information_schema.SCHEMATA s LEFT JOIN information_schema.TABLES t ON t.TABLE_SCHEMA = s.SCHEMA_NAME
		WHERE s.SCHEMA_NAME NOT IN ('mysql','information_schema','performance_schema','sys')
		GROUP BY s.SCHEMA_NAME ORDER BY s.SCHEMA_NAME`)
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

func (e mysqlEngine) test(ctx context.Context, m *Manager) error {
	db, err := e.open(m, false, "")
	if err != nil {
		return errors.New("Cấu hình MySQL/MariaDB không hợp lệ")
	}
	defer db.Close()
	if err = db.PingContext(ctx); err != nil {
		return errors.New("Không kết nối được MySQL/MariaDB; kiểm tra host, tài khoản, SSL và mạng từ máy chủ backup")
	}
	if m.cfg.RestoreUser == "" {
		return nil
	}
	restoreDB, err := e.open(m, true, "")
	if err != nil {
		return errors.New("Cấu hình restore không hợp lệ")
	}
	defer restoreDB.Close()
	if err = restoreDB.PingContext(ctx); err != nil {
		return errors.New("Tài khoản restore không đăng nhập được MySQL/MariaDB")
	}
	return nil
}

// optionValue quotes a value for a MySQL/MariaDB option file.
func optionValue(v string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	return `"` + r.Replace(v) + `"`
}

// client prepares the command-line options shared by mariadb-dump and
// mariadb: credentials go through a private option file, never argv.
func (e mysqlEngine) client(ctx context.Context, m *Manager, restore bool) ([]string, func(), error) {
	mode := m.cfg.PGSSLMode
	if mode == "prefer" {
		// The CLI has no "use TLS if available" mode: ask the server first.
		db, err := e.open(m, restore, "")
		if err != nil {
			return nil, nil, err
		}
		var name, cipher string
		err = db.QueryRowContext(ctx, `SHOW SESSION STATUS LIKE 'Ssl_cipher'`).Scan(&name, &cipher)
		db.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot connect to MySQL/MariaDB: %s", m.redact(err.Error()))
		}
		mode = "disable"
		if cipher != "" {
			mode = "require"
		}
	}
	user, password := m.credentials(restore)
	content := "[client]\nhost=" + optionValue(m.cfg.PGHost) + "\nport=" + m.cfg.PGPort + "\nuser=" + optionValue(user) + "\nprotocol=TCP\n"
	if password != "" {
		content += "password=" + optionValue(password) + "\n"
	}
	path, cleanup, err := secretFile("dbbm-*.cnf", content)
	if err != nil {
		return nil, nil, err
	}
	// --defaults-file must come first; it also ignores /etc/my.cnf.
	args := []string{"--defaults-file=" + path}
	switch mode {
	case "disable":
		args = append(args, "--skip-ssl")
	case "require":
		args = append(args, "--ssl", "--skip-ssl-verify-server-cert")
	case "verify-full":
		args = append(args, "--ssl", "--ssl-verify-server-cert")
	}
	return args, cleanup, nil
}

func (e mysqlEngine) dump(ctx context.Context, m *Manager, database, out string) error {
	args, cleanup, err := e.client(ctx, m, false)
	if err != nil {
		return err
	}
	defer cleanup()
	// No --databases: the dump has no CREATE DATABASE/USE, so it can be
	// restored under a new name.
	args = append(args, "--single-transaction", "--quick", "--routines", "--triggers", "--events", "--hex-blob",
		"--no-tablespaces", "--default-character-set=utf8mb4", "--max-allowed-packet=1G", database)
	return m.dumpCompressed(ctx, baseEnv(), out, mysqlDumpBinary, args...)
}

var mysqlDumpHeader = regexp.MustCompile(`(?i)-- (MySQL|MariaDB) dump`)

func (mysqlEngine) verify(ctx context.Context, _ *Manager, file string) error {
	head, tail, err := readGzip(ctx, file, 4096)
	if err != nil {
		return fmt.Errorf("not a valid gzip-compressed SQL dump: %w", err)
	}
	if !mysqlDumpHeader.Match(head) || !bytes.Contains(tail, []byte("-- Dump completed")) {
		return errors.New("not a complete mysqldump/mariadb-dump file")
	}
	return nil
}

// prepareUpload accepts a plain .sql dump and compresses it.
func (mysqlEngine) prepareUpload(path string) error {
	gz, err := isGzip(path)
	if err != nil || gz {
		return err
	}
	return gzipInPlace(path)
}

// definerRe matches DEFINER clauses in views, routines, triggers and events.
// They are removed on restore so objects belong to the restore account, like
// pg_restore --no-owner, and no SUPER/SET_USER_ID privilege is needed.
var definerRe = regexp.MustCompile("DEFINER=`(?:[^`]|``)*`@`(?:[^`]|``)*`")

type definerFilter struct {
	r   *bufio.Reader
	buf []byte
	err error
}

func (f *definerFilter) Read(p []byte) (int, error) {
	for len(f.buf) == 0 {
		if f.err != nil {
			return 0, f.err
		}
		line, err := f.r.ReadBytes('\n')
		f.err = err
		if bytes.Contains(line, []byte("DEFINER=")) && !bytes.HasPrefix(line, []byte("INSERT INTO")) {
			line = definerRe.ReplaceAll(line, nil)
		}
		f.buf = line
	}
	n := copy(p, f.buf)
	f.buf = f.buf[n:]
	return n, nil
}

func (e mysqlEngine) restore(ctx context.Context, m *Manager, target, _, file string) error {
	db, err := e.open(m, true, "")
	if err != nil {
		return fmt.Errorf("invalid restore configuration")
	}
	defer db.Close()
	// CREATE DATABASE fails if the target exists: never touch an existing database.
	if _, err = db.ExecContext(ctx, `CREATE DATABASE `+quoteMySQL(target)); err != nil {
		return fmt.Errorf("cannot create target database (it may already exist or restore user lacks CREATE): %s", m.redact(err.Error()))
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	z, err := gzip.NewReader(bufio.NewReaderSize(f, 1<<20))
	if err != nil {
		return err
	}
	defer z.Close()
	args, cleanup, err := e.client(ctx, m, true)
	if err != nil {
		return err
	}
	defer cleanup()
	args = append(args, "--database="+target, "--binary-mode", "--default-character-set=utf8mb4", "--max-allowed-packet=1G")
	input := &definerFilter{r: bufio.NewReaderSize(z, 1<<20)}
	if err = m.run(ctx, baseEnv(), io.Reader(input), nil, mysqlClientBinary, args...); err != nil {
		return fmt.Errorf("target %s was created but restore failed; inspect and remove it manually before retry: %w", target, err)
	}
	check, err := e.open(m, true, target)
	if err != nil {
		return err
	}
	defer check.Close()
	if err = check.PingContext(ctx); err != nil {
		return fmt.Errorf("restore finished but target connection verification failed")
	}
	return nil
}

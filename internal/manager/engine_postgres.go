package manager

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type postgresEngine struct{}

func (postgresEngine) name() string  { return enginePostgres }
func (postgresEngine) label() string { return "PostgreSQL" }
func (postgresEngine) ext() string   { return ".dump" }
func (postgresEngine) defaults() (string, string, string) {
	return "5432", "postgres", "prefer"
}
func (postgresEngine) sslModes() []string {
	return []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
}
func (postgresEngine) reserved(database string) bool {
	return database == "postgres" || strings.HasPrefix(database, "template")
}

func (postgresEngine) databases(ctx context.Context, m *Manager) ([]Database, string, error) {
	db, err := m.connect(false)
	if err != nil {
		return nil, "", fmt.Errorf("invalid PostgreSQL configuration")
	}
	defer db.Close()
	var version string
	if err = db.QueryRowContext(ctx, `SHOW server_version`).Scan(&version); err != nil {
		return nil, "", fmt.Errorf("cannot connect to PostgreSQL; check host, credentials and network")
	}
	rows, err := db.QueryContext(ctx, `SELECT datname, pg_database_size(oid) FROM pg_database WHERE NOT datistemplate AND datallowconn ORDER BY datname`)
	if err != nil {
		return nil, version, fmt.Errorf("cannot list database sizes; check database permissions")
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

func (postgresEngine) test(ctx context.Context, m *Manager) error {
	db, err := m.connect(false)
	if err != nil {
		return errors.New("Cấu hình PostgreSQL không hợp lệ")
	}
	defer db.Close()
	// Ping verifies the configured maintenance database without needing size permissions.
	if err = db.PingContext(ctx); err != nil {
		return errors.New("Không kết nối được PostgreSQL; kiểm tra host, tài khoản, SSL và mạng từ máy chủ backup")
	}
	if m.cfg.RestoreUser == "" {
		return nil
	}
	restoreDB, err := m.connect(true)
	if err != nil {
		return errors.New("Cấu hình restore không hợp lệ")
	}
	defer restoreDB.Close()
	var canCreate bool
	err = restoreDB.QueryRowContext(ctx, "SELECT rolcreatedb OR rolsuper FROM pg_roles WHERE rolname = current_user").Scan(&canCreate)
	if err != nil || !canCreate {
		return errors.New("Tài khoản restore cần kết nối được database maintenance và có quyền CREATEDB")
	}
	return nil
}

func (postgresEngine) dump(ctx context.Context, m *Manager, database, out string) error {
	return m.command(ctx, false, "pg_dump", "--no-password", "--format=custom", "--file="+out, "--dbname=dbname="+pq.QuoteLiteral(database))
}

func (postgresEngine) verify(ctx context.Context, m *Manager, file string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := m.command(ctx, false, "pg_restore", "--list", file); err != nil {
		return fmt.Errorf("not a valid PostgreSQL custom archive: %w", err)
	}
	return nil
}

func (postgresEngine) restore(ctx context.Context, m *Manager, target, _, file string) error {
	db, err := m.connect(true)
	if err != nil {
		return fmt.Errorf("invalid restore configuration")
	}
	defer db.Close()
	// CREATE DATABASE fails if the target exists: never clean/drop an existing database.
	if _, err = db.ExecContext(ctx, `CREATE DATABASE `+pq.QuoteIdentifier(target)+` TEMPLATE template0`); err != nil {
		return fmt.Errorf("cannot create target database (it may already exist or restore user lacks CREATEDB)")
	}
	if err = m.command(ctx, true, "pg_restore", "--no-password", "--exit-on-error", "--single-transaction", "--no-owner", "--no-privileges", "--dbname=dbname="+pq.QuoteLiteral(target), file); err != nil {
		return fmt.Errorf("target %s was created but restore failed; inspect and remove it manually before retry: %w", target, err)
	}
	check, err := openPostgres(m.connection(target, true))
	if err != nil {
		return err
	}
	defer check.Close()
	if err = check.PingContext(ctx); err != nil {
		return fmt.Errorf("restore finished but target connection verification failed")
	}
	return nil
}

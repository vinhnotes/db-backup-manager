package manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lib/pq"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// A restore test loads a backup into a new temporary database, checks that it
// opens and counts its tables, then drops it. The temporary database always
// has a fresh random name and is only dropped if this test created it.

const verifyPrefix = "dbbm_verify_"

// verifier is implemented by every engine to support restore tests. All
// methods use the restore login of the connection.
type verifier interface {
	exists(ctx context.Context, m *Manager, database string) (bool, error)
	// inspect returns the number of tables (collections for MongoDB) in
	// database, or -1 when the restore login cannot read it.
	inspect(ctx context.Context, m *Manager, database string) (int, error)
	drop(ctx context.Context, m *Manager, database string) error
}

// chainVerify queues a restore test after a successful scheduled backup.
func (m *Manager) chainVerify(parent Job, backupID string) {
	m.queueMu.Lock()
	_, err := m.enqueueJobLocked(Job{ConnectionID: parent.VerifyConnectionID, Kind: "verify", Database: parent.Database, BackupID: backupID})
	m.queueMu.Unlock()
	if err != nil {
		j := Job{ConnectionID: parent.VerifyConnectionID, ID: token(), Kind: "verify", Database: parent.Database, BackupID: backupID, Status: "failed", Error: "Không thể xếp hàng kiểm thử restore: " + err.Error(), CreatedAt: now()}
		j.FinishedAt = j.CreatedAt
		if _, e := m.db.Exec(`INSERT INTO jobs(connection_id,id,kind,database_name,backup_id,status,error,created_at,finished_at) VALUES(?,?,'verify',?,?,'failed',?,?,?)`, j.ConnectionID, j.ID, j.Database, backupID, j.Error, j.CreatedAt, j.FinishedAt); e != nil {
			m.logger.Print(e)
		}
		m.notifyJob(j)
	}
}

// verifyRestore runs a restore test of j.BackupID on this connection and
// stores the result on the backup.
func (m *Manager) verifyRestore(ctx context.Context, j Job) error {
	detail, passed, err := m.runVerify(ctx, j)
	status := "passed"
	if !passed {
		status, detail = "failed", err.Error()
	}
	if _, e := m.db.Exec(`UPDATE backups SET verify_status=?,verify_detail=?,verified_at=? WHERE id=?`, status, truncate(detail, 1000), now(), j.BackupID); e != nil {
		m.logger.Printf("restore test result persistence failed: %v", e)
	}
	return err
}

// runVerify returns passed=true when the backup restored and opened, even if
// dropping the temporary database then failed (err is set in that case).
func (m *Manager) runVerify(ctx context.Context, j Job) (detail string, passed bool, err error) {
	if m.cfg.RestoreUser == "" {
		return "", false, errors.New("Server kiểm thử chưa cấu hình tài khoản restore")
	}
	b, err := m.backup(j.BackupID)
	if err != nil {
		return "", false, errors.New("Không tìm thấy bản backup cần kiểm thử")
	}
	e := m.engine()
	if engineName(b.Engine) != e.name() {
		return "", false, fmt.Errorf("Không thể khôi phục bản backup %s vào máy chủ %s", engineLabel(b.Engine), e.label())
	}
	v, ok := e.(verifier)
	if !ok {
		return "", false, fmt.Errorf("%s chưa hỗ trợ kiểm thử restore", e.label())
	}
	sum, _, err := checksum(m.path(b.ID))
	if err != nil {
		return "", false, err
	}
	if sum != b.Checksum {
		return "", false, errors.New("backup checksum mismatch")
	}
	if err = e.verify(ctx, m, m.path(b.ID)); err != nil {
		return "", false, err
	}
	temp := verifyPrefix + token()[:12]
	exists, err := v.exists(ctx, m, temp)
	if err != nil {
		return "", false, fmt.Errorf("cannot check temporary database: %s", m.redact(err.Error()))
	}
	if exists {
		return "", false, fmt.Errorf("temporary database %s already exists", temp)
	}
	start := time.Now()
	restoreErr := e.restore(ctx, m, temp, b.Database, m.path(b.ID))
	tables := 0
	if restoreErr == nil {
		tables, restoreErr = v.inspect(ctx, m, temp)
	}
	elapsed := time.Since(start).Round(time.Second)
	// Drop even when the job timed out or the manager is stopping.
	dropCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dropErr := v.drop(dropCtx, m, temp)
	if dropErr != nil {
		dropErr = fmt.Errorf("Không xóa được database tạm %s, hãy xóa thủ công: %s", temp, m.redact(dropErr.Error()))
	}
	if restoreErr != nil {
		err = fmt.Errorf("Restore thử thất bại: %s", m.redact(restoreErr.Error()))
		if dropErr != nil {
			err = fmt.Errorf("%w; %v", err, dropErr)
		}
		return "", false, err
	}
	if tables < 0 {
		detail = fmt.Sprintf("Restore thử thành công trong %s", elapsed)
	} else {
		detail = fmt.Sprintf("Restore thử thành công trong %s · %d bảng/collection", elapsed, tables)
	}
	return detail, true, dropErr
}

// startVerify queues a restore test of b, by default on its own connection.
func (m *Manager) startVerify(w http.ResponseWriter, r *http.Request, b Backup) {
	m.connectionMu.Lock()
	defer m.connectionMu.Unlock()
	var body struct {
		ConnectionID *string `json:"connection_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	connectionID := b.ConnectionID
	if body.ConnectionID != nil {
		connectionID = *body.ConnectionID
	}
	if err := m.checkVerifyTarget(principalOf(r), connectionID, b.Engine); err != nil {
		fail(w, 400, err.Error())
		return
	}
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	j, err := m.enqueueJobLocked(Job{ConnectionID: connectionID, Kind: "verify", Database: b.Database, BackupID: b.ID})
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	m.auditReq(r, "backup.verify_queued", b.ID)
	respond(w, 202, j)
}

// checkVerifyTarget checks that restore tests of engine backups can run on
// connectionID. Tests create and drop databases there, so they need operate
// rights on the whole connection, like restores.
func (m *Manager) checkVerifyTarget(p *Principal, connectionID, engine string) error {
	target, err := m.forConnection(connectionID)
	if err != nil {
		return err
	}
	if !p.can(connectionID, "*", "operate") {
		return errors.New("Kiểm thử restore cần quyền vận hành trên toàn bộ Server kiểm thử")
	}
	if target.engine().name() != engineName(engine) {
		return fmt.Errorf("Không thể khôi phục bản backup %s vào máy chủ %s", engineLabel(engine), target.engine().label())
	}
	if target.cfg.RestoreUser == "" || target.cfg.PGHost == "" {
		return errors.New("Server kiểm thử chưa cấu hình tài khoản restore")
	}
	return nil
}

// ---- engines ----------------------------------------------------------------

func (postgresEngine) exists(ctx context.Context, m *Manager, database string) (bool, error) {
	db, err := m.connect(true)
	if err != nil {
		return false, err
	}
	defer db.Close()
	var found bool
	err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, database).Scan(&found)
	return found, err
}

func (postgresEngine) inspect(ctx context.Context, m *Manager, database string) (int, error) {
	db, err := openPostgres(m.connection(database, true))
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var n int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema')`).Scan(&n)
	return n, err
}

func (postgresEngine) drop(ctx context.Context, m *Manager, database string) error {
	db, err := m.connect(true)
	if err != nil {
		return err
	}
	defer db.Close()
	// WITH (FORCE) needs PostgreSQL 13+; older servers get a plain DROP.
	if _, err = db.ExecContext(ctx, `DROP DATABASE IF EXISTS `+pq.QuoteIdentifier(database)+` WITH (FORCE)`); err == nil {
		return nil
	}
	_, err = db.ExecContext(ctx, `DROP DATABASE IF EXISTS `+pq.QuoteIdentifier(database))
	return err
}

func (e mysqlEngine) exists(ctx context.Context, m *Manager, database string) (bool, error) {
	db, err := e.open(m, true, "")
	if err != nil {
		return false, err
	}
	defer db.Close()
	var n int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?`, database).Scan(&n)
	return n > 0, err
}

func (e mysqlEngine) inspect(ctx context.Context, m *Manager, database string) (int, error) {
	db, err := e.open(m, true, "")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var n int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=?`, database).Scan(&n)
	return n, err
}

func (e mysqlEngine) drop(ctx context.Context, m *Manager, database string) error {
	db, err := e.open(m, true, "")
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, `DROP DATABASE IF EXISTS `+quoteMySQL(database))
	return err
}

func (e mongoEngine) exists(ctx context.Context, m *Manager, database string) (bool, error) {
	c, err := e.client(m, true)
	if err != nil {
		return false, err
	}
	defer c.Disconnect(context.Background())
	names, err := c.ListDatabaseNames(ctx, bson.D{{Key: "name", Value: database}})
	return len(names) > 0, err
}

func (e mongoEngine) inspect(ctx context.Context, m *Manager, database string) (int, error) {
	c, err := e.client(m, true)
	if err != nil {
		return 0, err
	}
	defer c.Disconnect(context.Background())
	names, err := c.Database(database).ListCollectionNames(ctx, bson.D{})
	return len(names), err
}

func (e mongoEngine) drop(ctx context.Context, m *Manager, database string) error {
	c, err := e.client(m, true)
	if err != nil {
		return err
	}
	defer c.Disconnect(context.Background())
	db := c.Database(database)
	if err = db.Drop(ctx); err == nil {
		return nil
	}
	// The built-in "restore" role may lack dropDatabase; a database without
	// collections disappears, so drop them one by one instead.
	names, listErr := db.ListCollectionNames(ctx, bson.D{})
	if listErr != nil {
		return err
	}
	for _, name := range names {
		if e := db.Collection(name).Drop(ctx); e != nil {
			return e
		}
	}
	return nil
}

func quoteMSSQL(name string) string { return "[" + strings.ReplaceAll(name, "]", "]]") + "]" }

func (e mssqlEngine) exists(ctx context.Context, m *Manager, database string) (bool, error) {
	db, err := e.open(m, true, "")
	if err != nil {
		return false, err
	}
	defer db.Close()
	var id sql.NullInt64
	err = db.QueryRowContext(ctx, `SELECT DB_ID(@p1)`, database).Scan(&id)
	return id.Valid, err
}

func (e mssqlEngine) inspect(ctx context.Context, m *Manager, database string) (int, error) {
	db, err := e.open(m, true, "")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var n int
	// The restore login owns the new database, but it keeps the users of the
	// source: if it still cannot read it, the ONLINE check of restore is the test.
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteMSSQL(database)+`.sys.tables`).Scan(&n); err != nil {
		return -1, nil
	}
	return n, nil
}

func (e mssqlEngine) drop(ctx context.Context, m *Manager, database string) error {
	db, err := e.open(m, true, "")
	if err != nil {
		return err
	}
	defer db.Close()
	var id sql.NullInt64
	if err = db.QueryRowContext(ctx, `SELECT DB_ID(@p1)`, database).Scan(&id); err != nil || !id.Valid {
		return err
	}
	// Best effort: close other sessions so DROP does not wait for them.
	db.ExecContext(ctx, `ALTER DATABASE `+quoteMSSQL(database)+` SET SINGLE_USER WITH ROLLBACK IMMEDIATE`)
	_, err = db.ExecContext(ctx, `DROP DATABASE `+quoteMSSQL(database))
	return err
}

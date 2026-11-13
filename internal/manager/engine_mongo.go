package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoDB is backed up with mongodump --archive --gzip and restored with
// mongorestore. Variables so tests can stub them.
var (
	mongoDumpBinary    = "mongodump"
	mongoRestoreBinary = "mongorestore"
)

// mongoArchiveMagic starts every mongodump archive (0x8199e26d, little endian).
var mongoArchiveMagic = []byte{0x6d, 0xe2, 0x99, 0x81}

type mongoEngine struct{}

func (mongoEngine) name() string  { return "mongodb" }
func (mongoEngine) label() string { return "MongoDB" }
func (mongoEngine) ext() string   { return ".archive.gz" }

// The maintenance database is the authentication database (authSource).
func (mongoEngine) defaults() (string, string, string) {
	return "27017", "admin", "disable"
}
func (mongoEngine) sslModes() []string {
	return []string{"disable", "require", "verify-full"}
}
func (mongoEngine) reserved(database string) bool {
	switch database {
	case "admin", "local", "config":
		return true
	}
	return false
}

func (mongoEngine) uri(m *Manager, restore bool) string {
	user, password := m.credentials(restore)
	u := &url.URL{Scheme: "mongodb", Host: net.JoinHostPort(m.cfg.PGHost, m.cfg.PGPort), Path: "/"}
	q := url.Values{}
	if user != "" {
		u.User = url.UserPassword(user, password)
		q.Set("authSource", m.cfg.PGDatabase)
	}
	switch m.cfg.PGSSLMode {
	case "require":
		q.Set("tls", "true")
		q.Set("tlsInsecure", "true")
	case "verify-full":
		q.Set("tls", "true")
	}
	q.Set("connectTimeoutMS", "10000")
	q.Set("serverSelectionTimeoutMS", "15000")
	q.Set("appName", "db-backup-manager")
	u.RawQuery = q.Encode()
	return u.String()
}

func (e mongoEngine) client(m *Manager, restore bool) (*mongo.Client, error) {
	return mongo.Connect(options.Client().ApplyURI(e.uri(m, restore)))
}

func (e mongoEngine) databases(ctx context.Context, m *Manager) ([]Database, string, error) {
	c, err := e.client(m, false)
	if err != nil {
		return nil, "", errors.New("Cấu hình MongoDB không hợp lệ")
	}
	defer c.Disconnect(context.Background())
	var info struct {
		Version string `bson:"version"`
	}
	if err = c.Database("admin").RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&info); err != nil {
		return nil, "", errors.New("Không kết nối được MongoDB; kiểm tra host, tài khoản, SSL và mạng từ máy chủ backup")
	}
	res, err := c.ListDatabases(ctx, bson.D{})
	if err != nil {
		return nil, info.Version, fmt.Errorf("cannot list databases; check permissions")
	}
	out := []Database{}
	for _, d := range res.Databases {
		if !e.reserved(d.Name) {
			out = append(out, Database{Name: d.Name, Size: d.SizeOnDisk})
		}
	}
	return out, info.Version, nil
}

func (e mongoEngine) test(ctx context.Context, m *Manager) error {
	c, err := e.client(m, false)
	if err != nil {
		return errors.New("Cấu hình MongoDB không hợp lệ")
	}
	defer c.Disconnect(context.Background())
	if err = c.Ping(ctx, nil); err != nil {
		return errors.New("Không kết nối được MongoDB; kiểm tra host, tài khoản, SSL và mạng từ máy chủ backup")
	}
	if m.cfg.RestoreUser == "" {
		return nil
	}
	r, err := e.client(m, true)
	if err != nil {
		return errors.New("Cấu hình restore không hợp lệ")
	}
	defer r.Disconnect(context.Background())
	if err = r.Ping(ctx, nil); err != nil {
		return errors.New("Tài khoản restore không đăng nhập được MongoDB")
	}
	return nil
}

// toolConfig writes a mongodump/mongorestore --config file holding the URI,
// so the password never appears in the process list.
func (e mongoEngine) toolConfig(m *Manager, restore bool) (string, func(), error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e.uri(m, restore)); err != nil {
		return "", nil, err
	}
	// A JSON string is a valid YAML double-quoted scalar.
	return secretFile("dbbm-*.yaml", "uri: "+strings.TrimSpace(b.String())+"\n")
}

func (e mongoEngine) dump(ctx context.Context, m *Manager, database, out string) error {
	cfg, cleanup, err := e.toolConfig(m, false)
	if err != nil {
		return err
	}
	defer cleanup()
	return m.run(ctx, baseEnv(), nil, nil, mongoDumpBinary, "--config="+cfg, "--db="+database, "--archive="+out, "--gzip", "--quiet")
}

func (mongoEngine) verify(ctx context.Context, _ *Manager, file string) error {
	head, _, err := readGzip(ctx, file, 4)
	if err != nil {
		return fmt.Errorf("not a gzip-compressed mongodump archive: %w", err)
	}
	if !bytes.Equal(head, mongoArchiveMagic) {
		return errors.New("not a mongodump archive")
	}
	return nil
}

func (e mongoEngine) restore(ctx context.Context, m *Manager, target, source, file string) error {
	c, err := e.client(m, true)
	if err != nil {
		return fmt.Errorf("invalid restore configuration")
	}
	defer c.Disconnect(context.Background())
	// MongoDB creates databases implicitly: refuse a target that already exists.
	names, err := c.ListDatabaseNames(ctx, bson.D{{Key: "name", Value: target}})
	if err != nil {
		return fmt.Errorf("cannot check target database: %s", m.redact(err.Error()))
	}
	if len(names) > 0 {
		return fmt.Errorf("target database %s already exists", target)
	}
	cfg, cleanup, err := e.toolConfig(m, true)
	if err != nil {
		return err
	}
	defer cleanup()
	if err = m.run(ctx, baseEnv(), nil, nil, mongoRestoreBinary, "--config="+cfg, "--archive="+file, "--gzip", "--quiet", "--stopOnError",
		"--nsInclude="+source+".*", "--nsFrom="+source+".*", "--nsTo="+target+".*"); err != nil {
		return fmt.Errorf("restore into %s failed; inspect and remove it manually before retry: %w", target, err)
	}
	return nil
}

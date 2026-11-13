package manager

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// dbEngine is one supported database type. Each engine reads the
// per-connection settings from m.cfg (PGHost, PGPort, PGUser… are generic
// despite their names: they hold host, port and credentials for any engine).
type dbEngine interface {
	name() string
	label() string
	// defaults returns the default port, maintenance database and SSL mode.
	defaults() (port, database, sslmode string)
	sslModes() []string
	// reserved reports system databases that must never be a restore target.
	reserved(database string) bool
	// ext is the file extension used when downloading a backup.
	ext() string
	databases(ctx context.Context, m *Manager) ([]Database, string, error)
	// test checks the backup login and, when configured, the restore login.
	test(ctx context.Context, m *Manager) error
	// dump writes a backup of database to out.
	dump(ctx context.Context, m *Manager, database, out string) error
	// verify checks that file is a complete archive of this engine without
	// contacting the server, so uploads and cloud imports can use it too.
	verify(ctx context.Context, m *Manager, file string) error
	// restore loads file (a backup of source) into the new database target.
	// It must refuse to touch a database that already exists.
	restore(ctx context.Context, m *Manager, target, source, file string) error
}

// uploadPreparer is implemented by engines that accept more than one upload
// format and convert it to their stored format before verification.
type uploadPreparer interface {
	prepareUpload(path string) error
}

const enginePostgres = "postgres"

var engines = map[string]dbEngine{
	enginePostgres: postgresEngine{},
	"mysql":        mysqlEngine{},
	"mongodb":      mongoEngine{},
	"mssql":        mssqlEngine{},
	engineFiles:    filesEngine{},
}

// engineName normalises a stored engine name: rows created before
// multi-engine support have an empty value and are PostgreSQL.
func engineName(name string) string {
	if name == "" {
		return enginePostgres
	}
	return name
}

func engineByName(name string) (dbEngine, bool) {
	e, ok := engines[engineName(name)]
	return e, ok
}

func engineLabel(name string) string {
	if e, ok := engineByName(name); ok {
		return e.label()
	}
	return name
}

func (m *Manager) engine() dbEngine {
	if e, ok := engineByName(m.cfg.Engine); ok {
		return e
	}
	return postgresEngine{}
}

// credentials returns the backup or restore login of the current connection.
func (m *Manager) credentials(restore bool) (user, password string) {
	if restore {
		return m.cfg.RestoreUser, m.cfg.RestorePassword
	}
	return m.cfg.PGUser, m.cfg.PGPassword
}

// baseEnv is the process environment without database credentials or
// application secrets, for child processes.
func baseEnv() []string {
	out := []string{}
	for _, v := range os.Environ() {
		k, _, _ := strings.Cut(v, "=")
		if !strings.HasPrefix(k, "PG") && !strings.HasPrefix(k, "MYSQL") && k != "ADMIN_PASSWORD" && k != "VIEWER_PASSWORD" && k != "METRICS_TOKEN" && !strings.HasPrefix(k, "RESTORE_") {
			out = append(out, v)
		}
	}
	return out
}

// run executes a client tool. Its stderr is kept (truncated, with secrets
// redacted) for the error message.
func (m *Manager) run(ctx context.Context, env []string, stdin io.Reader, stdout io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- fixed client tool name and argument list; no shell is involved.
	cmd.Env = env
	cmd.Stdin, cmd.Stdout = stdin, stdout
	var stderr limitedBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%s failed: %v: %s", name, err, strings.TrimSpace(m.redact(stderr.String())))
	}
	return nil
}

func (m *Manager) redact(msg string) string {
	for _, secret := range []string{m.cfg.PGPassword, m.cfg.RestorePassword, m.cfg.AdminPassword, m.cfg.ViewerPassword} {
		if secret != "" {
			msg = strings.ReplaceAll(msg, secret, "[redacted]")
		}
	}
	return msg
}

// secretFile writes content to a new private temp file and returns its path
// and a cleanup function. Client tools read credentials from it so they never
// appear in the process list.
func secretFile(pattern, content string) (string, func(), error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	path := f.Name()
	cleanup := func() { os.Remove(path) }
	if err = f.Chmod(0600); err == nil {
		_, err = f.WriteString(content)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

// dumpCompressed runs a tool that writes the dump to stdout and stores it
// gzip-compressed in out.
func (m *Manager) dumpCompressed(ctx context.Context, env []string, out, name string, args ...string) error {
	f, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	z := gzip.NewWriter(w)
	err = m.run(ctx, env, nil, z, name, args...)
	if e := z.Close(); err == nil {
		err = e
	}
	if e := w.Flush(); err == nil {
		err = e
	}
	if e := f.Close(); err == nil {
		err = e
	}
	return err
}

var gzipMagic = []byte{0x1f, 0x8b}

func isGzip(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	head := make([]byte, 2)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return false, err
	}
	return bytes.Equal(head[:n], gzipMagic), nil
}

// readGzip decompresses the whole file (checking its CRC) and returns the
// first and last n bytes of the content.
func readGzip(ctx context.Context, path string, n int) (head, tail []byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	z, err := gzip.NewReader(bufio.NewReaderSize(f, 1<<20))
	if err != nil {
		return nil, nil, err
	}
	defer z.Close()
	buf := make([]byte, 1<<20)
	for {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		k, e := z.Read(buf)
		if k > 0 {
			if len(head) < n {
				head = append(head, buf[:min(k, n-len(head))]...)
			}
			tail = append(tail, buf[:k]...)
			if len(tail) > n {
				tail = append(tail[:0], tail[len(tail)-n:]...)
			}
		}
		if e == io.EOF {
			return head, tail, nil
		}
		if e != nil {
			return nil, nil, e
		}
	}
}

// gzipInPlace compresses path, replacing it.
func gzipInPlace(path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := path + ".gz"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	z := gzip.NewWriter(out)
	_, err = io.Copy(z, in)
	if e := z.Close(); err == nil {
		err = e
	}
	if e := out.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// moveFile renames src to dst, copying when they are on different devices.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst, 0600); err != nil {
		os.Remove(dst)
		return err
	}
	os.Remove(src) // best effort: the copy is what matters
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if e := out.Close(); err == nil {
		err = e
	}
	if err == nil {
		err = os.Chmod(dst, mode) // umask may have narrowed it
	}
	return err
}

// validLocalDir reports whether dir is a usable absolute directory path.
func validLocalDir(dir string) bool {
	return filepath.IsAbs(dir) && filepath.Clean(dir) == dir && len(dir) <= 1024 && !strings.ContainsAny(dir, "\x00\r\n")
}

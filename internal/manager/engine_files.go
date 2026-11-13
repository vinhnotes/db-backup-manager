package manager

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The files engine backs up a directory mounted into the container (for
// example the upload folder of a web app) as a .tar.gz archive, and restores
// it into a new folder under a separate restore directory.
//
// A files connection maps onto the generic connection fields like this:
//   - LocalDir:  the source directory (also copied into Host for display);
//   - Database:  the name of the backup item (one item per connection);
//   - ServerDir: the restore directory, optional (also copied into
//     RestoreUsername so the existing "restore configured" checks apply).
const engineFiles = "files"

type filesEngine struct{}

func (filesEngine) name() string  { return engineFiles }
func (filesEngine) label() string { return "Files" }
func (filesEngine) ext() string   { return ".tar.gz" }
func (filesEngine) defaults() (string, string, string) {
	return "", "files", ""
}
func (filesEngine) sslModes() []string   { return []string{""} }
func (filesEngine) reserved(string) bool { return false }

// validateFilesConnection checks a files connection and fills the generic
// fields from the directories (see engineFiles).
func validateFilesConnection(c *Connection) error {
	c.LocalDir, c.ServerDir = strings.TrimSpace(c.LocalDir), strings.TrimSpace(c.ServerDir)
	if c.Database == "" {
		c.Database = filesItemName(c.LocalDir)
	}
	if c.Name == "" || len(c.Name) > 100 || !dbName.MatchString(c.Database) {
		return errors.New("Tên kết nối hoặc tên mục backup không hợp lệ")
	}
	if !validLocalDir(c.LocalDir) || c.LocalDir == "/" {
		return errors.New("Thư mục nguồn phải là đường dẫn tuyệt đối trong container, ví dụ /sources/uploads")
	}
	if c.ServerDir != "" && (!validLocalDir(c.ServerDir) || c.ServerDir == "/") {
		return errors.New("Thư mục restore phải là đường dẫn tuyệt đối trong container, ví dụ /restore")
	}
	if c.ServerDir != "" && (within(c.LocalDir, c.ServerDir) || within(c.ServerDir, c.LocalDir)) {
		return errors.New("Thư mục restore không được nằm trong thư mục nguồn (hoặc ngược lại)")
	}
	c.Host, c.Port, c.Username, c.Password, c.SSLMode = c.LocalDir, "", "", "", ""
	c.RestoreUsername, c.RestorePassword = c.ServerDir, ""
	return nil
}

// filesItemName is the default item name of a files connection: the last
// part of dir made a valid name. Mirrors filesItemName in web/src/lib.ts.
func filesItemName(dir string) string {
	var b strings.Builder
	for _, r := range path.Base(strings.TrimRight(dir, "/")) {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	name := b.String()
	if name == "_" || name == "" { // "." from an empty path, or "/"
		return "files"
	}
	if !dbName.MatchString(name[:1]) {
		name = "_" + name
	}
	return name[:min(len(name), 63)]
}

// within reports whether path p is dir or inside it.
func within(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (filesEngine) source(m *Manager) (string, error) {
	dir := m.cfg.LocalDir
	if !validLocalDir(dir) {
		return "", errors.New("Thư mục nguồn chưa được cấu hình")
	}
	if backups, err := filepath.Abs(m.cfg.BackupDir); err == nil && (within(dir, backups) || within(backups, dir)) {
		return "", errors.New("Thư mục nguồn không được chứa (hoặc nằm trong) thư mục lưu backup")
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("Không đọc được thư mục nguồn %s; kiểm tra volume đã mount vào container", dir)
	}
	return dir, nil
}

func (e filesEngine) databases(ctx context.Context, m *Manager) ([]Database, string, error) {
	dir, err := e.source(m)
	if err != nil {
		return nil, "", err
	}
	// Sizing walks the whole tree; give up quietly on very large folders so
	// the dashboard stays responsive (size 0 is shown as unknown).
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var size int64
	walkErr := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil && d.Type().IsRegular() {
			if info, e := d.Info(); e == nil {
				size += info.Size()
			}
		}
		return nil
	})
	if walkErr != nil {
		size = 0
	}
	return []Database{{Name: m.cfg.PGDatabase, Size: size}}, "", nil
}

func (e filesEngine) test(_ context.Context, m *Manager) error {
	dir, err := e.source(m)
	if err != nil {
		return err
	}
	f, err := os.Open(dir)
	if err == nil {
		_, err = f.Readdirnames(1)
		f.Close()
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("Không có quyền đọc thư mục nguồn %s (container chạy với UID 999)", dir)
	}
	if restore := m.cfg.ServerDir; restore != "" {
		probe, err := os.CreateTemp(restore, ".dbbm-write-test-")
		if err != nil {
			return fmt.Errorf("Không ghi được vào thư mục restore %s; kiểm tra volume đã mount và quyền ghi cho UID 999", restore)
		}
		probe.Close()
		os.Remove(probe.Name())
	}
	return nil
}

func (e filesEngine) dump(ctx context.Context, m *Manager, database, out string) error {
	if database != m.cfg.PGDatabase {
		return fmt.Errorf("Kết nối này chỉ có mục backup %q", m.cfg.PGDatabase)
	}
	dir, err := e.source(m)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	z := gzip.NewWriter(w)
	tw := tar.NewWriter(z)
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if walkErr != nil {
			if errors.Is(walkErr, fs.ErrNotExist) {
				return nil // deleted while walking
			}
			return fmt.Errorf("không đọc được %s: %w", rel, walkErr)
		}
		if rel == "." {
			return nil
		}
		return addTarEntry(tw, p, filepath.ToSlash(rel), d)
	})
	if e := tw.Close(); err == nil {
		err = e
	}
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

// addTarEntry writes one directory, regular file or symlink. Other file
// types (sockets, devices, pipes) are skipped.
func addTarEntry(tw *tar.Writer, full, name string, d fs.DirEntry) error {
	switch {
	case d.IsDir():
		info, err := d.Info()
		if err != nil {
			return ignoreGone(err, name)
		}
		return tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: name + "/", Mode: int64(info.Mode().Perm()), ModTime: info.ModTime()})
	case d.Type()&fs.ModeSymlink != 0:
		target, err := os.Readlink(full)
		if err != nil {
			return ignoreGone(err, name)
		}
		return tw.WriteHeader(&tar.Header{Typeflag: tar.TypeSymlink, Name: name, Linkname: target, Mode: 0777})
	case d.Type().IsRegular():
		f, err := os.Open(full)
		if err != nil {
			return ignoreGone(err, name)
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil // replaced while walking
		}
		if err = tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Size: info.Size(), Mode: int64(info.Mode().Perm()), ModTime: info.ModTime()}); err != nil {
			return err
		}
		// A file that grows while copying is stored as it was when opened;
		// one that shrinks fails the backup (it is then retried).
		if _, err = io.CopyN(tw, f, info.Size()); err != nil {
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("file %s đã thay đổi trong lúc backup", name)
			}
			return fmt.Errorf("không đọc được %s: %w", name, err)
		}
	}
	return nil
}

func ignoreGone(err error, name string) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("không đọc được %s: %w", name, err)
}

// safeEntry returns the cleaned relative path of an archive entry, or an
// error when the entry could write outside the extraction folder. "." is the
// folder itself.
func safeEntry(h *tar.Header) (string, error) {
	name := h.Name
	if name == "" || strings.ContainsRune(name, 0) || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("đường dẫn không hợp lệ trong archive: %q", name)
	}
	clean := path.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("đường dẫn ra ngoài thư mục trong archive: %q", name)
	}
	switch h.Typeflag {
	case tar.TypeReg, tar.TypeRegA, tar.TypeDir:
	case tar.TypeSymlink:
		// Links are restored as they were backed up; extract never writes
		// through one, wherever it points.
		if h.Linkname == "" || strings.ContainsRune(h.Linkname, 0) || clean == "." {
			return "", fmt.Errorf("symlink không hợp lệ trong archive: %q", name)
		}
	default:
		return "", fmt.Errorf("loại file không được hỗ trợ trong archive: %q (chỉ hỗ trợ thư mục, file thường và symlink)", name)
	}
	return clean, nil
}

// skipEntry reports metadata-only entries written by some tar tools.
func skipEntry(h *tar.Header) bool {
	return h.Typeflag == tar.TypeXGlobalHeader
}

// eachEntry reads every entry of a .tar.gz archive, checking each path, the
// tar structure and the gzip checksum. fn may read the entry content; any
// content left is read and discarded.
func eachEntry(ctx context.Context, file string, fn func(h *tar.Header, name string, r io.Reader) error) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	z, err := gzip.NewReader(bufio.NewReaderSize(f, 1<<20))
	if err != nil {
		return errors.New("không phải file .tar.gz hợp lệ")
	}
	defer z.Close()
	tr := tar.NewReader(z)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("archive tar bị hỏng: %w", err)
		}
		if skipEntry(h) {
			continue
		}
		name, err := safeEntry(h)
		if err != nil {
			return err
		}
		if fn != nil {
			if err = fn(h, name, tr); err != nil {
				return err
			}
		}
		if _, err = io.Copy(io.Discard, tr); err != nil {
			return fmt.Errorf("archive tar bị hỏng: %w", err)
		}
	}
	// Read to the end of the gzip stream so its checksum is verified.
	if _, err = io.Copy(io.Discard, z); err != nil {
		return fmt.Errorf("file gzip bị hỏng: %w", err)
	}
	return nil
}

func (filesEngine) verify(ctx context.Context, _ *Manager, file string) error {
	if gz, err := isGzip(file); err != nil || !gz {
		return errors.New("không phải file .tar.gz hợp lệ")
	}
	return eachEntry(ctx, file, nil)
}

func (e filesEngine) restore(ctx context.Context, m *Manager, target, _, file string) error {
	root := m.cfg.ServerDir
	if !validLocalDir(root) || !dbName.MatchString(target) {
		return errors.New("Chưa cấu hình thư mục restore hoặc tên thư mục đích không hợp lệ")
	}
	dest := filepath.Join(root, target)
	// Mkdir fails if the target exists: never write into an existing folder.
	if err := os.Mkdir(dest, 0750); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("thư mục đích %s đã tồn tại; chọn tên khác", target)
		}
		return fmt.Errorf("không tạo được thư mục đích %s: %w", target, err)
	}
	if err := extract(ctx, file, dest); err != nil {
		return fmt.Errorf("thư mục %s đã được tạo nhưng restore thất bại; kiểm tra và xóa thủ công trước khi thử lại: %w", target, err)
	}
	return nil
}

// extract unpacks a checked archive into the empty folder dest. Files are
// created exclusively and symlinks last, so nothing is written through a
// link; directory permissions are applied at the end.
func extract(ctx context.Context, file, dest string) error {
	type dirMeta struct {
		path  string
		mode  os.FileMode
		mtime time.Time
	}
	var dirs []dirMeta
	var links [][2]string
	err := eachEntry(ctx, file, func(h *tar.Header, name string, r io.Reader) error {
		if name == "." {
			return nil
		}
		full := filepath.Join(dest, filepath.FromSlash(name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(full, 0700); err != nil {
				return err
			}
			dirs = append(dirs, dirMeta{full, os.FileMode(h.Mode).Perm(), h.ModTime})
		case tar.TypeSymlink:
			links = append(links, [2]string{h.Linkname, full})
		default:
			if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
				return err
			}
			out, err := os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(h.Mode).Perm()|0600)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, r)
			if e := out.Close(); err == nil {
				err = e
			}
			if err != nil {
				return err
			}
			os.Chmod(full, os.FileMode(h.Mode).Perm())
			os.Chtimes(full, h.ModTime, h.ModTime)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Create every parent first, while no symlink exists yet: a parent path
	// can then never lead through a link (a link named like a parent fails).
	for _, l := range links {
		if err = os.MkdirAll(filepath.Dir(l[1]), 0700); err != nil {
			return err
		}
	}
	for _, l := range links {
		if err = os.Symlink(l[0], l[1]); err != nil {
			return err
		}
	}
	// Deepest first, so a read-only parent does not block its children.
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i].path) > len(dirs[j].path) })
	for _, d := range dirs {
		os.Chmod(d.path, d.mode|0700)
		os.Chtimes(d.path, d.mtime, d.mtime)
	}
	return nil
}

// ---- restore tests -----------------------------------------------------------

func (filesEngine) exists(_ context.Context, m *Manager, database string) (bool, error) {
	if !validLocalDir(m.cfg.ServerDir) {
		return false, errors.New("Chưa cấu hình thư mục restore")
	}
	_, err := os.Lstat(filepath.Join(m.cfg.ServerDir, database))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// inspect returns the number of regular files restored.
func (filesEngine) inspect(ctx context.Context, m *Manager, database string) (int, error) {
	n := 0
	err := filepath.WalkDir(filepath.Join(m.cfg.ServerDir, database), func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.Type().IsRegular() {
			n++
		}
		return nil
	})
	return n, err
}

func (filesEngine) drop(_ context.Context, m *Manager, database string) error {
	if !validLocalDir(m.cfg.ServerDir) || !dbName.MatchString(database) {
		return errors.New("invalid restore folder")
	}
	dir := filepath.Join(m.cfg.ServerDir, database)
	// Make every directory writable first so read-only ones can be emptied.
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			os.Chmod(p, 0700)
		}
		return nil
	})
	return os.RemoveAll(dir)
}

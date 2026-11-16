package manager

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

// allowedLocalDirectory restricts API-configured Files/SQL Server directories
// to mounts selected by the operator, independently of connection settings.
// Resolve symlinks before checking containment: a link inside an allowed mount
// must not grant access to a directory outside it.
func allowedLocalDirectory(dir string) (string, error) {
	if !validLocalDir(dir) || dir == "/" {
		return "", errors.New("invalid local directory")
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	for _, base := range strings.Split(env("LOCAL_PATH_ALLOWED_ROOTS", "/sources,/restore,/mssql-backup"), ",") {
		base = strings.TrimSpace(base)
		if !validLocalDir(base) || base == "/" {
			continue
		}
		root, err := filepath.EvalSymlinks(base)
		if err != nil || root == "/" {
			continue
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || !filepath.IsLocal(rel) {
			continue
		}
		return filepath.Join(root, rel), nil
	}
	return "", errors.New("local directory is outside LOCAL_PATH_ALLOWED_ROOTS")
}

// resolveLocalPath also handles an as-yet uncreated backup directory, resolving
// its existing parents so aliases such as /var -> /private/var compare equally.
func resolveLocalPath(name string) (string, error) {
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if !errors.Is(err, fs.ErrNotExist) {
		return resolved, err
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return "", err
	}
	root, err := resolveLocalPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.Base(abs)), nil
}

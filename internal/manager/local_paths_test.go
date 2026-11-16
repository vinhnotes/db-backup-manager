package manager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllowedLocalDirectory(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "allowed")
	inside := filepath.Join(root, "uploads")
	outside := filepath.Join(parent, "allowed-sibling")
	for _, dir := range []string{inside, outside} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("LOCAL_PATH_ALLOWED_ROOTS", root)
	for _, dir := range []string{root, inside} {
		got, err := allowedLocalDirectory(dir)
		want, resolveErr := filepath.EvalSymlinks(dir)
		if err != nil || resolveErr != nil || got != want {
			t.Fatalf("allowed directory %q: got %q, %v", dir, got, err)
		}
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{outside, link, root + "/../allowed-sibling", "/", "relative"} {
		if got, err := allowedLocalDirectory(dir); err == nil || got != "" {
			t.Errorf("unauthorized directory accepted: %q -> %q", dir, got)
		}
	}
	if err := checkSharedDir(outside); err == nil {
		t.Fatal("SQL Server probe wrote outside approved roots")
	}
	if err := checkSharedDir(inside); err != nil {
		t.Fatalf("approved SQL Server probe: %v", err)
	}
	entries, err := os.ReadDir(inside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("probe left files behind: %v %v", entries, err)
	}
	// A broad '/' setting is rejected rather than turning off containment.
	t.Setenv("LOCAL_PATH_ALLOWED_ROOTS", "/")
	if _, err := allowedLocalDirectory(inside); err == nil {
		t.Fatal("filesystem root allowed")
	}
}

func TestFilesRejectUnapprovedRuntimeDirectories(t *testing.T) {
	m, _, _ := filesManager(t)
	outside := t.TempDir()
	m.cfg.ServerDir = outside
	engine := filesEngine{}
	if err := engine.test(t.Context(), m); err == nil {
		t.Fatal("unapproved restore probe accepted")
	}
	if err := engine.restore(t.Context(), m, "copy", "files", "unused.tar.gz"); err == nil {
		t.Fatal("unapproved restore accepted")
	}
	if _, err := engine.exists(t.Context(), m, "copy"); err == nil {
		t.Fatal("unapproved restore lookup accepted")
	}
	if _, err := engine.inspect(t.Context(), m, "copy"); err == nil {
		t.Fatal("unapproved restore inspection accepted")
	}
	if err := engine.drop(t.Context(), m, "copy"); err == nil {
		t.Fatal("unapproved restore removal accepted")
	}
	m.cfg.LocalDir = outside
	if _, err := engine.source(m); err == nil {
		t.Fatal("unapproved backup source accepted")
	}
}

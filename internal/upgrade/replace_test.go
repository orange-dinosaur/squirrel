package upgrade

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func writeOld(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "squirrel")
	if err := os.WriteFile(p, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func checkFile(t *testing.T, p, want string) {
	t.Helper()
	got, err := os.ReadFile(p)
	if err != nil || string(got) != want {
		t.Fatalf("%s holds %q, %v; want %q", p, got, err, want)
	}
}

func TestReplace(t *testing.T) {
	p := writeOld(t, t.TempDir())
	if err := Replace(p, []byte("new")); err != nil {
		t.Fatal(err)
	}
	checkFile(t, p, "new")
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode is %v, want 0755", info.Mode().Perm())
	}
}

func TestReplaceFollowsSymlink(t *testing.T) {
	real := writeOld(t, t.TempDir())
	link := filepath.Join(t.TempDir(), "squirrel")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := Replace(link, []byte("new")); err != nil {
		t.Fatal(err)
	}
	checkFile(t, real, "new")
	if info, _ := os.Lstat(link); info.Mode()&fs.ModeSymlink == 0 {
		t.Error("the symlink was replaced by a file")
	}
}

func TestReplaceReadOnlyFolder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write a read-only folder")
	}
	dir := t.TempDir()
	p := writeOld(t, dir)
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	err := Replace(p, []byte("new"))
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("got %v, want a permission error", err)
	}
	checkFile(t, p, "old")
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("folder holds %d files, want only the old binary", len(entries))
	}
}

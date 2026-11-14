package core

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

func testEnv(t *testing.T) Env {
	home := t.TempDir()
	return Env{Home: home, Store: filepath.Join(home, ".squirrel"), OS: "macos"}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertLinked(t *testing.T, env Env, e Entry) {
	t.Helper()
	link, err := os.Readlink(env.Expand(e.Target))
	if err != nil || link != env.StorePath(e) {
		t.Errorf("%s links to %q, %v, want %q", e.Target, link, err, env.StorePath(e))
	}
}

func TestAddLocksStore(t *testing.T) {
	for name, prepare := range map[string]func(Env){
		"missing Store":  func(Env) {},
		"open old Store": func(env Env) { os.MkdirAll(env.Store, 0o755) },
	} {
		env := testEnv(t)
		prepare(env)
		write(t, filepath.Join(env.Home, ".zshrc"), "zsh")
		if _, err := Add(env, &Manifest{}, "~/.zshrc", nil); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(env.Store); err != nil || info.Mode().Perm() != 0o700 {
			t.Errorf("%s: Store mode %v, %v, want 0700", name, info.Mode().Perm(), err)
		}
	}
}

func TestAddFile(t *testing.T) {
	env := testEnv(t)
	write(t, filepath.Join(env.Home, ".zshrc"), "zsh")
	m := &Manifest{}

	e, err := Add(env, m, "~/.zshrc", []string{"shell"})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.ID) != 8 || e.Target != "~/.zshrc" || !slices.Equal(e.Tags, []string{"macos", "shell"}) {
		t.Errorf("Entry = %+v", e)
	}
	assertLinked(t, env, e)
	if b, _ := os.ReadFile(filepath.Join(env.Home, ".zshrc")); string(b) != "zsh" {
		t.Errorf("content through the Link = %q", b)
	}
	saved, _ := Load(env.Store)
	if !slices.Equal(saved.Entries[0].Tags, e.Tags) || saved.Entries[0].ID != e.ID {
		t.Errorf("saved Manifest = %+v", saved)
	}
}

func TestAddDirectory(t *testing.T) {
	env := testEnv(t)
	write(t, filepath.Join(env.Home, ".config/nvim/init.lua"), "vim")
	m := &Manifest{}

	e, err := Add(env, m, filepath.Join(env.Home, ".config/nvim"), []string{"linux"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(e.Tags, []string{"linux", "macos"}) {
		t.Errorf("tags = %v, want linux and this OS", e.Tags)
	}
	assertLinked(t, env, e)
	if _, err := os.Stat(filepath.Join(env.StorePath(e), "init.lua")); err != nil {
		t.Error(err)
	}
}

func TestAddRefusals(t *testing.T) {
	env := testEnv(t)
	outside := filepath.Join(t.TempDir(), "hosts")
	write(t, outside, "x")
	write(t, filepath.Join(env.Store, "stray"), "x")
	write(t, filepath.Join(env.Home, ".zshrc"), "x")
	write(t, filepath.Join(env.Home, ".vimrc"), "x")
	write(t, filepath.Join(env.Home, ".config/nvim/init.lua"), "x")
	os.Symlink(filepath.Join(env.Home, ".zshrc"), filepath.Join(env.Home, ".bashrc"))
	fifo := filepath.Join(env.Home, ".fifo")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Entries: []Entry{
		{ID: "11111111", Target: "~/.vimrc"},
		{ID: "22222222", Target: "~/.config/nvim"},
	}}

	cases := []struct {
		path, tag, want string
	}{
		{outside, "", "not inside HOME"},
		{env.Home, "", "not inside HOME"},
		{filepath.Join(env.Store, "stray"), "", "inside the Store"},
		{"~/.bashrc", "", "is a symlink"},
		{"~/.vimrc", "", "already managed by Entry 11111111"},
		{"~/.config/nvim/init.lua", "", "managed by Entry 22222222"},
		{"~/.zshrc", "Shell", "must use only"},
		{"~/.missing", "", "~/.missing does not exist"},
		{"", "", "a path cannot be empty"},
		{"~/.config", "", "~/.config contains ~/.config/nvim, managed by Entry 22222222: remove that Entry first"},
		{"~/.fifo", "", "~/.fifo is not a file or a folder"},
	}
	for _, c := range cases {
		var tags []string
		if c.tag != "" {
			tags = []string{c.tag}
		}
		if _, err := Add(env, m, c.path, tags); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Add(%q) error = %v, want %q", c.path, err, c.want)
		}
	}
	if len(m.Entries) != 2 {
		t.Errorf("refusals changed the Manifest: %+v", m.Entries)
	}
	for _, p := range []string{".config/nvim/init.lua", ".fifo"} {
		if _, err := os.Lstat(filepath.Join(env.Home, p)); err != nil {
			t.Errorf("a refusal moved ~/%s: %v", p, err)
		}
	}
	if _, err := m.Find(env, ""); err == nil || err.Error() != "a path cannot be empty" {
		t.Errorf("Find(\"\") error = %v", err)
	}
}

func TestAddRefusesFolderHoldingStore(t *testing.T) {
	env := testEnv(t)
	env.Store = filepath.Join(env.Home, ".config/store")
	write(t, filepath.Join(env.Home, ".config/app/x"), "x")
	if _, err := Add(env, &Manifest{}, "~/.config", nil); err == nil || err.Error() != "~/.config contains the Store" {
		t.Errorf("Add error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(env.Home, ".config/app/x")); err != nil {
		t.Errorf("~/.config moved: %v", err)
	}
}

func TestAddUndoWhenSaveFails(t *testing.T) {
	env := testEnv(t)
	zshrc := filepath.Join(env.Home, ".zshrc")
	write(t, zshrc, "zsh")
	write(t, filepath.Join(env.Store, manifestName, "blocker"), "")
	m := &Manifest{}

	if _, err := Add(env, m, zshrc, nil); err == nil || !strings.Contains(err.Error(), manifestName+".tmp-") {
		t.Fatalf("Add error = %v, want the Save rename error", err)
	}
	info, err := os.Lstat(zshrc)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("~/.zshrc after undo: %v, %v", info, err)
	}
	if b, _ := os.ReadFile(zshrc); string(b) != "zsh" {
		t.Errorf("content after undo = %q", b)
	}
	if len(m.Entries) != 0 {
		t.Errorf("Manifest kept the Entry: %+v", m.Entries)
	}
}

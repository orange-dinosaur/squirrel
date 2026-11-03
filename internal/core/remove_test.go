package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemove(t *testing.T) {
	env := testEnv(t)
	write(t, filepath.Join(env.Home, ".zshrc"), "zsh")
	write(t, filepath.Join(env.Home, ".config/nvim/init.lua"), "lua")
	m := &Manifest{}
	zsh, _ := Add(env, m, "~/.zshrc", nil)
	nvim, _ := Add(env, m, "~/.config/nvim", nil)

	for _, c := range []struct {
		arg, file, want string
		e               Entry
	}{
		{"~/.zshrc", ".zshrc", "zsh", zsh},
		{nvim.ID, ".config/nvim/init.lua", "lua", nvim},
	} {
		e, replaced, err := Remove(env, m, c.arg)
		if err != nil || !replaced || !e.Removed {
			t.Fatalf("Remove(%q) = %+v, %v, %v", c.arg, e, replaced, err)
		}
		if info, err := os.Lstat(env.Expand(c.e.Target)); err != nil || info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%s is still a Link or gone: %v", c.e.Target, err)
		}
		if got := readFile(t, filepath.Join(env.Home, c.file)); got != c.want {
			t.Errorf("%s holds %q, want %q", c.file, got, c.want)
		}
		if _, err := os.Stat(env.StorePath(c.e)); err != nil {
			t.Errorf("Store item for %s is gone: %v", c.e.Target, err)
		}
	}

	saved, _ := Load(env.Store)
	if !saved.Entries[0].Removed || !saved.Entries[1].Removed {
		t.Errorf("saved Manifest = %+v, want both removed", saved.Entries)
	}

	if _, _, err := Remove(env, m, zsh.ID); err == nil || !strings.Contains(err.Error(), "already removed") {
		t.Errorf("removing by ID again: %v, want already removed", err)
	}
	if _, _, err := Remove(env, m, "~/.zshrc"); err == nil || !strings.Contains(err.Error(), "no Entry") {
		t.Errorf("removing by path again: %v, want no Entry", err)
	}
}

func TestRemoveOtherOSByID(t *testing.T) {
	env := testEnv(t)
	m := &Manifest{Entries: []Entry{{ID: "00000001", Target: "~/.xinitrc", Tags: []string{"linux"}}}}
	write(t, env.StorePath(m.Entries[0]), "x")

	e, replaced, err := Remove(env, m, "00000001")
	if err != nil || replaced || !e.Removed {
		t.Fatalf("Remove = %+v, %v, %v", e, replaced, err)
	}
	if _, err := os.Lstat(filepath.Join(env.Home, ".xinitrc")); !os.IsNotExist(err) {
		t.Error("removing an Entry not linked here created its target")
	}
}

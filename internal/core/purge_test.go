package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPurge(t *testing.T) {
	env := testEnv(t)
	m := &Manifest{Entries: []Entry{
		{ID: "00000001", Target: "~/.vimrc", Removed: true},
		{ID: "00000002", Target: "~/.vimrc", Removed: true, Tags: []string{"macos"}},
		{ID: "00000003", Target: "~/.zshrc"},
		{ID: "00000004", Target: "~/.config/nvim", Removed: true},
	}}
	for _, e := range m.Entries[:3] {
		write(t, env.StorePath(e), "x")
	}
	write(t, filepath.Join(env.StorePath(m.Entries[3]), "init.lua"), "lua")

	ids := func(es []Entry) string {
		var s []string
		for _, e := range es {
			s = append(s, e.ID)
		}
		return strings.Join(s, " ")
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{nil, "00000001 00000002 00000004"},
		{[]string{"~/.vimrc"}, "00000001 00000002"},
		{[]string{"00000004", filepath.Join(env.Home, ".config/nvim")}, "00000004"},
	} {
		got, err := PurgeTargets(env, m, c.args)
		if err != nil || ids(got) != c.want {
			t.Errorf("PurgeTargets(%q) = %q, %v, want %q", c.args, ids(got), err, c.want)
		}
	}
	for _, c := range []struct{ arg, want string }{
		{"00000003", "not removed"},
		{"~/.zshrc", "no removed Entry"},
	} {
		if _, err := PurgeTargets(env, m, []string{c.arg}); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("PurgeTargets(%q) = %v, want %q", c.arg, err, c.want)
		}
	}

	// The second ~/.vimrc Store item is already gone, which is fine.
	os.Remove(env.StorePath(m.Entries[1]))
	targets, _ := PurgeTargets(env, m, nil)
	if err := Purge(env, m, targets); err != nil {
		t.Fatal(err)
	}
	saved, _ := Load(env.Store)
	if ids(saved.Entries) != "00000003" {
		t.Errorf("saved Manifest holds %q, want only the live Entry", ids(saved.Entries))
	}
	for _, e := range targets {
		if _, err := os.Lstat(env.StorePath(e)); !os.IsNotExist(err) {
			t.Errorf("Store item for %s survived: %v", e.ID, err)
		}
	}
	if _, err := os.Stat(env.StorePath(Entry{ID: "00000003", Target: "~/.zshrc"})); err != nil {
		t.Errorf("live Store item went: %v", err)
	}
}

func TestPurgeRefusesStillLinked(t *testing.T) {
	env := testEnv(t)
	write(t, filepath.Join(env.Home, ".zshrc"), "zsh")
	m := &Manifest{}
	e, _ := Add(env, m, "~/.zshrc", nil)
	m.Entries[0].Removed = true // marked removed, Link not yet swapped

	if _, err := PurgeTargets(env, m, nil); err == nil || !strings.Contains(err.Error(), "run squirrel apply first") {
		t.Errorf("PurgeTargets = %v, want still linked", err)
	}
	if _, err := PurgeTargets(env, m, []string{e.ID}); err == nil {
		t.Error("PurgeTargets by ID purged an Entry still linked here")
	}
}

package core

import (
	"path/filepath"
	"testing"
)

func TestValidTag(t *testing.T) {
	for tag, ok := range map[string]bool{"shell": true, "work-2": true, "": false, "Work": false, "a b": false, "x_y": false} {
		if ValidTag(tag) != ok {
			t.Errorf("ValidTag(%q) = %v, want %v", tag, !ok, ok)
		}
	}
}

func TestAppliesOn(t *testing.T) {
	cases := []struct {
		tags         []string
		macos, linux bool
	}{
		{nil, true, true},
		{[]string{"shell"}, true, true},
		{[]string{"linux", "shell"}, false, true},
		{[]string{"macos"}, true, false},
		{[]string{"linux", "macos"}, true, true},
	}
	for _, c := range cases {
		e := Entry{Tags: c.tags}
		if e.AppliesOn("macos") != c.macos || e.AppliesOn("linux") != c.linux {
			t.Errorf("tags %v: macos=%v linux=%v, want %v %v",
				c.tags, e.AppliesOn("macos"), e.AppliesOn("linux"), c.macos, c.linux)
		}
	}
}

func TestHasAnyTag(t *testing.T) {
	e := Entry{Tags: []string{"shell", "work"}}
	if !e.HasAnyTag(nil) || !e.HasAnyTag([]string{"editor", "work"}) || e.HasAnyTag([]string{"editor"}) {
		t.Error("HasAnyTag should match no tags or any one tag")
	}
}

func TestFind(t *testing.T) {
	home := t.TempDir()
	env := Env{Home: home, OS: "macos"}
	m := &Manifest{Entries: []Entry{
		{ID: "11111111", Target: "~/.zshrc", Removed: true},
		{ID: "22222222", Target: "~/.zshrc", Tags: []string{"linux"}},
		{ID: "33333333", Target: "~/.zshrc", Tags: []string{"macos"}},
	}}

	t.Chdir(home)
	for _, arg := range []string{"~/.zshrc", filepath.Join(home, ".zshrc"), ".zshrc"} {
		if i, err := m.Find(env, arg); err != nil || m.Entries[i].ID != "33333333" {
			t.Errorf("Find(%q) = %d, %v, want the macos Entry", arg, i, err)
		}
	}
	for _, id := range []string{"11111111", "22222222"} {
		if i, err := m.Find(env, id); err != nil || m.Entries[i].ID != id {
			t.Errorf("Find(%q) = %d, %v, want that Entry", id, i, err)
		}
	}
	for _, arg := range []string{"~/.vimrc", "/etc/hosts"} {
		if _, err := m.Find(env, arg); err == nil {
			t.Errorf("Find(%q) found an Entry, want an error", arg)
		}
	}
}

func TestClash(t *testing.T) {
	m := &Manifest{Entries: []Entry{
		{ID: "11111111", Target: "~/.zshrc", Tags: []string{"linux"}},
		{ID: "22222222", Target: "~/.vimrc", Removed: true},
	}}
	cases := []struct {
		e    Entry
		want string
	}{
		{Entry{ID: "new", Target: "~/.zshrc", Tags: []string{"macos"}}, ""},
		{Entry{ID: "new", Target: "~/.zshrc"}, "11111111"},
		{Entry{ID: "new", Target: "~/.zshrc", Tags: []string{"linux", "macos"}}, "11111111"},
		{Entry{ID: "11111111", Target: "~/.zshrc"}, ""},
		{Entry{ID: "new", Target: "~/.vimrc"}, ""},
	}
	for _, c := range cases {
		if got, _ := m.Clash(c.e); got != c.want {
			t.Errorf("Clash(%+v) = %q, want %q", c.e, got, c.want)
		}
	}
}

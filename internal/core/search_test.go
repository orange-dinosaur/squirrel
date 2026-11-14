package core

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchCandidates(t *testing.T) {
	env := testEnv(t)
	for _, f := range []string{
		".zshrc",
		"notes.txt",                  // not hidden
		"Documents/.hidden",          // hidden, but not directly in home
		".config/a/b/c",              // depth 4
		".config/a/b/c-dir/too-deep", // depth 5
		".config/nvim/init.lua",      // inside a managed directory
		".config/git/.git/HEAD",
		".config/node_modules/x",
		".cache/x",
		".Trash/x",
		".local/share/Trash/files/old.conf",
	} {
		write(t, filepath.Join(env.Home, f), "x")
	}
	m := &Manifest{}
	if _, err := Add(env, m, "~/.config/nvim", nil); err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, c := range SearchCandidates(env, m) {
		s := c.Target
		if c.Dir {
			s += "/"
		}
		got = append(got, s)
	}
	want := []string{
		"~/.config/a/", "~/.config/a/b/", "~/.config/a/b/c", "~/.config/a/b/c-dir/",
		"~/.config/git/", "~/.local/", "~/.local/share/", "~/.zshrc",
	}
	for _, s := range got {
		if strings.Contains(s, "Trash") {
			t.Errorf("SearchCandidates offered %s", s)
		}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("SearchCandidates =\n  %s\nwant\n  %s", strings.Join(got, " "), strings.Join(want, " "))
	}
}

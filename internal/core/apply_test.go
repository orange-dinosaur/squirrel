package core

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func ids(es []Entry) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.ID)
	}
	return out
}

func TestPlanApply(t *testing.T) {
	env := testEnv(t)
	m := &Manifest{Entries: []Entry{
		{ID: "00000001", Target: "~/linked"},
		{ID: "00000002", Target: "~/notlinked"},
		{ID: "00000003", Target: "~/.config/a/nested"},
		{ID: "00000004", Target: "~/.config/a/sibling"},
		{ID: "00000005", Target: "~/realfile"},
		{ID: "00000006", Target: "~/realdir"},
		{ID: "00000007", Target: "~/otherlink"},
		{ID: "00000008", Target: "~/missing"},
		{ID: "00000009", Target: "~/stilllinked", Removed: true},
		{ID: "00000010", Target: "~/removed", Removed: true},
		{ID: "00000011", Target: "~/linuxonly", Tags: []string{"linux"}},
	}}
	for _, e := range m.Entries {
		if e.ID != "00000008" {
			write(t, env.StorePath(e), "stored")
		}
	}
	home := func(name string) string { return filepath.Join(env.Home, name) }
	os.Symlink(env.StorePath(m.Entries[0]), home("linked"))
	write(t, home("realfile"), "x")
	os.Mkdir(home("realdir"), 0o755)
	os.Symlink(home("realfile"), home("otherlink"))
	os.Symlink(env.StorePath(m.Entries[8]), home("stilllinked"))
	before, _ := os.ReadDir(env.Home)

	p, err := PlanApply(env, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(p.Links); !slices.Equal(got, []string{"00000002", "00000003", "00000004"}) {
		t.Errorf("Links = %v", got)
	}
	if !slices.Equal(p.Parents, []string{home(".config/a")}) {
		t.Errorf("Parents = %v", p.Parents)
	}
	if got := ids(p.Unmanage); !slices.Equal(got, []string{"00000009"}) {
		t.Errorf("Unmanage = %v", got)
	}
	if got := ids(p.Missing); !slices.Equal(got, []string{"00000008"}) {
		t.Errorf("Missing = %v", got)
	}
	var conflicts []State
	for _, r := range p.Conflicts {
		conflicts = append(conflicts, r.State)
	}
	if !slices.Equal(conflicts, []State{ConflictFile, ConflictDir, ConflictLink}) {
		t.Errorf("Conflicts = %v", conflicts)
	}

	if after, _ := os.ReadDir(env.Home); len(after) != len(before) {
		t.Error("PlanApply changed HOME")
	}
}

func TestPlanApplyTags(t *testing.T) {
	env := testEnv(t)
	m := &Manifest{Entries: []Entry{
		{ID: "00000001", Target: "~/.zshrc", Tags: []string{"shell"}},
		{ID: "00000002", Target: "~/.vimrc", Tags: []string{"editor"}},
	}}
	for _, e := range m.Entries {
		write(t, env.StorePath(e), "stored")
	}
	p, err := PlanApply(env, m, []string{"shell"})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(p.Links); !slices.Equal(got, []string{"00000001"}) {
		t.Errorf("Links = %v, want only the shell Entry", got)
	}
}

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
	var missing []Entry
	for _, r := range p.Missing {
		missing = append(missing, r.Entry)
	}
	if got := ids(missing); !slices.Equal(got, []string{"00000008"}) {
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

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestApplySkipsSymlinkedStoreItem(t *testing.T) {
	env := testEnv(t)
	e := Entry{ID: "aaaaaaaa", Target: "~/.zshrc"}
	secret := filepath.Join(env.Home, "secret")
	write(t, secret, "PRIVATE")
	os.MkdirAll(env.Store, 0o700)
	if err := os.Symlink(secret, env.StorePath(e)); err != nil {
		t.Fatal(err)
	}
	if got, err := linkState(env, e); got != StoreItemLink || err != nil {
		t.Errorf("linkState = %q, %v", got, err)
	}
	p, err := PlanApply(env, &Manifest{Entries: []Entry{e}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := ExecuteApply(env, p, nil)
	if len(res.Linked) != 0 || len(res.Skipped) != 1 || res.Skipped[0].Reason != string(StoreItemLink) {
		t.Errorf("ExecuteApply = %+v", res)
	}
	if _, err := os.Lstat(env.Expand(e.Target)); err == nil {
		t.Error("~/.zshrc was linked")
	}
}

func TestApplySaysWhatIsInTheWay(t *testing.T) {
	env := testEnv(t)
	nvim := Entry{ID: "aaaaaaaa", Target: "~/.config/nvim"}
	config := Entry{ID: "bbbbbbbb", Target: "~/.config"}
	write(t, filepath.Join(env.StorePath(nvim), "init.lua"), "x")
	write(t, filepath.Join(env.StorePath(config), "app"), "y")
	p, err := PlanApply(env, &Manifest{Entries: []Entry{nvim, config}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := ExecuteApply(env, p, nil)
	if len(res.Skipped) != 1 || res.Skipped[0].Entry.ID != config.ID || res.Skipped[0].Reason != "something is already at the target" {
		t.Errorf("ExecuteApply = %+v", res)
	}
}

func TestExecuteApplyPrivateParent(t *testing.T) {
	env := testEnv(t)
	m := &Manifest{Entries: []Entry{{ID: "00000001", Target: "~/.newdir/f"}}}
	write(t, env.StorePath(m.Entries[0]), "k")
	p, err := PlanApply(env, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	ExecuteApply(env, p, nil)
	if info, err := os.Stat(filepath.Join(env.Home, ".newdir")); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("parent mode %v, %v, want 0700", info.Mode().Perm(), err)
	}
}

func TestExecuteApply(t *testing.T) {
	env := testEnv(t)
	m := &Manifest{Entries: []Entry{
		{ID: "00000001", Target: "~/.config/a/nested"},
		{ID: "00000002", Target: "~/backedup"},
		{ID: "00000003", Target: "~/overwritten"},
		{ID: "00000004", Target: "~/skipped"},
		{ID: "00000005", Target: "~/hasbackup"},
		{ID: "00000006", Target: "~/missing"},
		{ID: "00000007", Target: "~/removedfile", Removed: true},
		{ID: "00000008", Target: "~/removeddir", Removed: true},
	}}
	home := func(name string) string { return filepath.Join(env.Home, name) }
	for _, e := range m.Entries[:7] {
		if e.ID != "00000006" {
			write(t, env.StorePath(e), "stored "+e.ID)
		}
	}
	write(t, filepath.Join(env.StorePath(m.Entries[7]), "sub/init.lua"), "lua")
	write(t, home("backedup"), "mine")
	write(t, home("overwritten/old"), "old")
	write(t, home("skipped"), "mine")
	write(t, home("hasbackup"), "mine")
	write(t, home("hasbackup.squirrel-backup"), "older")
	os.Symlink(env.StorePath(m.Entries[6]), home("removedfile"))
	os.Symlink(env.StorePath(m.Entries[7]), home("removeddir"))

	p, err := PlanApply(env, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := ExecuteApply(env, p, map[string]Resolution{
		"00000002": Backup, "00000003": Overwrite, "00000004": Skip, "00000005": Backup,
	})

	if got := ids(res.Linked); !slices.Equal(got, []string{"00000001", "00000002", "00000003"}) {
		t.Errorf("Linked = %v", got)
	}
	for _, e := range m.Entries[:3] {
		assertLinked(t, env, e)
	}
	if got := readFile(t, home("backedup.squirrel-backup")); got != "mine" {
		t.Errorf("backup holds %q", got)
	}

	var skipped []string
	for _, s := range res.Skipped {
		skipped = append(skipped, s.Entry.ID+": "+s.Reason)
	}
	want := []string{
		"00000004: conflict: real file",
		"00000005: backup ~/hasbackup.squirrel-backup already exists",
		"00000006: missing from Store",
	}
	if !slices.Equal(skipped, want) {
		t.Errorf("Skipped = %q, want %q", skipped, want)
	}
	if readFile(t, home("skipped")) != "mine" || readFile(t, home("hasbackup")) != "mine" ||
		readFile(t, home("hasbackup.squirrel-backup")) != "older" {
		t.Error("a skipped Conflict changed the files in the way")
	}
	if res.OK() {
		t.Error("Result with skips reports OK")
	}

	if got := ids(res.Unmanaged); !slices.Equal(got, []string{"00000007", "00000008"}) {
		t.Errorf("Unmanaged = %v", got)
	}
	for _, name := range []string{"removedfile", "removeddir"} {
		if info, err := os.Lstat(home(name)); err != nil || info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("~/%s is still a Link or gone: %v", name, err)
		}
	}
	if readFile(t, home("removedfile")) != "stored 00000007" ||
		readFile(t, home("removeddir/sub/init.lua")) != "lua" {
		t.Error("unmanaged copies have the wrong content")
	}
	if _, err := os.Stat(env.StorePath(m.Entries[7])); err != nil {
		t.Error("unmanaging removed the Store item")
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

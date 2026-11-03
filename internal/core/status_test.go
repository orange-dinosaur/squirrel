package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatusStates(t *testing.T) {
	env := testEnv(t)
	entry := func(id, name string, tags ...string) Entry {
		return Entry{ID: id, Target: "~/" + name, Tags: tags}
	}
	m := &Manifest{Entries: []Entry{
		entry("00000001", "linked"),
		entry("00000002", "notlinked"),
		entry("00000003", "realfile"),
		entry("00000004", "realdir"),
		entry("00000005", "otherlink"),
		entry("00000006", "missing"),
		entry("00000007", "removed"),
		entry("00000008", "stilllinked"),
		entry("00000009", "linuxonly", "linux"),
	}}
	m.Entries[6].Removed = true
	m.Entries[7].Removed = true
	for _, e := range m.Entries {
		if e.ID != "00000006" {
			write(t, env.StorePath(e), "stored")
		}
	}
	home := func(name string) string { return filepath.Join(env.Home, name) }
	os.Symlink(env.StorePath(m.Entries[0]), home("linked"))
	write(t, home("realfile"), "x")
	os.Mkdir(home("realdir"), 0o755)
	os.Symlink(home("realfile"), home("otherlink"))
	write(t, home("removed"), "x")
	os.Symlink(env.StorePath(m.Entries[7]), home("stilllinked"))

	want := []State{Linked, NotLinked, ConflictFile, ConflictDir, ConflictLink,
		MissingFromStore, RemovedState, RemovedStillLinked, OtherOS}
	rows, err := Status(env, m, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(rows), len(want))
	}
	for i, r := range rows {
		if r.State != want[i] {
			t.Errorf("%s: state %q, want %q", r.Entry.Target, r.State, want[i])
		}
	}

	if rows, _ := Status(env, m, nil, false); len(rows) != len(want)-1 {
		t.Errorf("without --all got %d rows, want the other-OS Entry left out", len(rows))
	}
}

func TestStatusTagFilter(t *testing.T) {
	env := testEnv(t)
	m := &Manifest{Entries: []Entry{
		{ID: "00000001", Target: "~/.zshrc", Tags: []string{"shell"}},
		{ID: "00000002", Target: "~/.vimrc", Tags: []string{"editor"}},
		{ID: "00000003", Target: "~/.gitconfig", Tags: []string{"work"}, Removed: true},
	}}
	rows, err := Status(env, m, []string{"shell", "work"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Entry.ID != "00000001" || rows[1].Entry.ID != "00000003" {
		t.Errorf("rows = %+v, want the shell and work Entries", rows)
	}
}

func TestRowOK(t *testing.T) {
	for state, ok := range map[State]bool{Linked: true, RemovedState: true, OtherOS: true,
		NotLinked: false, ConflictFile: false, MissingFromStore: false, RemovedStillLinked: false} {
		if (Row{State: state}).OK() != ok {
			t.Errorf("%q OK = %v, want %v", state, !ok, ok)
		}
	}
}

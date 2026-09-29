package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	m, err := Load(t.TempDir())
	if err != nil || len(m.Entries) != 0 {
		t.Fatalf("Load of empty Store = %+v, %v", m, err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	store := t.TempDir()
	want := &Manifest{Entries: []Entry{
		{ID: "a1b2c3d4", Target: "~/.gitconfig", Tags: []string{"linux", "work"}},
		{ID: "7a8b9c0d", Target: "~/.vimrc", Removed: true},
	}}
	if err := want.Save(store); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join(store, manifestName))
	const text = `[[entry]]
  id = "a1b2c3d4"
  target = "~/.gitconfig"
  tags = ["linux", "work"]

[[entry]]
  id = "7a8b9c0d"
  target = "~/.vimrc"
  removed = true
`
	if string(raw) != text {
		t.Errorf("Manifest file:\n%s\nwant:\n%s", raw, text)
	}

	got, err := Load(store)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}

	files, _ := os.ReadDir(store)
	if len(files) != 1 {
		t.Errorf("Store holds %d files after Save, want only the Manifest", len(files))
	}
}

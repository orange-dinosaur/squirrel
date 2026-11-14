package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
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
	const text = `tags = ["linux", "work"]

[[entry]]
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

	if info, _ := os.Stat(filepath.Join(store, manifestName)); info.Mode().Perm() != 0o644 {
		t.Errorf("Manifest mode = %v, want 0644", info.Mode().Perm())
	}

	files, _ := os.ReadDir(store)
	if len(files) != 1 {
		t.Errorf("Store holds %d files after Save, want only the Manifest", len(files))
	}
}

func TestLoadRejectsBadManifest(t *testing.T) {
	entry := func(id, target string) string {
		return "[[entry]]\nid = \"" + id + "\"\ntarget = \"" + target + "\"\n"
	}
	badTarget := func(target string) string {
		return "the Manifest has a bad Entry (id aaaaaaaa): target " + strconv.Quote(target) + " must be a clean path inside ~"
	}
	cases := []struct{ toml, want string }{
		{entry("../my", "~/x"), `the Manifest has a bad Entry: id "../my" must be 8 hex digits`},
		{entry("", "~/x"), `the Manifest has a bad Entry: id "" must be 8 hex digits`},
		{entry("AAAAAAAA", "~/x"), `the Manifest has a bad Entry: id "AAAAAAAA" must be 8 hex digits`},
		{entry("aaaaaaaa", "~/x") + entry("aaaaaaaa", "~/y"), `the Manifest has two Entries with id "aaaaaaaa"`},
		{entry("aaaaaaaa", "~"), badTarget("~")},
		{entry("aaaaaaaa", "~/"), badTarget("~/")},
		{entry("aaaaaaaa", "/etc/x"), badTarget("/etc/x")},
		{entry("aaaaaaaa", "x"), badTarget("x")},
		{entry("aaaaaaaa", "~/../x"), badTarget("~/../x")},
		{entry("aaaaaaaa", "~/a/../b"), badTarget("~/a/../b")},
		{entry("aaaaaaaa", "~/a/"), badTarget("~/a/")},
		{entry("aaaaaaaa", `~/.z\u001b]0;x\u0007shrc`), badTarget("~/.z\x1b]0;x\ashrc")},
		{entry("aaaaaaaa", "~/x") + "tags = [\"BAD TAG\\u001b[31m\"]\n", `the Manifest has a bad tag "BAD TAG\x1b[31m": use only a-z, 0-9 and -`},
		{"tags = [\"Work\"]\n" + entry("aaaaaaaa", "~/x"), `the Manifest has a bad tag "Work": use only a-z, 0-9 and -`},
		{entry("aaaaaaaa", "~/x") + entry("bbbbbbbb", "~/x"), "the Manifest has two Entries for ~/x on the same OS (aaaaaaaa and bbbbbbbb)"},
		{"this is [not toml", "the Manifest STORE/squirrel.toml does not parse: "},
	}
	for _, c := range cases {
		store := t.TempDir()
		write(t, filepath.Join(store, manifestName), c.toml)
		want := strings.Replace(c.want, "STORE", store, 1)
		if _, err := Load(store); err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("Load(%q) = %v, want %q", c.toml, err, want)
		}
	}
}

func TestLoadAcceptsGoodManifest(t *testing.T) {
	store := t.TempDir()
	write(t, filepath.Join(store, manifestName), `tags = ["work"]
[[entry]]
id = "a1b2c3d4"
target = "~/.config/nvim"
tags = ["linux", "work"]
[[entry]]
id = "0f0f0f0f"
target = "~/.config/nvim"
tags = ["macos"]
[[entry]]
id = "7a8b9c0d"
target = "~/.config/nvim"
removed = true
`)
	if m, err := Load(store); err != nil || len(m.Entries) != 3 {
		t.Errorf("Load = %+v, %v", m, err)
	}
}

func TestLoadWithoutTagList(t *testing.T) {
	store := t.TempDir()
	const old = `[[entry]]
  id = "a1b2c3d4"
  target = "~/.gitconfig"
  tags = ["work", "linux"]

[[entry]]
  id = "7a8b9c0d"
  target = "~/.vimrc"
  tags = ["work"]
`
	if err := os.WriteFile(filepath.Join(store, manifestName), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(store)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"linux", "work"}; !reflect.DeepEqual(m.Tags, want) {
		t.Errorf("Tags = %v, want %v", m.Tags, want)
	}
}

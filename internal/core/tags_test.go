package core

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func tagEnv(t *testing.T) (Env, *Manifest) {
	env := testEnv(t)
	if err := os.MkdirAll(env.Store, 0o755); err != nil {
		t.Fatal(err)
	}
	return env, &Manifest{Entries: []Entry{
		{ID: "00000001", Target: "~/.zshrc", Tags: []string{"macos", "shell"}},
		{ID: "00000002", Target: "~/.zshrc", Tags: []string{"linux", "shell"}},
		{ID: "00000003", Target: "~/.vimrc", Tags: []string{"editor"}, Removed: true},
	}}
}

func TestTagUntag(t *testing.T) {
	env, m := tagEnv(t)

	e, err := Tag(env, m, "~/.zshrc", []string{"work", "shell", "alpha"})
	if err != nil || !slices.Equal(e.Tags, []string{"alpha", "macos", "shell", "work"}) {
		t.Fatalf("Tag = %v, %v", e.Tags, err)
	}
	e, err = Untag(env, m, "00000001", []string{"alpha", "missing"})
	if err != nil || !slices.Equal(e.Tags, []string{"macos", "shell", "work"}) {
		t.Fatalf("Untag = %v, %v", e.Tags, err)
	}
	saved, _ := Load(env.Store)
	if !slices.Equal(saved.Entries[0].Tags, e.Tags) {
		t.Errorf("saved tags = %v, want %v", saved.Entries[0].Tags, e.Tags)
	}
}

func TestSetTags(t *testing.T) {
	env, m := tagEnv(t)

	e, err := SetTags(env, m, "00000001", []string{"work", "macos"})
	if err != nil || !slices.Equal(e.Tags, []string{"macos", "work"}) {
		t.Fatalf("SetTags = %v, %v", e.Tags, err)
	}
	// Dropping macos would make it clash with the linux Entry on linux.
	if _, err := SetTags(env, m, "00000001", []string{"work"}); err == nil || !strings.Contains(err.Error(), "00000002") {
		t.Fatalf("SetTags clash = %v", err)
	}
	if !slices.Equal(m.Entries[0].Tags, []string{"macos", "work"}) {
		t.Errorf("tags after refusal = %v", m.Entries[0].Tags)
	}
}

func TestTagRefusals(t *testing.T) {
	env, m := tagEnv(t)
	if err := m.Save(env.Store); err != nil {
		t.Fatal(err)
	}

	if _, err := Tag(env, m, "~/.zshrc", []string{"Work"}); err == nil || !strings.Contains(err.Error(), "must use only") {
		t.Errorf("invalid name: %v", err)
	}
	if _, err := Untag(env, m, "~/.zshrc", []string{"macos"}); err == nil || !strings.Contains(err.Error(), "Entry 00000002") {
		t.Errorf("untag macos: %v, want a clash with Entry 00000002", err)
	}
	if _, err := Tag(env, m, "00000001", []string{"linux"}); err == nil || !strings.Contains(err.Error(), "Entry 00000002") {
		t.Errorf("tag linux: %v, want a clash with Entry 00000002", err)
	}
	if !slices.Equal(m.Entries[0].Tags, []string{"macos", "shell"}) {
		t.Errorf("a refused change altered the Entry: %v", m.Entries[0].Tags)
	}
}

func TestUntagLinkedOS(t *testing.T) {
	env := testEnv(t)
	write(t, env.Home+"/.zshrc", "zsh")
	m := &Manifest{}
	e, err := Add(env, m, "~/.zshrc", []string{"linux"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Untag(env, m, e.ID, []string{"macos"}); err == nil || !strings.Contains(err.Error(), "linked here") {
		t.Errorf("untag macos while linked: %v, want a refusal", err)
	}
	if _, err := Untag(env, m, e.ID, []string{"linux", "macos"}); err != nil {
		t.Errorf("untag both OS tags leaves it applying here, want no refusal: %v", err)
	}
	if _, err := Tag(env, m, e.ID, []string{"linux"}); err == nil || !strings.Contains(err.Error(), "linked here") {
		t.Errorf("tag linux on an untagged linked Entry: %v, want a refusal", err)
	}

	os.Remove(env.Home + "/.zshrc")
	if _, err := Tag(env, m, e.ID, []string{"linux"}); err != nil {
		t.Errorf("tag linux once the Link is gone: %v", err)
	}
}

func TestTagOutlivesLastEntry(t *testing.T) {
	env := testEnv(t)
	if err := os.MkdirAll(env.Store, 0o755); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Entries: []Entry{{ID: "00000001", Target: "~/.vimrc", Tags: []string{"work"}}}}
	if err := m.Save(env.Store); err != nil {
		t.Fatal(err)
	}
	if _, err := Untag(env, m, "00000001", []string{"work"}); err != nil {
		t.Fatal(err)
	}
	saved, err := Load(env.Store)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := Tags(saved), []TagCount{{"work", 0}}; !slices.Equal(got, want) {
		t.Errorf("Tags after untagging the last Entry = %v, want %v", got, want)
	}
}

func TestDeleteTagRefusals(t *testing.T) {
	env, m := tagEnv(t)
	m.Entries[0].Tags = append(m.Entries[0].Tags, "work")
	before := Tags(m)

	for tag, want := range map[string]string{
		"macos": "OS tag",
		"nope":  `no Tag "nope"`,
		"shell": "shell is on 2 entries",
		"work":  "work is on 1 entry",
	} {
		if _, err := DeleteTag(env, m, tag, false); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("DeleteTag(%s) = %v, want %q", tag, err, want)
		}
	}
	if got := Tags(m); !slices.Equal(got, before) {
		t.Errorf("a refused delete changed the Tags: %v, want %v", got, before)
	}
}

func TestDeleteTag(t *testing.T) {
	env, m := tagEnv(t)

	// Only the Removed Entry carries editor, so nothing blocks it.
	if untagged, err := DeleteTag(env, m, "editor", false); err != nil || len(untagged) != 0 {
		t.Fatalf("DeleteTag(editor) = %v, %v", untagged, err)
	}
	if len(m.Entries[2].Tags) != 0 {
		t.Errorf("Removed Entry kept %v", m.Entries[2].Tags)
	}

	untagged, err := DeleteTag(env, m, "shell", true)
	if err != nil || len(untagged) != 2 || untagged[0].ID != "00000001" || !slices.Equal(untagged[0].Tags, []string{"macos"}) {
		t.Fatalf("DeleteTag(shell, untag) = %v, %v", untagged, err)
	}
	saved, err := Load(env.Store)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := Tags(saved), []TagCount{{"linux", 1}, {"macos", 1}}; !slices.Equal(got, want) {
		t.Errorf("saved Tags = %v, want %v", got, want)
	}
}

func TestTags(t *testing.T) {
	_, m := tagEnv(t)
	want := []TagCount{{"editor", 0}, {"linux", 1}, {"macos", 1}, {"shell", 2}}
	if got := Tags(m); !slices.Equal(got, want) {
		t.Errorf("Tags = %v, want %v", got, want)
	}
}

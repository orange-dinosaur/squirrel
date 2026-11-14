package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
)

const manifestName = "squirrel.toml"

type Entry struct {
	ID      string   `toml:"id"`
	Target  string   `toml:"target"`
	Tags    []string `toml:"tags,omitempty"`
	Removed bool     `toml:"removed,omitempty"`
}

type Manifest struct {
	Tags    []string `toml:"tags,omitempty"` // every Tag squirrel remembers, carried or not
	Entries []Entry  `toml:"entry"`
}

// Load reads the Manifest in store. A missing file is an empty Manifest.
func Load(store string) (*Manifest, error) {
	var m Manifest
	file := filepath.Join(store, manifestName)
	_, err := toml.DecodeFile(file, &m)
	if errors.Is(err, fs.ErrNotExist) {
		return &m, nil
	}
	if err != nil {
		return &m, fmt.Errorf("the Manifest %s does not parse: %w", file, err)
	}
	m.remember()
	return &m, m.validate()
}

var entryID = regexp.MustCompile(`^[0-9a-f]{8}$`)

// validate refuses a Manifest whose IDs or targets would point outside the
// Store or home, or that the rest of squirrel assumes cannot happen.
func (m *Manifest) validate() error {
	for _, e := range m.Entries {
		if !entryID.MatchString(e.ID) {
			return fmt.Errorf("the Manifest has a bad Entry: id %q must be 8 hex digits", e.ID)
		}
	}
	seen := map[string]bool{}
	for _, e := range m.Entries {
		if seen[e.ID] {
			return fmt.Errorf("the Manifest has two Entries with id %q", e.ID)
		}
		seen[e.ID] = true
	}
	for _, e := range m.Entries {
		t := e.Target
		if !strings.HasPrefix(t, "~/") || len(t) <= 2 || t != path.Clean(t) || strings.ContainsFunc(t, unicode.IsControl) {
			return fmt.Errorf("the Manifest has a bad Entry (id %s): target %q must be a clean path inside ~", e.ID, t)
		}
	}
	// remember has already merged every Entry's tags into m.Tags.
	for _, t := range m.Tags {
		if !ValidTag(t) {
			return fmt.Errorf("the Manifest has a bad tag %q: use only a-z, 0-9 and -", t)
		}
	}
	for _, e := range m.Entries {
		if e.Removed {
			continue
		}
		if other, clash := m.Clash(e); clash {
			return fmt.Errorf("the Manifest has two Entries for %s on the same OS (%s and %s)", e.Target, e.ID, other)
		}
	}
	return nil
}

// remember adds the Entries' Tags to the Tag list, so the list never misses
// one, including in a Manifest written before it had a list.
func (m *Manifest) remember() {
	for _, e := range m.Entries {
		m.Tags = append(m.Tags, e.Tags...)
	}
	slices.Sort(m.Tags)
	m.Tags = slices.Compact(m.Tags)
}

// Save writes the Manifest to a temp file in store, then renames it over the
// old one, so a crash never leaves a half-written Manifest.
func (m *Manifest) Save(store string) error {
	f, err := os.CreateTemp(store, manifestName+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	m.remember()
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	if err := toml.NewEncoder(f).Encode(m); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(store, manifestName))
}

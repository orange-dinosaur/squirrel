package core

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

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
	_, err := toml.DecodeFile(filepath.Join(store, manifestName), &m)
	if errors.Is(err, fs.ErrNotExist) {
		return &m, nil
	}
	m.remember()
	return &m, err
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

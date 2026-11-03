package core

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

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
	Entries []Entry `toml:"entry"`
}

// Load reads the Manifest in store. A missing file is an empty Manifest.
func Load(store string) (*Manifest, error) {
	var m Manifest
	_, err := toml.DecodeFile(filepath.Join(store, manifestName), &m)
	if errors.Is(err, fs.ErrNotExist) {
		return &m, nil
	}
	return &m, err
}

// Save writes the Manifest to a temp file in store, then renames it over the
// old one, so a crash never leaves a half-written Manifest.
func (m *Manifest) Save(store string) error {
	f, err := os.CreateTemp(store, manifestName+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
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

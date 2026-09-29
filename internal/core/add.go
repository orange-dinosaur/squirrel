package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// Add moves the item at path into the Store, links it back, and saves the
// Manifest. Tags get this OS's tag unless they already name an OS. If a later
// stage fails, the earlier ones are undone so the item ends up where it was.
func Add(env Env, m *Manifest, path string, tags []string) (Entry, error) {
	e, abs, err := checkAdd(env, m, path, tags)
	if err != nil {
		return Entry{}, err
	}

	if err := os.MkdirAll(env.Store, 0o755); err != nil {
		return Entry{}, err
	}
	stored := env.StorePath(e)
	if err := os.Rename(abs, stored); err != nil {
		if errors.Is(err, syscall.EXDEV) {
			return Entry{}, fmt.Errorf("the Store %s is on another disk than %s", env.Store, path)
		}
		return Entry{}, err
	}
	if err := os.Symlink(stored, abs); err != nil {
		return Entry{}, errors.Join(err, os.Rename(stored, abs))
	}
	m.Entries = append(m.Entries, e)
	if err := m.Save(env.Store); err != nil {
		m.Entries = m.Entries[:len(m.Entries)-1]
		return Entry{}, errors.Join(err, os.Remove(abs), os.Rename(stored, abs))
	}
	return e, nil
}

// checkAdd applies every refusal and returns the new Entry and the absolute path.
func checkAdd(env Env, m *Manifest, path string, tags []string) (Entry, string, error) {
	abs, err := filepath.Abs(env.Expand(path))
	if err != nil {
		return Entry{}, "", err
	}
	target, ok := env.Contract(abs)
	if !ok || target == "~" {
		return Entry{}, "", fmt.Errorf("%s is not inside HOME", path)
	}
	if rel, err := filepath.Rel(env.Store, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
		return Entry{}, "", fmt.Errorf("%s is inside the Store", path)
	}
	for _, o := range m.Entries {
		if !o.Removed && o.AppliesOn(env.OS) && strings.HasPrefix(target, o.Target+"/") {
			return Entry{}, "", fmt.Errorf("%s is inside %s, managed by Entry %s", path, o.Target, o.ID)
		}
	}

	for _, t := range tags {
		if !ValidTag(t) {
			return Entry{}, "", fmt.Errorf("tag %q must use only a-z, 0-9 and -", t)
		}
	}
	tags = slices.Clone(tags)
	if !slices.ContainsFunc(tags, func(t string) bool { return slices.Contains(osTags, t) }) {
		tags = append(tags, env.OS)
	}
	slices.Sort(tags)
	e := Entry{ID: newID(m), Target: target, Tags: slices.Compact(tags)}
	if id, ok := m.Clash(e); ok {
		return Entry{}, "", fmt.Errorf("%s is already managed by Entry %s", target, id)
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return Entry{}, "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Entry{}, "", fmt.Errorf("%s is a symlink", path)
	}
	return e, abs, nil
}

func newID(m *Manifest) string {
	b := make([]byte, 4)
	for {
		rand.Read(b)
		id := hex.EncodeToString(b)
		if !slices.ContainsFunc(m.Entries, func(e Entry) bool { return e.ID == id }) {
			return id
		}
	}
}

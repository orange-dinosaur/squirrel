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
// Manifest. Tags always get this OS's tag, since the Link is made here. If a later
// stage fails, the earlier ones are undone so the item ends up where it was.
// lockStore keeps other users out of the Store. git does not keep file modes
// beyond the executable bit, so the folder's mode is what protects them.
func lockStore(store string) error { return os.Chmod(store, 0o700) }

func Add(env Env, m *Manifest, path string, tags []string) (Entry, error) {
	e, abs, err := checkAdd(env, m, path, tags)
	if err != nil {
		return Entry{}, err
	}
	e.ID = newID(m)

	if err := os.MkdirAll(env.Store, 0o700); err != nil {
		return Entry{}, err
	}
	if err := lockStore(env.Store); err != nil {
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

// CheckAdd reports why Add would refuse path, or nil. Tags chosen later are
// checked again by Add itself.
func CheckAdd(env Env, m *Manifest, path string) error {
	_, _, err := checkAdd(env, m, path, nil)
	return err
}

// checkAdd applies every refusal and returns the new Entry and the absolute path.
func checkAdd(env Env, m *Manifest, path string, tags []string) (Entry, string, error) {
	if path == "" {
		return Entry{}, "", errors.New("a path cannot be empty")
	}
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
	if rel, err := filepath.Rel(abs, env.Store); err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
		return Entry{}, "", fmt.Errorf("%s contains the Store", path)
	}
	for _, o := range m.Entries {
		if o.Removed || !o.AppliesOn(env.OS) {
			continue
		}
		if strings.HasPrefix(target, o.Target+"/") {
			return Entry{}, "", fmt.Errorf("%s is inside %s, managed by Entry %s", path, o.Target, o.ID)
		}
		if strings.HasPrefix(o.Target, target+"/") {
			return Entry{}, "", fmt.Errorf("%s contains %s, managed by Entry %s: remove that Entry first", path, o.Target, o.ID)
		}
	}

	for _, t := range tags {
		if !ValidTag(t) {
			return Entry{}, "", fmt.Errorf("tag %q must use only a-z, 0-9 and -", t)
		}
	}
	tags = append(slices.Clone(tags), env.OS)
	slices.Sort(tags)
	e := Entry{Target: target, Tags: slices.Compact(tags)}
	if id, ok := m.Clash(e); ok {
		return Entry{}, "", fmt.Errorf("%s is already managed by Entry %s", target, id)
	}
	info, err := os.Lstat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return Entry{}, "", fmt.Errorf("%s does not exist", path)
	}
	if err != nil {
		return Entry{}, "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Entry{}, "", fmt.Errorf("%s is a symlink", path)
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return Entry{}, "", fmt.Errorf("%s is not a file or a folder", path)
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

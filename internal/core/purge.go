package core

import (
	"errors"
	"fmt"
	"os"
	"slices"
)

// PurgeTargets resolves purge arguments to Removed Entries. No arguments means
// every Removed Entry, on any OS. An ID reaches that Entry and a path reaches
// every Removed Entry for that target. It refuses an Entry still linked here.
func PurgeTargets(env Env, m *Manifest, args []string) ([]Entry, error) {
	var out []Entry
	add := func(e Entry) {
		if !slices.ContainsFunc(out, func(o Entry) bool { return o.ID == e.ID }) {
			out = append(out, e)
		}
	}
	if len(args) == 0 {
		for _, e := range m.Entries {
			if e.Removed {
				add(e)
			}
		}
	}
	for _, arg := range args {
		if i := slices.IndexFunc(m.Entries, func(e Entry) bool { return e.ID == arg }); i >= 0 {
			e := m.Entries[i]
			if !e.Removed {
				return nil, fmt.Errorf("Entry %s for %s is not removed; remove it first", e.ID, e.Target)
			}
			add(e)
			continue
		}
		target, err := env.targetOf(arg)
		if err != nil {
			return nil, err
		}
		found := false
		for _, e := range m.Entries {
			if e.Removed && e.Target == target {
				add(e)
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("no removed Entry for %s", target)
		}
	}
	for _, e := range out {
		if isLink(env, e) {
			return nil, fmt.Errorf("%s (Entry %s) is still linked here: run squirrel apply first", e.Target, e.ID)
		}
	}
	return out, nil
}

// Purge drops entries from the Manifest, saves it, then deletes their Store
// items. A Store item that is already gone is fine.
func Purge(env Env, m *Manifest, entries []Entry) error {
	before := m.Entries
	m.Entries = slices.DeleteFunc(slices.Clone(m.Entries), func(e Entry) bool {
		return slices.ContainsFunc(entries, func(p Entry) bool { return p.ID == e.ID })
	})
	if err := m.Save(env.Store); err != nil {
		m.Entries = before
		return err
	}
	var errs []error
	for _, e := range entries {
		if err := os.RemoveAll(env.StorePath(e)); err != nil {
			errs = append(errs, fmt.Errorf("purged %s from the Manifest, but its Store item is still there: %w", e.Target, err))
		}
	}
	return errors.Join(errs...)
}

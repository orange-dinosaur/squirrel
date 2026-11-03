package core

import "fmt"

// Remove marks the Entry for arg removed and, when its target here is still
// the Link, replaces the Link with a copy of the Store item. The Store item
// stays for other machines. It reports whether a Link was replaced.
//
// The Manifest is saved first. If the copy then fails, the Entry shows as
// "removed, still linked" and apply finishes the job.
func Remove(env Env, m *Manifest, arg string) (Entry, bool, error) {
	i, err := m.Find(env, arg)
	if err != nil {
		return Entry{}, false, err
	}
	e := m.Entries[i]
	if e.Removed {
		return e, false, fmt.Errorf("Entry %s for %s is already removed", e.ID, e.Target)
	}
	m.Entries[i].Removed = true
	if err := m.Save(env.Store); err != nil {
		m.Entries[i].Removed = false
		return e, false, err
	}
	e.Removed = true
	if !isLink(env, e) {
		return e, false, nil
	}
	if err := unmanage(env, e); err != nil {
		return e, false, fmt.Errorf("%s is marked removed, but its Link is still there: %w", e.Target, err)
	}
	return e, true, nil
}

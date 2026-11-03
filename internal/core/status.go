package core

import (
	"errors"
	"io/fs"
	"os"
)

type State string

const (
	Linked             State = "linked"
	NotLinked          State = "not linked"
	ConflictFile       State = "conflict: real file"
	ConflictDir        State = "conflict: directory"
	ConflictLink       State = "conflict: other link"
	MissingFromStore   State = "missing from Store"
	RemovedState       State = "removed"
	RemovedStillLinked State = "removed, still linked"
	OtherOS            State = "other OS"
)

type Row struct {
	Entry Entry
	State State
}

// OK reports whether the row needs no action.
func (r Row) OK() bool {
	return r.State == Linked || r.State == RemovedState || r.State == OtherOS
}

// Status returns one row per Entry that applies on this OS, plus every
// Removed Entry. With all, Entries for other OSes are included too.
// Tags narrow the rows to Entries with any of them.
func Status(env Env, m *Manifest, tags []string, all bool) ([]Row, error) {
	var rows []Row
	for _, e := range m.Entries {
		if !e.HasAnyTag(tags) {
			continue
		}
		var state State
		var err error
		switch {
		case e.Removed:
			state = RemovedState
			if isLink(env, e) {
				state = RemovedStillLinked
			}
		case !e.AppliesOn(env.OS):
			if !all {
				continue
			}
			state = OtherOS
		default:
			state, err = linkState(env, e)
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, Row{e, state})
	}
	return rows, nil
}

// linkState compares an applying Entry's target with its Store item.
func linkState(env Env, e Entry) (State, error) {
	if _, err := os.Lstat(env.StorePath(e)); errors.Is(err, fs.ErrNotExist) {
		return MissingFromStore, nil
	} else if err != nil {
		return "", err
	}
	info, err := os.Lstat(env.Expand(e.Target))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return NotLinked, nil
	case err != nil:
		return "", err
	case info.Mode()&os.ModeSymlink != 0:
		if isLink(env, e) {
			return Linked, nil
		}
		return ConflictLink, nil
	case info.IsDir():
		return ConflictDir, nil
	}
	return ConflictFile, nil
}

// isLink reports whether the target is a symlink to the Entry's Store item.
func isLink(env Env, e Entry) bool {
	dest, err := os.Readlink(env.Expand(e.Target))
	return err == nil && dest == env.StorePath(e)
}

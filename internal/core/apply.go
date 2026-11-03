package core

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// Plan is what apply would do, worked out without touching anything.
type Plan struct {
	Links     []Entry  // targets to link
	Parents   []string // missing parent directories of those targets
	Unmanage  []Entry  // Removed Entries whose target is still the Link
	Conflicts []Row    // targets in the way, with their conflict State
	Missing   []Entry  // Entries whose Store item is gone, skipped
}

// PlanApply works out what apply does for the Entries with any of tags, or all
// Entries when tags is empty.
func PlanApply(env Env, m *Manifest, tags []string) (Plan, error) {
	rows, err := Status(env, m, tags, false)
	if err != nil {
		return Plan{}, err
	}
	var p Plan
	for _, r := range rows {
		switch r.State {
		case NotLinked:
			p.Links = append(p.Links, r.Entry)
			parent := filepath.Dir(env.Expand(r.Entry.Target))
			if _, err := os.Lstat(parent); errors.Is(err, fs.ErrNotExist) {
				if !slices.Contains(p.Parents, parent) {
					p.Parents = append(p.Parents, parent)
				}
			} else if err != nil {
				return Plan{}, err
			}
		case ConflictFile, ConflictDir, ConflictLink:
			p.Conflicts = append(p.Conflicts, r)
		case MissingFromStore:
			p.Missing = append(p.Missing, r.Entry)
		case RemovedStillLinked:
			p.Unmanage = append(p.Unmanage, r.Entry)
		}
	}
	return p, nil
}

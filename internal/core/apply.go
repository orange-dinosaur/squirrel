package core

import (
	"errors"
	"fmt"
	"io"
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

// Resolution is what to do with a Conflict.
type Resolution string

const (
	Backup    Resolution = "backup"
	Overwrite Resolution = "overwrite"
	Skip      Resolution = "skip"
)

type Skipped struct {
	Entry  Entry
	Reason string
}

type Result struct {
	Linked    []Entry
	Unmanaged []Entry
	Skipped   []Skipped
}

// OK reports whether apply did everything it was asked to.
func (r Result) OK() bool { return len(r.Skipped) == 0 }

// ExecuteApply carries out a Plan. resolve maps an Entry ID to the Resolution
// for its Conflict; a Conflict with no Resolution is skipped. A failure on one
// Entry is recorded in Skipped and the rest carry on.
func ExecuteApply(env Env, p Plan, resolve map[string]Resolution) Result {
	var res Result
	skip := func(e Entry, reason string) { res.Skipped = append(res.Skipped, Skipped{e, reason}) }
	link := func(e Entry) {
		target := env.Expand(e.Target)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			skip(e, err.Error())
		} else if err := os.Symlink(env.StorePath(e), target); err != nil {
			skip(e, err.Error())
		} else {
			res.Linked = append(res.Linked, e)
		}
	}

	for _, e := range p.Links {
		link(e)
	}
	for _, c := range p.Conflicts {
		e, target := c.Entry, env.Expand(c.Entry.Target)
		switch resolve[e.ID] {
		case Backup:
			backup := target + ".squirrel-backup"
			if _, err := os.Lstat(backup); err == nil {
				skip(e, fmt.Sprintf("backup %s already exists", e.Target+".squirrel-backup"))
			} else if err := os.Rename(target, backup); err != nil {
				skip(e, err.Error())
			} else {
				link(e)
			}
		case Overwrite:
			if err := os.RemoveAll(target); err != nil {
				skip(e, err.Error())
			} else {
				link(e)
			}
		default:
			skip(e, string(c.State))
		}
	}
	for _, e := range p.Unmanage {
		if err := unmanage(env, e); err != nil {
			skip(e, err.Error())
		} else {
			res.Unmanaged = append(res.Unmanaged, e)
		}
	}
	for _, e := range p.Missing {
		skip(e, string(MissingFromStore))
	}
	return res
}

// unmanage replaces an Entry's Link with a real copy of its Store item. The
// copy is made beside the Link first, so a failed copy leaves the Link alone.
func unmanage(env Env, e Entry) error {
	target := env.Expand(e.Target)
	tmp := target + ".squirrel-tmp"
	if _, err := os.Lstat(tmp); err == nil {
		return fmt.Errorf("%s.squirrel-tmp is in the way", e.Target)
	}
	if err := copyItem(env.StorePath(e), tmp); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.Remove(target); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	return os.Rename(tmp, target)
}

// copyItem copies a file or a whole directory from src to a new path dst.
func copyItem(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.CopyFS(dst, os.DirFS(src))
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

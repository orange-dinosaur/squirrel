package core

import (
	"errors"
	"fmt"
	"slices"

	"github.com/BurntSushi/toml"
)

// DirtyError lists the Store paths that changed since the last commit.
type DirtyError struct{ Paths []string }

func (e DirtyError) Error() string {
	return "the Store has changes that are not pushed"
}

// dirtyPaths lists the Store paths that changed since the last commit.
func dirtyPaths(store string) ([]string, error) {
	return statusPaths(store, "")
}

// RemoteStatus is how the Store stands against its Remote.
type RemoteStatus struct {
	URL           string
	Ahead, Behind int      // commits only the Store has, and only the Remote has
	Dirty         []string // Store paths changed since the last commit
	Ignored       []string // Store paths git ignores, so they never reach the Remote
}

// RemoteState fetches and compares the Store with its Remote.
func RemoteState(env Env) (RemoteStatus, error) {
	var s RemoteStatus
	store := env.Store
	url, err := requireRemote(store)
	if err != nil {
		return s, err
	}
	s.URL = url
	if s.Dirty, err = dirtyPaths(store); err != nil {
		return s, err
	}
	if s.Ignored, err = statusPaths(store, "!!", "--ignored"); err != nil {
		return s, err
	}
	if _, err := git(store, "fetch", "-q", "origin"); err != nil {
		return s, err
	}
	head, remote := hasRef(store, "HEAD"), hasRef(store, "origin/main")
	var counts string
	switch {
	case head && remote:
		s.Ahead, s.Behind, err = aheadBehind(store)
		return s, err
	case head:
		counts, err = git(store, "rev-list", "--count", "HEAD")
	case remote:
		counts, err = git(store, "rev-list", "--count", "origin/main")
		counts = "0 " + counts
	}
	fmt.Sscan(counts, &s.Ahead, &s.Behind)
	return s, err
}

// aheadBehind counts the commits HEAD has that origin/main lacks, and the reverse.
func aheadBehind(store string) (ahead, behind int, err error) {
	counts, err := git(store, "rev-list", "--left-right", "--count", "HEAD...origin/main")
	if err != nil {
		return 0, 0, err
	}
	fmt.Sscan(counts, &ahead, &behind)
	return ahead, behind, nil
}

// PullResult says what Pull did.
type PullResult struct {
	Moved bool    // the Store fast-forwarded to the Remote
	Kept  []Entry // linked Entries the Remote no longer has, now real copies
}

// Pull fast-forwards the Store to the Remote. It refuses when the Store has
// changes of its own, and never merges.
func Pull(env Env) (PullResult, error) {
	var res PullResult
	store := env.Store
	if _, err := requireRemote(store); err != nil {
		return res, err
	}
	if err := lockStore(store); err != nil {
		return res, err
	}
	paths, err := dirtyPaths(store)
	if err != nil {
		return res, err
	}
	if paths != nil {
		return res, DirtyError{paths}
	}
	if _, err := git(store, "fetch", "-q", "origin"); err != nil {
		return res, err
	}
	if !hasRef(store, "origin/main") {
		return res, nil // the Remote is still empty
	}
	if hasRef(store, "HEAD") {
		ahead, behind, err := aheadBehind(store)
		if err != nil {
			return res, err
		}
		if behind == 0 {
			return res, nil
		}
		if ahead > 0 {
			return res, errors.New("the Store and the Remote have both changed: run squirrel push to combine them")
		}
	}
	if res.Kept, err = keepVanished(env); err != nil {
		return res, err
	}
	if _, err := git(store, "merge", "-q", "--ff-only", "origin/main"); err != nil {
		return res, err
	}
	res.Moved = true
	return res, nil
}

// keepVanished turns every linked Entry that the incoming Manifest no longer
// has into a real copy, before the Store item it points at disappears.
func keepVanished(env Env) ([]Entry, error) {
	current, err := Load(env.Store)
	if err != nil {
		return nil, err
	}
	var incoming Manifest
	if text, err := git(env.Store, "show", "origin/main:"+manifestName); err == nil {
		if _, err := toml.Decode(text, &incoming); err != nil {
			return nil, fmt.Errorf("the Remote's Manifest does not parse: %w", err)
		}
	}
	var kept []Entry
	for _, e := range current.Entries {
		if !isLink(env, e) || slices.ContainsFunc(incoming.Entries, func(i Entry) bool { return i.ID == e.ID }) {
			continue
		}
		if err := unmanage(env, e); err != nil {
			return kept, err
		}
		kept = append(kept, e)
	}
	return kept, nil
}

// ClashError lists the Store paths that both the Store and the Remote changed.
type ClashError struct{ Paths []string }

func (e ClashError) Error() string {
	return "the Store and the Remote changed the same files"
}

// PushResult says what Push did.
type PushResult struct {
	Pulled bool    // the Remote had commits the Store lacked, and the Store took them in
	Pushed bool    // the Store's commits went up to the Remote
	Kept   []Entry // linked Entries the Remote no longer has, now real copies
}

// Push commits the Store's changes, rebases them onto the Remote, and pushes.
// On a clash it aborts the rebase and returns a ClashError.
func Push(env Env) (PushResult, error) {
	var res PushResult
	store := env.Store
	if _, err := requireRemote(store); err != nil {
		return res, err
	}
	if _, err := commitAll(store); err != nil {
		return res, err
	}
	if !hasRef(store, "HEAD") {
		return res, nil // nothing was ever committed
	}
	if _, err := git(store, "fetch", "-q", "origin"); err != nil {
		return res, err
	}
	ahead := "HEAD"
	if hasRef(store, "origin/main") {
		ahead = "origin/main..HEAD"
		behind, err := git(store, "rev-list", "--count", "HEAD..origin/main")
		if err != nil {
			return res, err
		}
		if behind != "0" {
			if res.Kept, err = keepVanished(env); err != nil {
				return res, err
			}
			if _, err := git(store, "rebase", "-q", "origin/main"); err != nil {
				out, _ := git(store, "diff", "--name-only", "-z", "--diff-filter=U")
				if _, abortErr := git(store, "rebase", "--abort"); abortErr != nil {
					return res, abortErr
				}
				clashes := nulPaths(out)
				if clashes == nil {
					return res, err
				}
				return res, ClashError{clashes}
			}
			res.Pulled = true
		}
	}
	if n, err := git(store, "rev-list", "--count", ahead); err != nil || n == "0" {
		return res, err
	}
	if _, err := git(store, "push", "-q", "-u", "origin", "HEAD:main"); err != nil {
		return res, err
	}
	res.Pushed = true
	return res, nil
}

package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RemoteAddResult says what connecting a Remote did.
type RemoteAddResult int

const (
	RemotePushed    RemoteAddResult = iota // the Store's content went up to an empty Remote
	RemoteCloned                           // the Remote's content came down into an empty Store
	RemoteConnected                        // both were empty, so nothing moved
	RemoteUnchanged                        // the Store was already connected to this Remote
)

// RemoteAdd connects the Store to the Remote at url. Whichever side has
// content fills the other. It refuses when both do.
func RemoteAdd(env Env, url string) (RemoteAddResult, error) {
	store := env.Store
	if _, err := os.Stat(url); err == nil {
		// A local path is stored as-is in the Store's git config, so it must not be relative.
		if url, err = filepath.Abs(url); err != nil {
			return 0, err
		}
	}
	_, statErr := os.Stat(filepath.Join(store, ".git"))
	isRepo := statErr == nil
	origin := ""
	if isRepo {
		origin, _ = git(store, "remote", "get-url", "origin") // fails only when there is none
	}
	if origin != "" && origin != url {
		return 0, fmt.Errorf("the Store is already connected to %s", origin)
	}

	refs, err := git("", "ls-remote", url)
	if err != nil {
		return 0, err
	}
	remoteEmpty := refs == ""
	storeEmpty, err := isEmptyDir(store)
	if err != nil {
		return 0, err
	}

	if !remoteEmpty {
		if origin == url {
			return RemoteUnchanged, nil
		}
		if isRepo || !storeEmpty {
			return 0, errors.New("the Remote already has content, and so does the Store: empty one of them first")
		}
		if _, err := git("", "clone", "-q", url, store); err != nil {
			return 0, err
		}
		return RemoteCloned, nil
	}

	// The Remote is empty. A rerun after a failed push lands here too, with
	// origin already set, and pushes again.
	if !isRepo {
		if err := os.MkdirAll(store, 0o755); err != nil {
			return 0, err
		}
		if _, err := git(store, "init", "-q", "-b", "main"); err != nil {
			return 0, err
		}
	}
	if origin == "" {
		if _, err := git(store, "remote", "add", "origin", url); err != nil {
			return 0, err
		}
	}
	if storeEmpty {
		return RemoteConnected, nil
	}
	if _, err := commitAll(store); err != nil {
		return 0, err
	}
	if _, err := git(store, "push", "-q", "-u", "origin", "HEAD:main"); err != nil {
		return 0, err
	}
	return RemotePushed, nil
}

// ErrNoRemote is returned by commands that need a Remote when the Store has none.
var ErrNoRemote = errors.New("the Store has no Remote: run squirrel remote add <url> first")

// DirtyError lists the Store paths that changed since the last commit.
type DirtyError struct{ Paths []string }

func (e DirtyError) Error() string {
	return "the Store has changes that are not pushed"
}

// requireRemote fails with ErrNoRemote unless the Store is a git repo with an origin.
func requireRemote(store string) error {
	if _, err := git(store, "remote", "get-url", "origin"); err != nil {
		if errors.Is(err, errNoGit) {
			return err
		}
		return ErrNoRemote
	}
	return nil
}

// Pull fast-forwards the Store to the Remote. It refuses when the Store has
// changes of its own, and never merges. It reports whether the Store moved.
func Pull(env Env) (bool, error) {
	store := env.Store
	if _, err := os.Stat(filepath.Join(store, ".git")); err != nil {
		return false, ErrNoRemote
	}
	if err := requireRemote(store); err != nil {
		return false, err
	}
	status, err := git(store, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	if status != "" {
		var paths []string
		for _, l := range strings.Split(status, "\n") {
			paths = append(paths, l[3:])
		}
		return false, DirtyError{paths}
	}
	if _, err := git(store, "fetch", "-q", "origin"); err != nil {
		return false, err
	}
	if _, err := git(store, "rev-parse", "--verify", "-q", "origin/main"); err != nil {
		return false, nil // the Remote is still empty
	}
	if _, err := git(store, "rev-parse", "--verify", "-q", "HEAD"); err == nil {
		counts, err := git(store, "rev-list", "--left-right", "--count", "HEAD...origin/main")
		if err != nil {
			return false, err
		}
		var ahead, behind int
		fmt.Sscan(counts, &ahead, &behind)
		if behind == 0 {
			return false, nil
		}
		if ahead > 0 {
			return false, errors.New("the Store and the Remote have both changed: run squirrel push to combine them")
		}
	}
	if _, err := git(store, "merge", "-q", "--ff-only", "origin/main"); err != nil {
		return false, err
	}
	return true, nil
}

// ClashError lists the Store paths that both the Store and the Remote changed.
type ClashError struct{ Paths []string }

func (e ClashError) Error() string {
	return "the Store and the Remote changed the same files"
}

// PushResult says what Push did.
type PushResult struct {
	Pulled bool // the Remote had commits the Store lacked, and the Store took them in
	Pushed bool // the Store's commits went up to the Remote
}

// Push commits the Store's changes, rebases them onto the Remote, and pushes.
// On a clash it aborts the rebase and returns a ClashError.
func Push(env Env) (PushResult, error) {
	var res PushResult
	store := env.Store
	if _, err := os.Stat(filepath.Join(store, ".git")); err != nil {
		return res, ErrNoRemote
	}
	if err := requireRemote(store); err != nil {
		return res, err
	}
	if _, err := commitAll(store); err != nil {
		return res, err
	}
	if _, err := git(store, "rev-parse", "--verify", "-q", "HEAD"); err != nil {
		return res, nil // nothing was ever committed
	}
	if _, err := git(store, "fetch", "-q", "origin"); err != nil {
		return res, err
	}
	ahead := "HEAD"
	if _, err := git(store, "rev-parse", "--verify", "-q", "origin/main"); err == nil {
		ahead = "origin/main..HEAD"
		behind, err := git(store, "rev-list", "--count", "HEAD..origin/main")
		if err != nil {
			return res, err
		}
		if behind != "0" {
			if _, err := git(store, "rebase", "-q", "origin/main"); err != nil {
				clashes, _ := git(store, "diff", "--name-only", "--diff-filter=U")
				if _, abortErr := git(store, "rebase", "--abort"); abortErr != nil {
					return res, abortErr
				}
				if clashes == "" {
					return res, err
				}
				return res, ClashError{strings.Split(clashes, "\n")}
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

// isEmptyDir reports whether dir is missing or holds nothing besides .git.
func isEmptyDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	for _, e := range entries {
		if e.Name() != ".git" {
			return false, nil
		}
	}
	return true, err
}

package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// RemoteAddResult says what connecting a Remote did.
type RemoteAddResult int

const (
	RemotePushed    RemoteAddResult = iota // the Store's content went up to an empty Remote
	RemoteCloned                           // the Remote's content came down into an empty Store
	RemoteConnected                        // both were empty, so nothing moved
	RemoteUnchanged                        // the Store was already connected to this Remote
	RemoteRejoined                         // both have content from the same history, so nothing moved
)

// RemoteAdd connects the Store to the Remote at url. Whichever side has
// content fills the other. It refuses when both do.
func RemoteAdd(env Env, url string) (RemoteAddResult, error) {
	store := env.Store
	url, err := absURL(url)
	if err != nil {
		return 0, err
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

	refs, err := git("", "ls-remote", "--", url)
	if err != nil {
		return 0, err
	}
	if refs != "" && !slices.ContainsFunc(strings.Split(refs, "\n"), func(l string) bool {
		return strings.HasSuffix(l, "\trefs/heads/main")
	}) {
		return 0, errNoMain
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
		if _, noHead := git(store, "rev-parse", "--verify", "-q", "HEAD"); isRepo && noHead == nil &&
			sharesHistory(store, url) == nil {
			// Reconnecting after a remove: the Store kept its history.
			if _, err := git(store, "remote", "add", "origin", url); err != nil {
				return 0, err
			}
			if _, err := git(store, "fetch", "-q", "origin"); err != nil {
				return 0, err
			}
			return RemoteRejoined, nil
		}
		if isRepo || !storeEmpty {
			return 0, errors.New("the Remote already has content, and so does the Store: empty one of them first")
		}
		if _, err := git("", "clone", "-q", "--", url, store); err != nil {
			return 0, err
		}
		if err := lockStore(store); err != nil {
			return 0, err
		}
		return RemoteCloned, nil
	}

	// The Remote is empty. A rerun after a failed push lands here too, with
	// origin already set, and pushes again.
	if !isRepo {
		if err := os.MkdirAll(store, 0o700); err != nil {
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

// absURL makes a local path absolute, since git stores it as-is in the Store's config.
func absURL(url string) (string, error) {
	if strings.HasPrefix(url, "-") {
		return "", errors.New("a Remote URL cannot start with -")
	}
	if _, err := os.Stat(url); err == nil {
		return filepath.Abs(url)
	}
	return url, nil
}

// RemoteRemove disconnects the Store from its Remote and returns the Remote's
// URL. The Store, its history and its Links stay.
func RemoteRemove(env Env) (string, error) {
	url, err := requireRemote(env.Store)
	if err != nil {
		return "", err
	}
	_, err = git(env.Store, "remote", "remove", "origin")
	return url, err
}

// RemoteSetURL points the Store at a new address for the same Remote. It
// returns the old address and the new one, made absolute when it is a local
// path. It refuses a Remote that shares no history with the Store, and keeps
// the old address on any failure.
func RemoteSetURL(env Env, url string) (old, now string, err error) {
	store := env.Store
	if old, err = requireRemote(store); err != nil {
		return "", "", err
	}
	if url, err = absURL(url); err != nil || url == old {
		return old, url, err
	}
	// Check the new address before touching origin, so a failure changes nothing.
	refs, err := git("", "ls-remote", "--", url, "refs/heads/main")
	if err != nil {
		return "", "", err
	}
	if refs == "" {
		if all, err := git("", "ls-remote", "--", url); err != nil {
			return "", "", err
		} else if all != "" {
			return "", "", errNoMain
		}
	}
	_, noHead := git(store, "rev-parse", "--verify", "-q", "HEAD")
	if refs != "" && noHead == nil {
		if err := sharesHistory(store, url); err != nil {
			return "", "", err
		}
	}
	if _, err := git(store, "remote", "set-url", "origin", url); err != nil {
		return "", "", err
	}
	// Drop refs from the old address. Offline is fine: the next push or pull fetches.
	git(store, "fetch", "-q", "--prune", "origin")
	return old, url, nil
}

// errNoMain is returned for a Remote that has branches but no main. squirrel
// only ever syncs main.
var errNoMain = errors.New("the Remote has no main branch: rename its branch to main, then run squirrel remote add again")

// errDifferentStore is returned when a Remote's commits share no history with the Store.
var errDifferentStore = errors.New("that Remote holds a different Store: run squirrel remote remove, then squirrel remote add <url>")

// sharesHistory fetches main from url without touching origin and fails
// unless it shares a commit with the Store's HEAD.
func sharesHistory(store, url string) error {
	if _, err := git(store, "fetch", "-q", "--", url, "main"); err != nil {
		return err
	}
	if _, err := git(store, "merge-base", "HEAD", "FETCH_HEAD"); err != nil {
		return errDifferentStore
	}
	return nil
}

// ErrNoRemote is returned by commands that need a Remote when the Store has none.
var ErrNoRemote = errors.New("the Store has no Remote: run squirrel remote add <url> first")

// DirtyError lists the Store paths that changed since the last commit.
type DirtyError struct{ Paths []string }

func (e DirtyError) Error() string {
	return "the Store has changes that are not pushed"
}

// requireRemote returns the Remote's URL, or ErrNoRemote unless the Store is
// a git repo with an origin. The .git check stops git from finding a repo
// above the Store.
func requireRemote(store string) (string, error) {
	if _, err := os.Stat(filepath.Join(store, ".git")); err != nil {
		return "", ErrNoRemote
	}
	url, err := git(store, "remote", "get-url", "origin")
	if errors.Is(err, errNoGit) {
		return "", err
	}
	if err != nil {
		return "", ErrNoRemote
	}
	return url, nil
}

// HasRemote reports whether the Store is connected to a Remote, without
// touching the network.
func HasRemote(env Env) bool {
	_, err := requireRemote(env.Store)
	return err == nil
}

// dirtyPaths lists the Store paths that changed since the last commit.
func dirtyPaths(store string) ([]string, error) {
	status, err := git(store, "status", "--porcelain")
	if err != nil || status == "" {
		return nil, err
	}
	var paths []string
	for _, l := range strings.Split(status, "\n") {
		paths = append(paths, l[3:])
	}
	return paths, nil
}

// RemoteStatus is how the Store stands against its Remote.
type RemoteStatus struct {
	URL           string
	Ahead, Behind int      // commits only the Store has, and only the Remote has
	Dirty         []string // Store paths changed since the last commit
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
	if _, err := git(store, "fetch", "-q", "origin"); err != nil {
		return s, err
	}
	_, noHead := git(store, "rev-parse", "--verify", "-q", "HEAD")
	_, noRemote := git(store, "rev-parse", "--verify", "-q", "origin/main")
	var counts string
	switch {
	case noHead == nil && noRemote == nil:
		counts, err = git(store, "rev-list", "--left-right", "--count", "HEAD...origin/main")
	case noHead == nil:
		counts, err = git(store, "rev-list", "--count", "HEAD")
	case noRemote == nil:
		counts, err = git(store, "rev-list", "--count", "origin/main")
		counts = "0 " + counts
	}
	fmt.Sscan(counts, &s.Ahead, &s.Behind)
	return s, err
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
	if _, err := git(store, "rev-parse", "--verify", "-q", "origin/main"); err != nil {
		return res, nil // the Remote is still empty
	}
	if _, err := git(store, "rev-parse", "--verify", "-q", "HEAD"); err == nil {
		counts, err := git(store, "rev-list", "--left-right", "--count", "HEAD...origin/main")
		if err != nil {
			return res, err
		}
		var ahead, behind int
		fmt.Sscan(counts, &ahead, &behind)
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
			if res.Kept, err = keepVanished(env); err != nil {
				return res, err
			}
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

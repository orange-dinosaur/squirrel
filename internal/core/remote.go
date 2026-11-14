package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
		if isRepo && hasRef(store, "HEAD") && sharesHistory(store, url) == nil {
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
	if refs != "" && hasRef(store, "HEAD") {
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

// statusPaths lists the paths git status reports with the two-letter code
// want, or every path when want is empty. -z keeps names with spaces or
// non-ASCII letters unquoted.
func statusPaths(store, want string, flags ...string) ([]string, error) {
	out, err := git(store, append([]string{"status", "--porcelain", "-z"}, flags...)...)
	if err != nil {
		return nil, err
	}
	var paths []string
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		if f[0] == 'R' || f[0] == 'C' {
			i++ // the next field is the old path
		}
		if want == "" || f[:2] == want {
			paths = append(paths, f[3:])
		}
	}
	return paths, nil
}

// nulPaths splits git's -z output into paths.
func nulPaths(out string) []string {
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// hasRef reports whether ref names a commit in store.
func hasRef(store, ref string) bool {
	_, err := git(store, "rev-parse", "--verify", "-q", ref)
	return err == nil
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

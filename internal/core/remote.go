package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

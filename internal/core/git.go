package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var errNoGit = errors.New("git is not installed or not on PATH")

// gitEnv is the caller's environment without the variables that point git at
// another repository or inject config. English messages keep error matching
// working, and helper transports such as ext:: are refused.
func gitEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
			"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR", "GIT_PREFIX",
			"GIT_IMPLICIT_WORK_TREE", "GIT_GRAFT_FILE", "GIT_NO_REPLACE_OBJECTS",
			"GIT_REPLACE_REF_BASE", "GIT_SHALLOW_FILE", "GIT_CONFIG",
			"GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT":
			continue
		}
		env = append(env, kv)
	}
	return append(env, "LC_ALL=C", "GIT_ALLOW_PROTOCOL=file:git:http:https:ssh")
}

// git runs git in dir, or in the current directory when dir is empty, and
// returns its stdout without the final newline. A failure carries git's own message.
func git(dir string, args ...string) (string, error) {
	sub := args[0]
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	var out, errOut bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Env = gitEnv()
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if errors.Is(err, exec.ErrNotFound) {
		return "", errNoGit
	}
	if err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s failed: %s", sub, msg)
	}
	return strings.TrimRight(out.String(), "\n"), nil // leading spaces matter in status --porcelain
}

// commitAll commits every change in the Store as "push from <hostname>".
// It reports whether there was anything to commit.
// NestedRepoError names Store items that hold their own .git folder. git would
// record each one as a pointer to that repository instead of its files.
type NestedRepoError struct{ Paths []string }

func (e NestedRepoError) Error() string {
	return "a folder in the Store has its own .git folder, so its files cannot be synced"
}

func commitAll(store string) (bool, error) {
	changes, err := git(store, "status", "--porcelain")
	if err != nil || changes == "" {
		return false, err
	}
	host, err := os.Hostname()
	if err != nil {
		host = "unknown host"
	}
	if _, err := git(store, "add", "-A"); err != nil {
		return false, err
	}
	staged, err := git(store, "ls-files", "-s")
	if err != nil {
		return false, err
	}
	var nested []string
	for l := range strings.Lines(staged) {
		if mode, rest, _ := strings.Cut(l, " "); mode == "160000" {
			_, p, _ := strings.Cut(rest, "\t")
			nested = append(nested, strings.TrimSuffix(p, "\n"))
		}
	}
	if len(nested) > 0 {
		git(store, "reset", "-q")
		return false, NestedRepoError{nested}
	}
	_, err = git(store, "commit", "-q", "-m", "push from "+host)
	if err != nil && strings.Contains(err.Error(), "Please tell me who you are") {
		err = errors.New(`git has no name or email: run git config --global user.name "Your Name" and git config --global user.email "you@example.com"`)
	}
	return err == nil, err
}

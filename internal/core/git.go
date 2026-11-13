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

// git runs git in dir, or in the current directory when dir is empty, and
// returns its stdout without the final newline. A failure carries git's own message.
func git(dir string, args ...string) (string, error) {
	sub := args[0]
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	var out, errOut bytes.Buffer
	cmd := exec.Command("git", args...)
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
	_, err = git(store, "commit", "-q", "-m", "push from "+host)
	if err != nil && strings.Contains(err.Error(), "Please tell me who you are") {
		err = errors.New(`git has no name or email: run git config --global user.name "Your Name" and git config --global user.email "you@example.com"`)
	}
	return err == nil, err
}

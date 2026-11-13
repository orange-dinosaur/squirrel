package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// git runs git in dir, or in the current directory when dir is empty, and
// returns its trimmed stdout. A failure carries git's own message.
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
		return "", errors.New("git is not installed or not on PATH")
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
	return strings.TrimSpace(out.String()), nil
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
	return err == nil, err
}

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orange-dinosaur/squirrel/internal/core"
)

func TestRemoteRows(t *testing.T) {
	cliEnv(t)
	remote := filepath.Join(t.TempDir(), "r.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", remote).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	writeHome(t, ".config/app/config.toml", "a")
	writeHome(t, ".config/app/keys.local", "secret")
	writeHome(t, ".config/app/.gitignore", "*.local\n")
	mustRun(t, "add", "~/.config/app")
	mustRun(t, "remote", "add", remote)

	out, _ := squirrel(t, "remote")
	if !strings.Contains(out, "\nStatus      in sync\nNot synced  ~/.config/app (ignored by git)\n") {
		t.Errorf("ignored file:\n%s", out)
	}

	if err := os.Rename(remote, remote+".gone"); err != nil {
		t.Fatal(err)
	}
	out, errText := squirrel(t, "remote")
	if errText != "" || !strings.Contains(out, "\nStatus      unknown, could not reach the Remote\nReason      git fetch failed: ") {
		t.Errorf("unreachable Remote: err %q\n%s", errText, out)
	}
}

func TestSyncStatus(t *testing.T) {
	for want, s := range map[string]core.RemoteStatus{
		"in sync":                             {},
		"changes to push":                     {Dirty: []string{"squirrel.toml"}},
		"2 commits to push":                   {Ahead: 2, Dirty: []string{"squirrel.toml"}},
		"1 commit to pull":                    {Behind: 1},
		"1 commit to push, 3 commits to pull": {Ahead: 1, Behind: 3},
	} {
		if got := syncStatus(s); got != want {
			t.Errorf("syncStatus(%+v) = %q, want %q", s, got, want)
		}
	}
}

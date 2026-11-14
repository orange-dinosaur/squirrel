package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/orange-dinosaur/squirrel/internal/core"
)

var entryID = regexp.MustCompile(`\b[0-9a-f]{8}\b`)

// squirrel runs one command line and returns its output and error text with
// Entry IDs and this OS's Tag replaced, so the wants read the same on every
// machine. The error text is empty when the command succeeds.
func squirrel(t *testing.T, args ...string) (out, errText string) {
	t.Helper()
	root := newRootCmd()
	root.SetArgs(args)
	var b bytes.Buffer
	root.SetOut(&b)
	root.SetErr(&b)
	root.SilenceErrors, root.SilenceUsage = true, true
	if err := root.Execute(); err != nil {
		errText = err.Error()
	}
	env, _ := core.LoadEnv()
	osTag := regexp.MustCompile(`\b` + env.OS + `\b`)
	clean := func(s string) string { return osTag.ReplaceAllString(entryID.ReplaceAllString(s, "ID"), "OS") }
	return clean(b.String()), clean(errText)
}

// cliEnv gives the test its own HOME and Store, and git a config with an identity.
func cliEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SQUIRREL_STORE", "")
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(cfg, []byte("[user]\n\tname = Test\n\temail = test@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func mustRun(t *testing.T, args ...string) {
	t.Helper()
	if out, err := squirrel(t, args...); err != "" {
		t.Fatalf("%v: %s\n%s", args, err, out)
	}
}

func writeHome(t *testing.T, name, text string) {
	t.Helper()
	path := filepath.Join(os.Getenv("HOME"), name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// conflict leaves an added ~/.zshrc replaced by a real file, so apply meets a Conflict.
func conflict(t *testing.T) {
	writeHome(t, ".zshrc", "old")
	mustRun(t, "add", "~/.zshrc")
	if err := os.Remove(filepath.Join(os.Getenv("HOME"), ".zshrc")); err != nil {
		t.Fatal(err)
	}
	writeHome(t, ".zshrc", "local")
}

func TestCommands(t *testing.T) {
	zshrc := func(t *testing.T) { writeHome(t, ".zshrc", "x") }
	added := func(t *testing.T) { zshrc(t); mustRun(t, "add", "~/.zshrc") }
	tagged := func(t *testing.T) { zshrc(t); mustRun(t, "add", "-t", "work", "~/.zshrc") }
	// An empty bare Remote, made once and only pushed to by "remote add empty".
	remote := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", remote).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for _, c := range []struct {
		name      string
		setup     func(t *testing.T)
		args      []string
		want      string
		wantErr   string
		errSuffix bool // wantErr is only the end of the error, which holds a temp file name
	}{
		{name: "add then status", setup: added, args: []string{"status"},
			want: "STATE   TARGET    TAGS   ID\nLINKED  ~/.zshrc  OS  ID\n"},
		{name: "add twice", setup: added, args: []string{"add", "~/.zshrc"},
			wantErr: "~/.zshrc is already managed by Entry ID"},
		{name: "add missing", args: []string{"add", "~/.nope"}, wantErr: "~/.nope does not exist"},
		{name: "add empty", args: []string{"add", ""}, wantErr: "a path cannot be empty"},
		{name: "remove", setup: added, args: []string{"remove", "~/.zshrc"},
			want: "REMOVED  ~/.zshrc  ID  a real copy replaced the Link\n"},
		{name: "tag", setup: added, args: []string{"tag", "~/.zshrc", "work"},
			want: "TAGGED  ~/.zshrc  OS work\n"},
		{name: "untag", setup: tagged, args: []string{"untag", "~/.zshrc", "work"},
			want: "UNTAGGED  ~/.zshrc  OS\n"},
		{name: "tags rm carried", setup: tagged, args: []string{"tags", "rm", "work"},
			wantErr: "work is on 1 entry"},
		{name: "purge without yes", setup: func(t *testing.T) { added(t); mustRun(t, "remove", "~/.zshrc") },
			args:    []string{"purge"},
			wantErr: "purging asks first: run purge in a terminal, or pass --yes to purge without asking"},
		{name: "purge --yes", setup: func(t *testing.T) { added(t); mustRun(t, "remove", "~/.zshrc") },
			args: []string{"purge", "--yes"}, want: "PURGED  ~/.zshrc  ID\n"},
		{name: "purge read-only Store", setup: func(t *testing.T) {
			added(t)
			mustRun(t, "remove", "~/.zshrc")
			store := filepath.Join(os.Getenv("HOME"), ".squirrel")
			if err := os.Chmod(store, 0o500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(store, 0o700) })
		}, args: []string{"purge", "--yes"}, wantErr: ": permission denied", errSuffix: true},
		{name: "tags rm --untag", setup: tagged, args: []string{"tags", "rm", "work", "--untag"},
			want: "UNTAGGED  ~/.zshrc  work\nDELETED   work\n"},
		{name: "tags", setup: tagged, args: []string{"tags"},
			want: "TAG    ENTRIES\nOS  1\nwork   1\n"}, // both OS Tags are five letters wide
		{name: "remote no Remote", args: []string{"remote"},
			want: "NOTE  " + core.ErrNoRemote.Error() + "\n"},
		{name: "remote add empty", setup: added, args: []string{"remote", "add", remote},
			want: "PUSHED  " + remote + "  the Store is on the Remote\n"},
		{name: "pull no Remote", args: []string{"pull"}, wantErr: core.ErrNoRemote.Error()},
		{name: "push no Remote", args: []string{"push"}, wantErr: core.ErrNoRemote.Error()},
		{name: "apply Conflict", setup: conflict, args: []string{"apply"},
			wantErr: "1 Conflict needs a decision: run squirrel apply in a terminal, or squirrel apply --backup or --overwrite"},
		{name: "apply --backup", setup: conflict, args: []string{"apply", "--backup"},
			want: "LINKED  ~/.zshrc  old one saved as ~/.zshrc.squirrel-backup\n1 linked\n"},
		{name: "apply --overwrite", setup: conflict, args: []string{"apply", "--overwrite"},
			want: "LINKED  ~/.zshrc  old one deleted\n1 linked\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cliEnv(t)
			if c.setup != nil {
				c.setup(t)
			}
			got, gotErr := squirrel(t, c.args...)
			errOK := gotErr == c.wantErr || c.errSuffix && strings.HasSuffix(gotErr, c.wantErr)
			if got != c.want || !errOK {
				t.Errorf("%v:\n got  %q, err %q\n want %q, err %q", c.args, got, gotErr, c.want, c.wantErr)
			}
		})
	}
}

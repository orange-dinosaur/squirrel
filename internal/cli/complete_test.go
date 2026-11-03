package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orange-dinosaur/squirrel/internal/core"
)

func TestCompletion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SQUIRREL_STORE", "")
	env, _ := core.LoadEnv()
	os.WriteFile(filepath.Join(home, ".zshrc"), nil, 0o644)
	os.WriteFile(filepath.Join(home, ".vimrc"), nil, 0o644)
	m := &core.Manifest{}
	core.Add(env, m, "~/.zshrc", []string{"shell"})
	core.Add(env, m, "~/.vimrc", []string{"editor"})

	complete := func(args ...string) string {
		root := newRootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs(append([]string{"__complete"}, args...))
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		return strings.Join(lines[:len(lines)-1], " ")
	}
	for args, want := range map[string]string{
		"tag|":             "~/.zshrc ~/.vimrc",
		"tag|~/.zshrc|":    "editor",
		"untag|~/.zshrc|":  "macos shell",
		"remove|~/.vimrc|": "",
		"apply|shell|":     "editor macos",
	} {
		if got := complete(strings.Split(args, "|")...); got != want {
			t.Errorf("complete %q = %q, want %q", args, got, want)
		}
	}
}

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gitTest isolates git from the developer's config and gives it an identity.
func gitTest(t *testing.T) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	write(t, cfg, "[user]\n\tname = Test\n\temail = test@example.com\n")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// bareRemote makes an empty Remote and returns its URL.
func bareRemote(t *testing.T) string {
	t.Helper()
	url := filepath.Join(t.TempDir(), "remote.git")
	if _, err := git("", "init", "-q", "--bare", "-b", "main", url); err != nil {
		t.Fatal(err)
	}
	return url
}

// pushedRemote makes a Remote holding one file, pushed from another machine.
func pushedRemote(t *testing.T) string {
	t.Helper()
	url := bareRemote(t)
	other := testEnv(t)
	write(t, filepath.Join(other.Store, "squirrel.toml"), "# from the Mac\n")
	if got, err := RemoteAdd(other, url); got != RemotePushed || err != nil {
		t.Fatalf("seeding the Remote: %v, %v", got, err)
	}
	return url
}

func TestRemoteAddPushesStore(t *testing.T) {
	gitTest(t)
	env, url := testEnv(t), bareRemote(t)
	write(t, filepath.Join(env.Store, "squirrel.toml"), "# manifest\n")
	write(t, filepath.Join(env.Store, "00000001-zshrc"), "zsh")

	if got, err := RemoteAdd(env, url); got != RemotePushed || err != nil {
		t.Fatalf("RemoteAdd = %v, %v", got, err)
	}
	files, err := git(url, "ls-tree", "--name-only", "main")
	if err != nil || files != "00000001-zshrc\nsquirrel.toml" {
		t.Errorf("Remote holds %q, %v", files, err)
	}
	if msg, _ := git(url, "log", "-1", "--format=%s", "main"); !strings.HasPrefix(msg, "push from ") {
		t.Errorf("commit message %q", msg)
	}
	if got, err := RemoteAdd(env, url); got != RemoteUnchanged || err != nil {
		t.Errorf("second RemoteAdd = %v, %v, want RemoteUnchanged", got, err)
	}
}

func TestRemoteAddClones(t *testing.T) {
	gitTest(t)
	url := pushedRemote(t)
	for name, prepare := range map[string]func(Env){
		"missing Store": func(Env) {},
		"empty Store":   func(env Env) { os.MkdirAll(env.Store, 0o755) },
	} {
		env := testEnv(t)
		prepare(env)
		if got, err := RemoteAdd(env, url); got != RemoteCloned || err != nil {
			t.Errorf("%s: RemoteAdd = %v, %v", name, got, err)
		}
		if got := readFile(t, filepath.Join(env.Store, "squirrel.toml")); got != "# from the Mac\n" {
			t.Errorf("%s: cloned Manifest %q", name, got)
		}
	}
}

func TestRemoteAddBothEmpty(t *testing.T) {
	gitTest(t)
	env := testEnv(t)
	if got, err := RemoteAdd(env, bareRemote(t)); got != RemoteConnected || err != nil {
		t.Errorf("RemoteAdd = %v, %v", got, err)
	}
}

func TestRemoteAddRefusesBothWithContent(t *testing.T) {
	gitTest(t)
	env := testEnv(t)
	write(t, filepath.Join(env.Store, "squirrel.toml"), "# Linux\n")
	_, err := RemoteAdd(env, pushedRemote(t))
	if err == nil || !strings.Contains(err.Error(), "empty one of them") {
		t.Errorf("err = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(env.Store, ".git")); statErr == nil {
		t.Error("refusing left a .git behind")
	}
}

func TestRemoteAddReusesRepo(t *testing.T) {
	gitTest(t)
	env, url := testEnv(t), bareRemote(t)
	write(t, filepath.Join(env.Store, "squirrel.toml"), "# mine\n")
	if _, err := git(env.Store, "init", "-q", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := commitAll(env.Store); err != nil {
		t.Fatal(err)
	}
	if got, err := RemoteAdd(env, url); got != RemotePushed || err != nil {
		t.Fatalf("RemoteAdd = %v, %v", got, err)
	}
	if n, _ := git(url, "rev-list", "--count", "main"); n != "1" {
		t.Errorf("Remote has %s commits, want the user's 1", n)
	}
}

func TestGitMissing(t *testing.T) {
	t.Setenv("PATH", "")
	if _, err := git("", "version"); err == nil || err.Error() != "git is not installed or not on PATH" {
		t.Errorf("err = %v", err)
	}
}

func TestRemoteAddRefusesOtherURL(t *testing.T) {
	gitTest(t)
	env, first := testEnv(t), bareRemote(t)
	if _, err := RemoteAdd(env, first); err != nil {
		t.Fatal(err)
	}
	_, err := RemoteAdd(env, bareRemote(t))
	if err == nil || !strings.Contains(err.Error(), "already connected to "+first) {
		t.Errorf("err = %v", err)
	}
}

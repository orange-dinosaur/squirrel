package core

import (
	"os"
	"path/filepath"
	"slices"
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

func TestGitIgnoresInheritedGitDir(t *testing.T) {
	gitTest(t)
	env, url := testEnv(t), bareRemote(t)
	decoy := bareRemote(t)
	t.Setenv("GIT_DIR", decoy)
	write(t, filepath.Join(env.Store, "squirrel.toml"), "# manifest\n")
	if _, err := RemoteAdd(env, url); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(env.Store, "00000001-zshrc"), "zsh")
	if _, err := Push(env); err != nil {
		t.Fatal(err)
	}
	if out, err := git(decoy, "config", "--get", "remote.origin.url"); err == nil {
		t.Errorf("decoy has origin %q", out)
	}
	if out, err := git(decoy, "rev-parse", "--verify", "-q", "HEAD"); err == nil {
		t.Errorf("decoy has commit %q", out)
	}
	if files, err := git(url, "ls-tree", "--name-only", "main"); err != nil || files != "00000001-zshrc\nsquirrel.toml" {
		t.Errorf("Remote holds %q, %v", files, err)
	}
}

func TestRemoteAddRefusesExtTransport(t *testing.T) {
	gitTest(t)
	env := testEnv(t)
	marker := filepath.Join(t.TempDir(), "pwned")
	if _, err := RemoteAdd(env, "ext::sh -c 'touch "+marker+"'"); err == nil {
		t.Error("RemoteAdd accepted an ext:: URL")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the ext:: command ran")
	}
}

func TestRemoteRefusesDashURL(t *testing.T) {
	gitTest(t)
	marker := filepath.Join(t.TempDir(), "pwned")
	url := "--upload-pack=touch " + marker
	want := "a Remote URL cannot start with -"
	if _, err := RemoteAdd(testEnv(t), url); err == nil || err.Error() != want {
		t.Errorf("RemoteAdd: err = %v", err)
	}
	env := testEnv(t)
	if _, err := RemoteAdd(env, bareRemote(t)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RemoteSetURL(env, url); err == nil || err.Error() != want {
		t.Errorf("RemoteSetURL: err = %v", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the --upload-pack command ran")
	}
}

func TestRemoteRefusesNoMainBranch(t *testing.T) {
	gitTest(t)
	url := filepath.Join(t.TempDir(), "m.git")
	seed := filepath.Join(t.TempDir(), "seed")
	write(t, filepath.Join(seed, "squirrel.toml"), "# m\n")
	for _, args := range [][]string{
		{"init", "-q", "--bare", "-b", "master", url},
		{"-C", seed, "init", "-q", "-b", "master"},
		{"-C", seed, "add", "-A"},
		{"-C", seed, "commit", "-qm", "seed"},
		{"-C", seed, "push", "-q", url, "master"},
	} {
		if _, err := git("", args...); err != nil {
			t.Fatal(err)
		}
	}
	want := "the Remote has no main branch: rename its branch to main, then run squirrel remote add again"

	env := testEnv(t)
	if _, err := RemoteAdd(env, url); err == nil || err.Error() != want {
		t.Errorf("RemoteAdd: err = %v", err)
	}
	if _, err := os.Stat(env.Store); err == nil {
		t.Error("RemoteAdd created a Store")
	}

	_, linux := twoMachines(t)
	old, _ := git(linux.Store, "remote", "get-url", "origin")
	if _, _, err := RemoteSetURL(linux, url); err == nil || err.Error() != want {
		t.Errorf("RemoteSetURL: err = %v", err)
	}
	if now, _ := git(linux.Store, "remote", "get-url", "origin"); now != old {
		t.Errorf("origin changed to %s", now)
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
		if info, err := os.Stat(env.Store); err != nil || info.Mode().Perm() != 0o700 {
			t.Errorf("%s: Store mode %v, %v, want 0700", name, info.Mode().Perm(), err)
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

func TestDirtyPathsUnquoted(t *testing.T) {
	gitTest(t)
	env := testEnv(t)
	write(t, filepath.Join(env.Store, "squirrel.toml"), "# manifest\n")
	if _, err := RemoteAdd(env, bareRemote(t)); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(env.Store, "00000001-my app.conf"), "x")
	if got, err := dirtyPaths(env.Store); err != nil || !slices.Equal(got, []string{"00000001-my app.conf"}) {
		t.Errorf("dirtyPaths = %q, %v", got, err)
	}
}

func TestRemoteRemove(t *testing.T) {
	gitTest(t)
	if _, err := RemoteRemove(testEnv(t)); err != ErrNoRemote {
		t.Errorf("no Remote: err = %v", err)
	}
	_, linux := twoMachines(t)
	want, _ := git(linux.Store, "remote", "get-url", "origin")
	if url, err := RemoteRemove(linux); url != want || err != nil {
		t.Fatalf("RemoteRemove = %q, %v", url, err)
	}
	if HasRemote(linux) {
		t.Error("still connected")
	}
	if got := readFile(t, filepath.Join(linux.Store, "squirrel.toml")); got != "# v1\n" {
		t.Errorf("Store after remove %q", got)
	}
}

func TestRemoteSetURL(t *testing.T) {
	gitTest(t)
	if _, _, err := RemoteSetURL(testEnv(t), bareRemote(t)); err != ErrNoRemote {
		t.Errorf("no Remote: err = %v", err)
	}
	_, linux := twoMachines(t)
	old, _ := git(linux.Store, "remote", "get-url", "origin")
	origin := func() string { u, _ := git(linux.Store, "remote", "get-url", "origin"); return u }

	_, _, err := RemoteSetURL(linux, pushedRemote(t))
	if err == nil || !strings.Contains(err.Error(), "different Store") || origin() != old {
		t.Errorf("unrelated: err = %v, origin %s", err, origin())
	}

	moved := filepath.Join(t.TempDir(), "moved.git")
	if _, err := git("", "clone", "-q", "--bare", old, moved); err != nil {
		t.Fatal(err)
	}
	if was, now, err := RemoteSetURL(linux, moved); was != old || now != moved || err != nil || origin() != moved {
		t.Errorf("moved: %q, %q, %v, origin %s", was, now, err, origin())
	}

	empty := bareRemote(t)
	if _, _, err := RemoteSetURL(linux, empty); err != nil || origin() != empty {
		t.Fatalf("empty: %v, origin %s", err, origin())
	}
	if got, err := Push(linux); !got.Pushed || err != nil {
		t.Errorf("push to the new empty Remote: %+v, %v", got, err)
	}
}

func TestRemoteSetURLNoCommit(t *testing.T) {
	gitTest(t)
	env := testEnv(t)
	if _, err := RemoteAdd(env, bareRemote(t)); err != nil {
		t.Fatal(err)
	}
	url := pushedRemote(t)
	if _, now, err := RemoteSetURL(env, url); now != url || err != nil {
		t.Fatalf("RemoteSetURL = %q, %v", now, err)
	}
	if got, _ := git(env.Store, "remote", "get-url", "origin"); got != url {
		t.Errorf("origin = %q", got)
	}
}

func TestRemoteAddRejoins(t *testing.T) {
	gitTest(t)
	mac, linux := twoMachines(t)
	url, _ := git(linux.Store, "remote", "get-url", "origin")
	if _, err := RemoteRemove(linux); err != nil {
		t.Fatal(err)
	}
	pushByHand(t, mac, "00000003-gitconfig", "Mac")
	if got, err := RemoteAdd(linux, url); got != RemoteRejoined || err != nil {
		t.Fatalf("RemoteAdd = %v, %v", got, err)
	}
	if res, err := Pull(linux); !res.Moved || err != nil {
		t.Errorf("catching up: Pull = %+v, %v", res, err)
	}

	if _, err := RemoteRemove(linux); err != nil {
		t.Fatal(err)
	}
	_, err := RemoteAdd(linux, pushedRemote(t))
	if err == nil || !strings.Contains(err.Error(), "empty one of them") || HasRemote(linux) {
		t.Errorf("unrelated: err = %v, connected %v", err, HasRemote(linux))
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

package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

// twoMachines returns a Mac that pushed a Store to a Remote and a Linux box
// that cloned it.
func twoMachines(t *testing.T) (mac, linux Env) {
	t.Helper()
	mac, linux = testEnv(t), testEnv(t)
	url := bareRemote(t)
	write(t, filepath.Join(mac.Store, "squirrel.toml"), "# v1\n")
	if _, err := RemoteAdd(mac, url); err != nil {
		t.Fatal(err)
	}
	if _, err := RemoteAdd(linux, url); err != nil {
		t.Fatal(err)
	}
	return mac, linux
}

// pushByHand commits and pushes one changed file, as a later step's push will.
func pushByHand(t *testing.T, env Env, name, text string) {
	t.Helper()
	write(t, filepath.Join(env.Store, name), text)
	if _, err := commitAll(env.Store); err != nil {
		t.Fatal(err)
	}
	if _, err := git(env.Store, "push", "-q", "origin", "HEAD:main"); err != nil {
		t.Fatal(err)
	}
}

func TestPullNoRemote(t *testing.T) {
	gitTest(t)
	env := testEnv(t)
	if _, err := Pull(env); err != ErrNoRemote {
		t.Errorf("no Store: err = %v", err)
	}
	if _, err := git("", "init", "-q", env.Store); err != nil {
		t.Fatal(err)
	}
	if _, err := Pull(env); err != ErrNoRemote {
		t.Errorf("repo without origin: err = %v", err)
	}
}

func TestPullFastForwards(t *testing.T) {
	gitTest(t)
	mac, linux := twoMachines(t)
	if res, err := Pull(linux); res.Moved || err != nil {
		t.Errorf("up to date: Pull = %+v, %v", res, err)
	}
	pushByHand(t, mac, "squirrel.toml", "# v2\n")
	if res, err := Pull(linux); !res.Moved || err != nil {
		t.Fatalf("behind: Pull = %+v, %v", res, err)
	}
	if got := readFile(t, filepath.Join(linux.Store, "squirrel.toml")); got != "# v2\n" {
		t.Errorf("Manifest after Pull %q", got)
	}
}

func TestPullKeepsVanishedEntry(t *testing.T) {
	gitTest(t)
	mac, linux := twoMachines(t)
	e := Entry{ID: "aaaaaaaa", Target: "~/.zshrc"}
	write(t, mac.StorePath(e), "conf")
	pushByHand(t, mac, "squirrel.toml", "[[entry]]\nid = \"aaaaaaaa\"\ntarget = \"~/.zshrc\"\n")
	if _, err := Pull(linux); err != nil {
		t.Fatal(err)
	}
	target := linux.Expand(e.Target)
	if err := os.Symlink(linux.StorePath(e), target); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(mac.StorePath(e)); err != nil {
		t.Fatal(err)
	}
	pushByHand(t, mac, "squirrel.toml", "# purged\n")
	res, err := Pull(linux)
	if err != nil || !res.Moved || !reflect.DeepEqual(res.Kept, []Entry{e}) {
		t.Fatalf("Pull = %+v, %v", res, err)
	}
	if info, err := os.Lstat(target); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("target is not a regular file: %v, %v", info, err)
	}
	if got := readFile(t, target); got != "conf" {
		t.Errorf("target holds %q", got)
	}
}

func TestPullRefusesDirty(t *testing.T) {
	gitTest(t)
	_, linux := twoMachines(t)
	write(t, filepath.Join(linux.Store, "squirrel.toml"), "# edited on Linux\n")
	write(t, filepath.Join(linux.Store, "00000002-vimrc"), "new")
	_, err := Pull(linux)
	d, ok := err.(DirtyError)
	if !ok || strings.Join(d.Paths, ",") != "squirrel.toml,00000002-vimrc" {
		t.Errorf("err = %#v", err)
	}
}

func TestPullRefusesDiverged(t *testing.T) {
	gitTest(t)
	mac, linux := twoMachines(t)
	pushByHand(t, mac, "squirrel.toml", "# Mac\n")
	write(t, filepath.Join(linux.Store, "00000002-vimrc"), "Linux")
	if _, err := commitAll(linux.Store); err != nil {
		t.Fatal(err)
	}
	_, err := Pull(linux)
	if err == nil || !strings.Contains(err.Error(), "both changed") {
		t.Errorf("err = %v", err)
	}
	if got := readFile(t, filepath.Join(linux.Store, "squirrel.toml")); got != "# v1\n" {
		t.Errorf("refused Pull changed the Store: %q", got)
	}
}

func TestPullIntoEmptyConnectedStore(t *testing.T) {
	gitTest(t)
	url := bareRemote(t)
	linux, mac := testEnv(t), testEnv(t)
	if got, err := RemoteAdd(linux, url); got != RemoteConnected || err != nil {
		t.Fatalf("connect: %v, %v", got, err)
	}
	if res, err := Pull(linux); res.Moved || err != nil {
		t.Errorf("Remote still empty: Pull = %+v, %v", res, err)
	}
	write(t, filepath.Join(mac.Store, "squirrel.toml"), "# Mac\n")
	if _, err := RemoteAdd(mac, url); err != nil {
		t.Fatal(err)
	}
	if res, err := Pull(linux); !res.Moved || err != nil {
		t.Fatalf("Pull = %+v, %v", res, err)
	}
	if got := readFile(t, filepath.Join(linux.Store, "squirrel.toml")); got != "# Mac\n" {
		t.Errorf("Manifest after Pull %q", got)
	}
}

func TestPushNoRemote(t *testing.T) {
	gitTest(t)
	if _, err := Push(testEnv(t)); err != ErrNoRemote {
		t.Errorf("err = %v", err)
	}
}

func TestPushNothingToPush(t *testing.T) {
	gitTest(t)
	_, linux := twoMachines(t)
	if got, err := Push(linux); !reflect.DeepEqual(got, PushResult{}) || err != nil {
		t.Errorf("Push = %+v, %v", got, err)
	}
}

func TestPushLocalChange(t *testing.T) {
	gitTest(t)
	mac, linux := twoMachines(t)
	write(t, filepath.Join(linux.Store, "00000002-vimrc"), "Linux")
	if got, err := Push(linux); !reflect.DeepEqual(got, PushResult{Pushed: true}) || err != nil {
		t.Fatalf("Push = %+v, %v", got, err)
	}
	if res, err := Pull(mac); !res.Moved || err != nil {
		t.Fatalf("Pull = %+v, %v", res, err)
	}
	if got := readFile(t, filepath.Join(mac.Store, "00000002-vimrc")); got != "Linux" {
		t.Errorf("Mac got %q", got)
	}
}

func TestPushRebasesOntoRemote(t *testing.T) {
	gitTest(t)
	mac, linux := twoMachines(t)
	pushByHand(t, mac, "00000003-gitconfig", "Mac")
	write(t, filepath.Join(linux.Store, "00000002-vimrc"), "Linux")
	if got, err := Push(linux); !reflect.DeepEqual(got, PushResult{Pulled: true, Pushed: true}) || err != nil {
		t.Fatalf("Push = %+v, %v", got, err)
	}
	if got := readFile(t, filepath.Join(linux.Store, "00000003-gitconfig")); got != "Mac" {
		t.Errorf("Linux got %q", got)
	}
	files, _ := git(linux.Store, "ls-tree", "--name-only", "origin/main")
	if files != "00000002-vimrc\n00000003-gitconfig\nsquirrel.toml" {
		t.Errorf("Remote holds %q", files)
	}
}

func TestPushClash(t *testing.T) {
	gitTest(t)
	mac, linux := twoMachines(t)
	pushByHand(t, mac, "squirrel.toml", "# Mac\n")
	write(t, filepath.Join(linux.Store, "squirrel.toml"), "# Linux\n")
	got, err := Push(linux)
	c, ok := err.(ClashError)
	if !ok || strings.Join(c.Paths, ",") != "squirrel.toml" || !reflect.DeepEqual(got, PushResult{}) {
		t.Fatalf("Push = %+v, %#v", got, err)
	}
	if got := readFile(t, filepath.Join(linux.Store, "squirrel.toml")); got != "# Linux\n" {
		t.Errorf("clash changed the Store: %q", got)
	}
	if _, err := os.Stat(filepath.Join(linux.Store, ".git", "rebase-merge")); err == nil {
		t.Error("rebase left in progress")
	}
}

func TestPushFirstToEmptyRemote(t *testing.T) {
	gitTest(t)
	env, url := testEnv(t), bareRemote(t)
	if got, err := RemoteAdd(env, url); got != RemoteConnected || err != nil {
		t.Fatalf("connect: %v, %v", got, err)
	}
	write(t, filepath.Join(env.Store, "squirrel.toml"), "# first\n")
	if got, err := Push(env); !reflect.DeepEqual(got, PushResult{Pushed: true}) || err != nil {
		t.Fatalf("Push = %+v, %v", got, err)
	}
	if files, _ := git(url, "ls-tree", "--name-only", "main"); files != "squirrel.toml" {
		t.Errorf("Remote holds %q", files)
	}
}

func TestPushRefusesNestedRepo(t *testing.T) {
	gitTest(t)
	env, url := testEnv(t), bareRemote(t)
	write(t, filepath.Join(env.Store, "squirrel.toml"), "# manifest\n")
	if _, err := RemoteAdd(env, url); err != nil {
		t.Fatal(err)
	}
	nvim := filepath.Join(env.Store, "00000001-nvim")
	write(t, filepath.Join(nvim, "init.lua"), "set nu")
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-qm", "x"}} {
		if _, err := git(nvim, args...); err != nil {
			t.Fatal(err)
		}
	}

	_, err := Push(env)
	n, ok := errors.AsType[NestedRepoError](err)
	if !ok || !slices.Equal(n.Paths, []string{"00000001-nvim"}) {
		t.Fatalf("Push = %v, want NestedRepoError for 00000001-nvim", err)
	}
	if tree, _ := git(url, "ls-tree", "-r", "main"); strings.Contains(tree, "160000") {
		t.Errorf("Remote has a gitlink:\n%s", tree)
	}
	if status, _ := git(env.Store, "status", "--porcelain"); status != "?? 00000001-nvim/" {
		t.Errorf("Store status %q, want the folder untracked", status)
	}
}

func TestPushNoIdentity(t *testing.T) {
	gitTest(t)
	_, linux := twoMachines(t)
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	write(t, cfg, "[user]\n\tuseConfigOnly = true\n")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("LC_ALL", "de_AT.utf8")
	t.Setenv("LANGUAGE", "de")
	write(t, filepath.Join(linux.Store, "00000002-vimrc"), "Linux")
	_, err := Push(linux)
	if err == nil || !strings.HasPrefix(err.Error(), "git has no name or email: run git config --global user.name") {
		t.Errorf("err = %v", err)
	}
}

func TestRemoteStateNoRemote(t *testing.T) {
	gitTest(t)
	env := testEnv(t)
	if HasRemote(env) {
		t.Error("HasRemote with no Store")
	}
	if _, err := RemoteState(env); err != ErrNoRemote {
		t.Errorf("err = %v", err)
	}
}

func TestRemoteState(t *testing.T) {
	gitTest(t)
	mac, linux := twoMachines(t)
	url, _ := git(linux.Store, "remote", "get-url", "origin")
	check := func(name string, want RemoteStatus) {
		t.Helper()
		want.URL = url
		got, err := RemoteState(linux)
		if err != nil || got.URL != want.URL || got.Ahead != want.Ahead || got.Behind != want.Behind ||
			strings.Join(got.Dirty, ",") != strings.Join(want.Dirty, ",") {
			t.Errorf("%s: RemoteState = %+v, %v, want %+v", name, got, err, want)
		}
	}
	if !HasRemote(linux) {
		t.Fatal("HasRemote = false")
	}
	check("in sync", RemoteStatus{})
	pushByHand(t, mac, "00000003-gitconfig", "Mac")
	check("behind", RemoteStatus{Behind: 1})
	write(t, filepath.Join(linux.Store, "00000002-vimrc"), "Linux")
	check("behind and dirty", RemoteStatus{Behind: 1, Dirty: []string{"00000002-vimrc"}})
	if _, err := commitAll(linux.Store); err != nil {
		t.Fatal(err)
	}
	check("ahead and behind", RemoteStatus{Ahead: 1, Behind: 1})
}

func TestRemoteStateEmptyRemote(t *testing.T) {
	gitTest(t)
	env, url := testEnv(t), bareRemote(t)
	if _, err := RemoteAdd(env, url); err != nil {
		t.Fatal(err)
	}
	if got, err := RemoteState(env); got.Ahead != 0 || got.Behind != 0 || err != nil {
		t.Errorf("both empty: %+v, %v", got, err)
	}
	write(t, filepath.Join(env.Store, "squirrel.toml"), "# x\n")
	if _, err := commitAll(env.Store); err != nil {
		t.Fatal(err)
	}
	if got, err := RemoteState(env); got.Ahead != 1 || err != nil {
		t.Errorf("committed, not pushed: %+v, %v", got, err)
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

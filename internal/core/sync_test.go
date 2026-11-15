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

func TestPullRefusesInvalidRemoteManifest(t *testing.T) {
	gitTest(t)
	mac, linux := twoMachines(t)
	before, err := git(linux.Store, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	pushByHand(t, mac, "squirrel.toml", "[[entry]]\nid = \"aaaaaaaa\"\ntarget = \"~\"\n")
	res, err := Pull(linux)
	if err == nil || !strings.HasPrefix(err.Error(), "the Remote's Manifest is not valid, so the Store stays as it is: ") || res.Moved {
		t.Fatalf("Pull = %+v, %v", res, err)
	}
	if after, _ := git(linux.Store, "rev-parse", "HEAD"); after != before {
		t.Errorf("Store HEAD moved from %s to %s", before, after)
	}
	if _, err := Load(linux.Store); err != nil {
		t.Errorf("local Manifest no longer loads: %v", err)
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

func TestPushRebaseFailsWithoutClash(t *testing.T) {
	gitTest(t)
	mac, linux := twoMachines(t)
	pushByHand(t, mac, "00000003-gitconfig", "Mac")
	write(t, filepath.Join(linux.Store, "00000002-vimrc"), "Linux")
	hook := filepath.Join(linux.Store, ".git", "hooks", "pre-rebase")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Push(linux)
	if _, clash := err.(ClashError); err == nil || clash || got.Pushed {
		t.Fatalf("Push = %+v, %#v", got, err)
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

func TestRemoteStateIgnored(t *testing.T) {
	gitTest(t)
	env := testEnv(t)
	write(t, filepath.Join(env.Store, "squirrel.toml"), "# manifest\n")
	write(t, filepath.Join(env.Store, "00000001-app/.gitignore"), "*.local\n")
	write(t, filepath.Join(env.Store, "00000001-app/keys.local"), "secret")
	if _, err := RemoteAdd(env, bareRemote(t)); err != nil {
		t.Fatal(err)
	}
	s, err := RemoteState(env)
	if err != nil || !slices.Equal(s.Ignored, []string{"00000001-app/keys.local"}) || len(s.Dirty) != 0 {
		t.Errorf("RemoteState = %+v, %v", s, err)
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

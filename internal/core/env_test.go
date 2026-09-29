package core

import (
	"path/filepath"
	"testing"
)

func TestLoadEnvStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Setenv("SQUIRREL_STORE", "")
	env, err := LoadEnv()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".squirrel"); env.Store != want {
		t.Errorf("default Store = %q, want %q", env.Store, want)
	}

	t.Setenv("SQUIRREL_STORE", "/elsewhere/store")
	if env, _ = LoadEnv(); env.Store != "/elsewhere/store" {
		t.Errorf("SQUIRREL_STORE Store = %q", env.Store)
	}
}

func TestExpandContract(t *testing.T) {
	env := Env{Home: "/home/me"}
	for target, path := range map[string]string{
		"~":              "/home/me",
		"~/.zshrc":       "/home/me/.zshrc",
		"~/.config/nvim": "/home/me/.config/nvim",
	} {
		if got := env.Expand(target); got != path {
			t.Errorf("Expand(%q) = %q, want %q", target, got, path)
		}
		if got, ok := env.Contract(path); !ok || got != target {
			t.Errorf("Contract(%q) = %q, %v, want %q", path, got, ok, target)
		}
	}
	for _, outside := range []string{"/home", "/home/meh/.zshrc", "/etc/hosts"} {
		if got, ok := env.Contract(outside); ok {
			t.Errorf("Contract(%q) = %q, want outside Home", outside, got)
		}
	}
}

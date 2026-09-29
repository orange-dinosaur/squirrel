package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Env is the machine squirrel runs on: its home directory, Store and OS tag.
type Env struct {
	Home  string
	Store string
	OS    string
}

// LoadEnv reads HOME and SQUIRREL_STORE. The Store defaults to ~/.squirrel.
func LoadEnv() (Env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Env{}, err
	}
	store := filepath.Join(home, ".squirrel")
	if s := os.Getenv("SQUIRREL_STORE"); s != "" {
		if store, err = filepath.Abs(s); err != nil {
			return Env{}, err
		}
	}
	return Env{Home: filepath.Clean(home), Store: store, OS: osTag(runtime.GOOS)}, nil
}

func osTag(goos string) string {
	if goos == "darwin" {
		return "macos"
	}
	return goos
}

// Expand turns a Manifest target such as ~/.zshrc into an absolute path.
func (e Env) Expand(target string) string {
	if target == "~" {
		return e.Home
	}
	if rest, ok := strings.CutPrefix(target, "~/"); ok {
		return filepath.Join(e.Home, rest)
	}
	return target
}

// Contract turns an absolute path into its ~ form. It reports false for a
// path outside Home.
func (e Env) Contract(path string) (string, bool) {
	rel, err := filepath.Rel(e.Home, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	if rel == "." {
		return "~", true
	}
	return "~/" + rel, true
}

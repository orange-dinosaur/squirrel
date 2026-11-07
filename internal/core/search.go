package core

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// Candidate is a path Add's search offers.
type Candidate struct {
	Target string // ~/ form
	Dir    bool
}

const searchDepth = 4 // a hidden item in home is depth 1

var searchSkip = map[string]bool{".git": true, "node_modules": true, ".cache": true, ".Trash": true}

// SearchCandidates lists the hidden items in home and what lies inside them,
// down to searchDepth, keeping only what Add would accept. It never descends
// into searchSkip folders, the Store, or symlinks. Each directory comes right
// before what is inside it.
func SearchCandidates(env Env, m *Manifest) []Candidate {
	var out []Candidate
	filepath.WalkDir(env.Home, func(path string, d fs.DirEntry, err error) error {
		if path == env.Home {
			return err // an unreadable home ends the walk
		}
		rel, _ := filepath.Rel(env.Home, path)
		depth := strings.Count(rel, string(filepath.Separator)) + 1
		skip := (depth == 1 && !strings.HasPrefix(d.Name(), ".")) || searchSkip[d.Name()] || path == env.Store
		if skip || err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := "~/" + filepath.ToSlash(rel)
		if CheckAdd(env, m, target) == nil {
			out = append(out, Candidate{target, d.IsDir()})
		}
		if d.IsDir() && depth == searchDepth {
			return filepath.SkipDir
		}
		return nil
	})
	return out
}

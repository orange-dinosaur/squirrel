package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// OSTags are the reserved Tags that limit an Entry to an OS.
var OSTags = []string{"macos", "linux"}

var tagName = regexp.MustCompile(`^[a-z0-9-]+$`)

// ValidTag reports whether t is a legal Tag name.
func ValidTag(t string) bool {
	return tagName.MatchString(t)
}

// CheckTags refuses the first Tag name that is not legal.
func CheckTags(tags []string) error {
	for _, t := range tags {
		if !ValidTag(t) {
			return fmt.Errorf("tag %q must use only a-z, 0-9 and -", t)
		}
	}
	return nil
}

// AppliesOn reports whether the Entry is meant for os. An Entry with no OS tag
// applies everywhere.
func (e Entry) AppliesOn(os string) bool {
	hasOSTag := false
	for _, t := range e.Tags {
		if slices.Contains(OSTags, t) {
			if t == os {
				return true
			}
			hasOSTag = true
		}
	}
	return !hasOSTag
}

// HasAnyTag reports whether the Entry carries at least one of tags. No tags
// matches every Entry.
func (e Entry) HasAnyTag(tags []string) bool {
	return len(tags) == 0 || slices.ContainsFunc(tags, func(t string) bool {
		return slices.Contains(e.Tags, t)
	})
}

// Find resolves a command argument to the index of one Entry. An Entry ID
// reaches any Entry. A path reaches the non-removed Entry for that target
// that applies on this OS; relative paths resolve against the working directory.
func (m *Manifest) Find(env Env, arg string) (int, error) {
	if i := slices.IndexFunc(m.Entries, func(e Entry) bool { return e.ID == arg }); i >= 0 {
		return i, nil
	}
	target, err := env.targetOf(arg)
	if err != nil {
		return -1, bareMiss(arg, err)
	}
	i := slices.IndexFunc(m.Entries, func(e Entry) bool {
		return e.Target == target && !e.Removed && e.AppliesOn(env.OS)
	})
	if i < 0 {
		return -1, bareMiss(arg, fmt.Errorf("no Entry for %s on %s", target, env.OS))
	}
	return i, nil
}

// bareMiss explains a failed lookup. A bare word such as "fdedb1e" was most
// likely a mistyped ID, so it gets a message that says so instead of err.
func bareMiss(arg string, err error) error {
	if arg != "" && !strings.Contains(arg, "/") && !strings.HasPrefix(arg, "~") && !strings.HasPrefix(arg, ".") {
		return fmt.Errorf("no Entry with ID or path %s", arg)
	}
	return err
}

// targetOf turns a path argument into its ~ form, resolving relative paths
// against the working directory.
func (e Env) targetOf(arg string) (string, error) {
	if arg == "" {
		return "", errors.New("a path cannot be empty")
	}
	path, err := filepath.Abs(e.Expand(arg))
	if err != nil {
		return "", err
	}
	target, ok := e.Contract(path)
	if !ok {
		return "", fmt.Errorf("%s is outside HOME", arg)
	}
	return target, nil
}

// Clash returns the ID of another non-removed Entry for the same target that
// would apply on an OS where e applies too. Entries sharing e's ID are e itself.
func (m *Manifest) Clash(e Entry) (string, bool) {
	for _, o := range m.Entries {
		if o.ID == e.ID || o.Removed || o.Target != e.Target {
			continue
		}
		for _, os := range OSTags {
			if e.AppliesOn(os) && o.AppliesOn(os) {
				return o.ID, true
			}
		}
	}
	return "", false
}

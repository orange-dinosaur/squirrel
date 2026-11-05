package core

import (
	"fmt"
	"slices"
	"strings"
)

// Tag adds tags to the Entry for arg and saves the Manifest.
func Tag(env Env, m *Manifest, arg string, tags []string) (Entry, error) {
	return retag(env, m, arg, tags, func(old []string) []string {
		return append(slices.Clone(old), tags...)
	})
}

// Untag drops tags from the Entry for arg and saves the Manifest. Tags the
// Entry doesn't have are ignored.
func Untag(env Env, m *Manifest, arg string, tags []string) (Entry, error) {
	return retag(env, m, arg, tags, func(old []string) []string {
		return slices.DeleteFunc(slices.Clone(old), func(t string) bool { return slices.Contains(tags, t) })
	})
}

// SetTags replaces the Entry's tags with tags and saves the Manifest, under
// the same checks as Tag and Untag.
func SetTags(env Env, m *Manifest, arg string, tags []string) (Entry, error) {
	return retag(env, m, arg, tags, func([]string) []string { return slices.Clone(tags) })
}

func retag(env Env, m *Manifest, arg string, tags []string, change func([]string) []string) (Entry, error) {
	for _, t := range tags {
		if !ValidTag(t) {
			return Entry{}, fmt.Errorf("tag %q must use only a-z, 0-9 and -", t)
		}
	}
	i, err := m.Find(env, arg)
	if err != nil {
		return Entry{}, err
	}
	e := m.Entries[i]
	e.Tags = change(e.Tags)
	slices.Sort(e.Tags)
	e.Tags = slices.Compact(e.Tags)
	if !e.Removed && !e.AppliesOn(env.OS) && isLink(env, e) {
		return Entry{}, fmt.Errorf("%s is linked here, so it must keep applying on %s: remove it first, or change its tags on a machine where it isn't linked", e.Target, env.OS)
	}
	if id, ok := m.Clash(e); ok && !e.Removed {
		return Entry{}, fmt.Errorf("%s would then apply on the same OS as Entry %s", e.Target, id)
	}
	old := m.Entries[i]
	m.Entries[i] = e
	if err := m.Save(env.Store); err != nil {
		m.Entries[i] = old
		return Entry{}, err
	}
	return e, nil
}

type TagCount struct {
	Tag   string
	Count int // non-removed Entries carrying the Tag
}

// Tags lists every Tag in the Manifest, remembered or carried, sorted.
func Tags(m *Manifest) []TagCount {
	counts := map[string]int{}
	for _, t := range m.Tags {
		counts[t] = 0
	}
	for _, e := range m.Entries {
		for _, t := range e.Tags {
			n := counts[t]
			if !e.Removed {
				n++
			}
			counts[t] = n
		}
	}
	var out []TagCount
	for t, n := range counts {
		out = append(out, TagCount{t, n})
	}
	slices.SortFunc(out, func(a, b TagCount) int { return strings.Compare(a.Tag, b.Tag) })
	return out
}

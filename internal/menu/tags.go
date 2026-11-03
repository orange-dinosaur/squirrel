package menu

import (
	"slices"

	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

// askTags picks an Entry, then asks for its full set of Tags with the current
// ones ticked. It returns the Entry's ID and the Tags to keep.
func askTags() (string, []string, error) {
	env, err := core.LoadEnv()
	if err != nil {
		return "", nil, err
	}
	m, err := core.Load(env.Store)
	if err != nil {
		return "", nil, err
	}
	id, err := pickEntry(env, m, "Change tags of")
	if err != nil {
		return "", nil, err
	}
	e := m.Entries[slices.IndexFunc(m.Entries, func(e core.Entry) bool { return e.ID == id })]

	var choices []string
	for _, tc := range core.Tags(m) {
		choices = append(choices, tc.Tag)
	}
	for _, t := range core.OSTags {
		if !slices.Contains(choices, t) {
			choices = append(choices, t)
		}
	}
	slices.Sort(choices)

	picked := slices.Clone(e.Tags)
	err = huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Tags for " + e.Target).
			Options(huh.NewOptions(choices...)...).
			Height(min(len(choices), 10) + 1).
			Value(&picked),
	)).WithKeyMap(keys()).Run()
	return id, picked, err
}

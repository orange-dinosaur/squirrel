package menu

import (
	"fmt"
	"slices"
	"strings"

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

	picked := slices.Clone(e.Tags)
	var extra string
	pick, add := tagFields("Tags for "+e.Target, "", tagChoices(m), &picked, &extra)
	err = huh.NewForm(huh.NewGroup(pick, add)).WithKeyMap(keys()).Run()
	return id, append(picked, strings.Fields(extra)...), err
}

// tagChoices lists the Manifest's Tags and the OS Tags, sorted.
func tagChoices(m *core.Manifest) []string {
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
	return choices
}

// tagFields builds a Tags multi-select over choices and a New Tags input for
// Tags not in the list yet.
func tagFields(title, desc string, choices []string, picked *[]string, extra *string) (huh.Field, huh.Field) {
	// Without a height huh subtracts the title and description from the
	// options' own height and hides short lists.
	height := min(len(choices), 10) + 1
	if desc != "" {
		height++
	}
	pick := huh.NewMultiSelect[string]().
		Title(title).
		Description(desc).
		Options(huh.NewOptions(choices...)...).
		Height(height).
		Value(picked)
	add := huh.NewInput().
		Title("New Tags").
		Description("separated by spaces, a-z, 0-9 and -").
		Value(extra).
		Validate(func(s string) error {
			for _, t := range strings.Fields(s) {
				if !core.ValidTag(t) {
					return fmt.Errorf("%q must use only a-z, 0-9 and -", t)
				}
			}
			return nil
		})
	return pick, add
}

package menu

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

// askAdd asks for a path, checked as you type it, then its Tags.
func askAdd() (string, []string, error) {
	env, err := core.LoadEnv()
	if err != nil {
		return "", nil, err
	}
	m, err := core.Load(env.Store)
	if err != nil {
		return "", nil, err
	}

	var choices []string
	for _, tc := range core.Tags(m) {
		choices = append(choices, tc.Tag)
	}
	for _, t := range core.OSTags {
		if !slices.Contains(choices, t) {
			choices = append(choices, t)
		}
	}
	choices = slices.DeleteFunc(choices, func(t string) bool { return t == env.OS })
	slices.Sort(choices)

	var path, extra string
	var picked []string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Path to add").
				Placeholder("~/.zshrc").
				Value(&path).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("type a path")
					}
					return core.CheckAdd(env, m, strings.TrimSpace(s))
				}),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Tags").
				Description(env.OS+" is always added").
				Options(huh.NewOptions(choices...)...).
				// Without a height huh subtracts the title and description
				// from the options' own height and hides short lists.
				Height(min(len(choices), 10)+2).
				Value(&picked),
			huh.NewInput().
				Title("New Tags").
				Description("separated by spaces, a-z, 0-9 and -").
				Value(&extra).
				Validate(func(s string) error {
					for _, t := range strings.Fields(s) {
						if !core.ValidTag(t) {
							return fmt.Errorf("%q must use only a-z, 0-9 and -", t)
						}
					}
					return nil
				}),
		),
	).WithKeyMap(keys()).Run()
	return strings.TrimSpace(path), append(picked, strings.Fields(extra)...), err
}

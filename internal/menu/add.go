package menu

import (
	"errors"
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

	choices := slices.DeleteFunc(tagChoices(m), func(t string) bool { return t == env.OS })

	var path, extra string
	var picked []string
	pick, add := tagFields("Tags", env.OS+" is always added", choices, &picked, &extra)
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
		huh.NewGroup(pick, add),
	).WithKeyMap(keys()).Run()
	return strings.TrimSpace(path), append(picked, strings.Fields(extra)...), err
}

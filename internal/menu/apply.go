package menu

import (
	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

// askApply asks which Tags to apply. None means every Entry for this OS.
func askApply() ([]string, error) {
	env, err := core.LoadEnv()
	if err != nil {
		return nil, err
	}
	m, err := core.Load(env.Store)
	if err != nil {
		return nil, err
	}

	var choices []string
	for _, tc := range core.Tags(m) {
		choices = append(choices, tc.Tag)
	}
	if len(choices) == 0 {
		return nil, nil
	}

	var picked []string
	err = huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Tags to apply").
			Description("pick none to apply everything for " + env.OS).
			Options(huh.NewOptions(choices...)...).
			Height(min(len(choices), 10) + 2).
			Value(&picked),
	)).WithKeyMap(keys()).Run()
	return picked, err
}

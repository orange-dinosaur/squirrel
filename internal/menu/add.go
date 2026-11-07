package menu

import (
	"errors"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

// AskAdd asks for a path, picked from the home folder or typed and checked as
// you go, then its Tags unless withTags is false.
func AskAdd(withTags bool) (string, []string, error) {
	env, err := core.LoadEnv()
	if err != nil {
		return "", nil, err
	}
	m, err := core.Load(env.Store)
	if err != nil {
		return "", nil, err
	}

	choices := slices.DeleteFunc(tagChoices(m), func(t string) bool { return t == env.OS })

	browse := true
	err = huh.NewForm(huh.NewGroup(
		huh.NewSelect[bool]().
			Title("How do you want to find it?").
			Options(huh.NewOption("Browse", true), huh.NewOption("Type a path", false)).
			Value(&browse),
	)).WithKeyMap(keys()).Run()
	if err != nil {
		return "", nil, err
	}

	var path, extra string
	check := func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("pick or type a path")
		}
		s = strings.TrimSpace(s)
		if t, ok := env.Contract(s); ok {
			s = t // the picker gives absolute paths; errors read better as ~/
		}
		return core.CheckAdd(env, m, s)
	}
	var field huh.Field = huh.NewInput().
		Title("Path to add").
		Placeholder("~/.zshrc").
		Value(&path).
		Validate(check)
	if browse {
		field = huh.NewFilePicker().
			Title("Pick a file or directory").
			Description("→ opens a folder, Enter picks it").
			CurrentDirectory(env.Home).
			ShowHidden(true).
			ShowPermissions(false).
			DirAllowed(true).
			Picking(true).
			Height(15).
			Value(&path).
			Validate(check)
	}

	var picked []string
	groups := []*huh.Group{huh.NewGroup(field)}
	if withTags {
		pick, add := tagFields("Tags", env.OS+" is always added", choices, &picked, &extra)
		groups = append(groups, huh.NewGroup(pick, add))
	}
	err = huh.NewForm(groups...).WithKeyMap(keys()).Run()
	return strings.TrimSpace(path), append(picked, strings.Fields(extra)...), err
}

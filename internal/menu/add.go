package menu

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

// AskAdd asks for a path, searched for, picked from the home folder or typed
// and checked as you go, then its Tags unless withTags is false.
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

	how := "search"
	err = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("How do you want to find it?").
			Options(
				huh.NewOption("Search", "search"),
				huh.NewOption("Browse", "browse"),
				huh.NewOption("Type a path", "type"),
			).
			Value(&how),
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
	var groups []*huh.Group
	var fields []huh.Field
	switch how {
	case "search":
		cands := core.SearchCandidates(env, m)
		if len(cands) == 0 {
			fields = append(fields, huh.NewNote().Title("Nothing to search").
				Description("No hidden files in your home folder can be added. Type a path instead."))
			break
		}
		// huh's Select renders every option on each keypress, so a few
		// thousand candidates are too slow to list at once.
		const limit = 200
		matches := func(q string) ([]huh.Option[string], int) {
			q = strings.ToLower(strings.TrimSpace(q))
			var opts []huh.Option[string]
			n := 0
			for _, c := range cands {
				if !strings.Contains(strings.ToLower(c.Target), q) {
					continue
				}
				if n++; n > limit {
					continue
				}
				label := c.Target
				if c.Dir {
					label += "/"
				}
				opts = append(opts, huh.NewOption(label, c.Target))
			}
			return opts, n
		}
		var query string
		groups = append(groups, huh.NewGroup(huh.NewInput().
			Title("Search").
			Placeholder("part of a name, like nvim").
			Value(&query).
			Validate(func(q string) error {
				if _, n := matches(q); n == 0 {
					return errors.New("nothing matches")
				}
				return nil
			})))
		field = huh.NewSelect[string]().
			Title("Pick one").
			DescriptionFunc(func() string {
				if _, n := matches(query); n > limit {
					return fmt.Sprintf("First %d of %d matches, shift+tab to narrow", limit, n)
				}
				return ""
			}, &query).
			OptionsFunc(func() []huh.Option[string] {
				opts, _ := matches(query)
				return opts
			}, &query).
			Height(15).
			Value(&path).
			Validate(check)
	case "browse":
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
	groups = append(groups, huh.NewGroup(append(fields, field)...))
	if withTags {
		pick, add := tagFields("Tags", env.OS+" is always added", choices, &picked, &extra)
		groups = append(groups, huh.NewGroup(pick, add))
	}
	err = huh.NewForm(groups...).WithKeyMap(keys()).Run()
	return strings.TrimSpace(path), append(picked, strings.Fields(extra)...), err
}

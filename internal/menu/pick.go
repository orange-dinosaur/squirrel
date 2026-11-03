package menu

import (
	"errors"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

// pickEntry asks for one non-removed Entry and returns its ID. It lists the
// Entries for this OS first, with an option to list every Entry instead.
func pickEntry(env core.Env, m *core.Manifest, title string) (string, error) {
	if !slices.ContainsFunc(m.Entries, func(e core.Entry) bool { return !e.Removed }) {
		return "", errors.New("no Entries yet")
	}
	const showAll = ""
	all := false
	for {
		var opts []huh.Option[string]
		var id string
		if !all {
			opts = append(opts, huh.NewOption("Show Entries for other OSes", showAll))
		}
		for _, e := range m.Entries {
			if e.Removed || !all && !e.AppliesOn(env.OS) {
				continue
			}
			if id == showAll {
				id = e.ID // start on the first Entry
			}
			opts = append(opts, huh.NewOption(e.Target+"  "+strings.Join(e.Tags, " "), e.ID))
		}

		err := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Options(opts...).
				Value(&id),
		)).WithKeyMap(keys()).Run()
		if err != nil || id != showAll {
			return id, err
		}
		all = true
	}
}

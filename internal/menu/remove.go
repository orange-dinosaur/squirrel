package menu

import (
	"slices"

	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

// askRemove picks an Entry and asks to confirm unmanaging it. It returns the
// Entry's ID and whether the user said yes.
func askRemove() (string, bool, error) {
	env, err := core.LoadEnv()
	if err != nil {
		return "", false, err
	}
	m, err := core.Load(env.Store)
	if err != nil {
		return "", false, err
	}
	id, err := pickEntry(env, m, "Remove")
	if err != nil {
		return "", false, err
	}
	e := m.Entries[slices.IndexFunc(m.Entries, func(e core.Entry) bool { return e.ID == id })]

	desc := "It becomes a real file again."
	if !e.AppliesOn(env.OS) {
		desc = "It isn't meant for " + env.OS + ", so nothing here changes."
	}
	var ok bool
	err = huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Unmanage " + e.Target + "?").
			Description(desc).
			Value(&ok),
	)).WithKeyMap(keys()).Run()
	return id, ok, err
}

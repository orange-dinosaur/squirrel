package menu

import (
	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

// askRemove asks to confirm unmanaging e.
func askRemove(env core.Env, e core.Entry) (bool, error) {
	desc := "It becomes a real file again."
	if !e.AppliesOn(env.OS) {
		desc = "It isn't meant for " + env.OS + ", so nothing here changes."
	}
	var ok bool
	err := RunForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Unmanage " + e.Target + "?").
			Description(desc).
			Value(&ok),
	))
	return ok, err
}

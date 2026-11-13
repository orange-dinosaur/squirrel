package menu

import (
	"errors"
	"io"
	"strings"

	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

// connected reports whether the Store has a Remote. It runs no network.
func connected() bool {
	env, err := core.LoadEnv()
	return err == nil && core.HasRemote(env)
}

// askURL asks for a Remote's address.
func askURL(title string) (string, error) {
	var url string
	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title(title).
			Placeholder("git@github.com:you/dotfiles.git").
			Value(&url).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("type a URL or a path")
				}
				return nil
			}),
	)).WithKeyMap(keys()).WithTheme(theme).Run()
	return strings.TrimSpace(url), err
}

// manageRemote shows the Remote, then offers to disconnect it or change its URL.
func manageRemote(a Actions, w io.Writer) error {
	if err := a.ShowRemote(w); err != nil {
		return err
	}
	const disconnect, setURL, back = "disconnect", "set-url", "back"
	choice := back
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("What do you want to do with the Remote?").
			Options(
				huh.NewOption("Disconnect", disconnect),
				huh.NewOption("Change the URL", setURL),
				huh.NewOption("Back", back),
			).
			Value(&choice),
	)).WithKeyMap(keys()).WithTheme(theme).Run()
	if err != nil {
		return err
	}
	switch choice {
	case disconnect:
		return a.RemoteRemove(w)
	case setURL:
		url, err := askURL("New URL for the Remote")
		if err != nil {
			return err
		}
		return a.RemoteSetURL(w, url)
	}
	return nil
}

// Package menu is the guided menu bare squirrel opens in a terminal.
package menu

import (
	"errors"
	"fmt"
	"os"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
)

// Actions are the commands the menu runs. Each prints its own output.
type Actions struct {
	Status func() error
	Tags   func() error
}

type item int

const (
	add item = iota
	apply
	status
	changeTags
	remove
	listTags
	quit
)

// Run shows the menu until Quit, Esc or Ctrl+C. An action's error is printed
// and the menu comes back.
func Run(a Actions) error {
	for {
		choice, err := pick()
		if errors.Is(err, huh.ErrUserAborted) || choice == quit {
			return nil
		}
		if err != nil {
			return err
		}
		switch choice {
		case status:
			err = a.Status()
		case listTags:
			err = a.Tags()
		default:
			fmt.Println("not yet available")
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Println()
	}
}

func pick() (item, error) {
	keys := huh.NewDefaultKeyMap()
	keys.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))
	choice := add
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[item]().
			Title("What do you want to do?").
			Options(
				huh.NewOption("Add a file or directory", add),
				huh.NewOption("Apply", apply),
				huh.NewOption("Show status", status),
				huh.NewOption("Change tags", changeTags),
				huh.NewOption("Remove", remove),
				huh.NewOption("List tags", listTags),
				huh.NewOption("Quit", quit),
			).
			Value(&choice),
	)).WithKeyMap(keys).Run()
	return choice, err
}

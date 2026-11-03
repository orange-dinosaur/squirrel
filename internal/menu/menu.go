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
	Add     func(path string, tags []string) error
	Apply   func(tags []string) error
	Status  func() error
	SetTags func(id string, tags []string) error
	Tags    func() error
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
// and the menu comes back. Esc inside an action's form also comes back.
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
		case add:
			var path string
			var tags []string
			if path, tags, err = askAdd(); err == nil {
				err = a.Add(path, tags)
			}
		case apply:
			var tags []string
			if tags, err = askApply(); err == nil {
				err = a.Apply(tags)
			}
		case status:
			err = a.Status()
		case changeTags:
			var id string
			var tags []string
			if id, tags, err = askTags(); err == nil {
				err = a.SetTags(id, tags)
			}
		case listTags:
			err = a.Tags()
		default:
			fmt.Println("not yet available")
		}
		if err != nil && !errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Println()
	}
}

// keys lets Esc leave a form, as well as Ctrl+C.
func keys() *huh.KeyMap {
	k := huh.NewDefaultKeyMap()
	k.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))
	return k
}

func pick() (item, error) {
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
	)).WithKeyMap(keys()).Run()
	return choice, err
}

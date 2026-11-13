// Package menu is the guided menu bare squirrel opens in a terminal.
package menu

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/orange-dinosaur/squirrel/internal/style"
)

// Actions are the commands the menu runs. Each prints its own output to w.
type Actions struct {
	Add       func(w io.Writer, path string, tags []string) error
	Apply     func(w io.Writer, tags []string) error
	Remove    func(w io.Writer, id string) error
	Status    func(w io.Writer) error
	SetTags   func(w io.Writer, id string, tags []string) error
	Tags      func(w io.Writer) error
	DeleteTag func(w io.Writer, tag string, untag bool) error
	Purge     func(w io.Writer, ids []string) error // the menu has already asked
	Note      func(w io.Writer, msg string)         // prints a line for an action that did nothing
	Error     func(w io.Writer, err error)          // prints an action's failure
}

type item int

const (
	add item = iota
	apply
	status
	browseEntries
	listTags
	deleteTag
	purge
	quit
)

// Run shows the menu until Quit, Esc or Ctrl+C. An action's error is printed
// and the menu comes back. Esc inside an action's form also comes back.
// After each action the screen clears and shows the header, the action's output, then the menu.
func Run(a Actions) error {
	// Ask the terminal now: once a form owns stdin, the reply would race its keys.
	style.Dark()
	// The tee is not a terminal, so it carries stdout's profile or the actions would drop color.
	profile := colorprofile.Detect(os.Stdout, os.Environ())
	redraw(nil)
	for {
		choice, err := pick()
		if errors.Is(err, huh.ErrUserAborted) || choice == quit {
			return nil
		}
		if err != nil {
			return err
		}
		var out bytes.Buffer
		w := &colorprofile.Writer{Forward: io.MultiWriter(os.Stdout, &out), Profile: profile}
		switch choice {
		case add:
			var path string
			var tags []string
			if path, tags, err = AskAdd(true); err == nil {
				err = a.Add(w, path, tags)
			}
		case apply:
			var tags []string
			if tags, err = askApply(); err == nil {
				err = a.Apply(w, tags)
			}
		case status:
			err = a.Status(w)
		case browseEntries:
			err = browse(a, w)
		case listTags:
			err = a.Tags(w)
		case deleteTag:
			var tag, note string
			var untag bool
			if tag, untag, note, err = askDeleteTag(); err == nil {
				if note != "" {
					a.Note(w, note)
				} else {
					err = a.DeleteTag(w, tag, untag)
				}
			}
		case purge:
			var ids []string
			var note string
			if ids, note, err = askPurge(); err == nil {
				if note != "" {
					a.Note(w, note)
				} else {
					err = a.Purge(w, ids)
				}
			}
		}
		if err != nil && !errors.Is(err, huh.ErrUserAborted) {
			a.Error(w, err)
		}
		redraw(out.Bytes())
	}
}

// redraw clears the screen, then prints the header and the last action's output. The clear
// keeps earlier output in the scrollback.
func redraw(out []byte) {
	fmt.Print("\x1b[H\x1b[2J", header(), "\n\n")
	if len(out) > 0 {
		os.Stdout.Write(out)
		fmt.Println()
	}
}

// theme follows squirrel's dark/light decision rather than huh's, which assumes light when the
// terminal does not answer.
var theme = huh.ThemeFunc(func(bool) *huh.Styles { return huh.ThemeCatppuccin(style.Dark()) })

// keys lets Esc leave a form, as well as Ctrl+C.
func keys() *huh.KeyMap {
	k := huh.NewDefaultKeyMap()
	k.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))
	// Enter picks a folder rather than opening it, so the help names → instead.
	k.FilePicker.Open = key.NewBinding(key.WithKeys("l", "right", "enter"), key.WithHelp("→", "open"))
	return k
}

func pick() (item, error) {
	choice := add
	opts := []huh.Option[item]{
		huh.NewOption("Add a file or directory", add),
		huh.NewOption("Apply", apply),
		huh.NewOption("Show status", status),
		huh.NewOption("Browse Entries", browseEntries),
		huh.NewOption("List tags", listTags),
		huh.NewOption("Delete a tag", deleteTag),
	}
	if hasRemoved() {
		opts = append(opts, huh.NewOption("Purge removed Entries", purge))
	}
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[item]().
			Title("What do you want to do?").
			Options(append(opts, huh.NewOption("Quit", quit))...).
			Value(&choice),
	)).WithKeyMap(keys()).WithTheme(theme).Run()
	return choice, err
}

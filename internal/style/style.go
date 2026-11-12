// Package style holds squirrel's Catppuccin colors and badges, shared by the commands and the menu.
package style

import (
	"image/color"
	"os"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	catppuccin "github.com/catppuccin/go"
	"github.com/charmbracelet/x/term"
)

// Outcome is what a result line or state means for the user.
type Outcome int

const (
	Done Outcome = iota // it worked
	Noop                // nothing happened, or something went away
	Warn                // worth a look
	Fail                // the command failed
)

// Dark reports whether the terminal background is dark. It asks the terminal once per run, and
// only when stdout is a terminal. With no answer it assumes dark.
var Dark = sync.OnceValue(func() bool {
	if !term.IsTerminal(os.Stdout.Fd()) {
		return true
	}
	return lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
})

func flavour() catppuccin.Flavour {
	if Dark() {
		return catppuccin.Mocha
	}
	return catppuccin.Latte
}

// catppuccin.Color's RGBA does not follow color.Color, so lipgloss gets the hex string.
func hex(c catppuccin.Color) color.Color { return lipgloss.Color(c.Hex) }

func Green() color.Color   { return hex(flavour().Green()) }
func Yellow() color.Color  { return hex(flavour().Yellow()) }
func Red() color.Color     { return hex(flavour().Red()) }
func Mauve() color.Color   { return hex(flavour().Mauve()) }
func Overlay() color.Color { return hex(flavour().Overlay1()) }
func Base() color.Color    { return hex(flavour().Base()) }

// Color is the badge color for an outcome.
func Color(o Outcome) color.Color {
	switch o {
	case Done:
		return Green()
	case Warn:
		return Yellow()
	case Fail:
		return Red()
	}
	return Overlay()
}

// Badge renders word in uppercase on its outcome's color, padded to width cells plus one space
// each side: " ADDED   ".
func Badge(o Outcome, word string, width int) string {
	word = strings.ToUpper(word)
	word += strings.Repeat(" ", max(0, width-lipgloss.Width(word)))
	return lipgloss.NewStyle().Background(Color(o)).Foreground(Base()).Bold(true).Render(" " + word + " ")
}

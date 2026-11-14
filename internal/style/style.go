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
	"github.com/orange-dinosaur/squirrel/internal/core"
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
	// The terminal answers on stdin, so without one the question can only time out.
	if !term.IsTerminal(os.Stdout.Fd()) || !term.IsTerminal(os.Stdin.Fd()) {
		return true
	}
	return lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
})

// Flavour is the Catppuccin flavour that matches the terminal: Mocha on dark, Latte on light.
func Flavour() catppuccin.Flavour {
	if Dark() {
		return catppuccin.Mocha
	}
	return catppuccin.Latte
}

// catppuccin.Color's RGBA does not follow color.Color, so lipgloss gets the hex string.
func hex(c catppuccin.Color) color.Color { return lipgloss.Color(c.Hex) }

func green() color.Color   { return hex(Flavour().Green()) }
func yellow() color.Color  { return hex(Flavour().Yellow()) }
func red() color.Color     { return hex(Flavour().Red()) }
func Mauve() color.Color   { return hex(Flavour().Mauve()) }
func Overlay() color.Color { return hex(Flavour().Overlay1()) }
func base() color.Color    { return hex(Flavour().Base()) }

// badgeColor is the badge color for an outcome.
func badgeColor(o Outcome) color.Color {
	switch o {
	case Done:
		return green()
	case Warn:
		return yellow()
	case Fail:
		return red()
	}
	return Overlay()
}

// StateOutcome gives an Entry's state its badge color.
func StateOutcome(s core.State) Outcome {
	switch s {
	case core.Linked:
		return Done
	case core.RemovedState, core.OtherOS:
		return Noop
	}
	return Warn
}

// Badge renders word in uppercase on its outcome's color, padded to width cells plus one space
// each side: " ADDED   ".
func Badge(o Outcome, word string, width int) string {
	word = strings.ToUpper(word)
	word += strings.Repeat(" ", max(0, width-lipgloss.Width(word)))
	return lipgloss.NewStyle().Background(badgeColor(o)).Foreground(base()).Bold(true).Render(" " + word + " ")
}

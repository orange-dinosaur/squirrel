package cli

import (
	"image/color"

	"charm.land/lipgloss/v2"
	catppuccin "github.com/catppuccin/go"
	"github.com/charmbracelet/fang"
)

// helpColors maps Fang's help colors onto Catppuccin, Latte on light terminals and Mocha on dark.
func helpColors(c lipgloss.LightDarkFunc) fang.ColorScheme {
	pick := func(f func(catppuccin.Flavour) catppuccin.Color) color.Color {
		return c(lipgloss.Color(f(catppuccin.Latte).Hex), lipgloss.Color(f(catppuccin.Mocha).Hex))
	}
	text := pick(catppuccin.Flavour.Text)
	overlay := pick(catppuccin.Flavour.Overlay1)
	return fang.ColorScheme{
		Base:           text,
		Title:          pick(catppuccin.Flavour.Mauve),
		Description:    text,
		Codeblock:      pick(catppuccin.Flavour.Mantle),
		Program:        pick(catppuccin.Flavour.Blue),
		DimmedArgument: overlay,
		Comment:        overlay,
		Flag:           pick(catppuccin.Flavour.Subtext1),
		FlagDefault:    overlay,
		Command:        pick(catppuccin.Flavour.Pink),
		QuotedString:   pick(catppuccin.Flavour.Green),
		Argument:       text,
		Help:           text,
		Dash:           overlay,
		ErrorHeader:    [2]color.Color{pick(catppuccin.Flavour.Base), pick(catppuccin.Flavour.Red)},
		ErrorDetails:   pick(catppuccin.Flavour.Red),
	}
}

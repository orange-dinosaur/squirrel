package cli

import (
	"image/color"

	"charm.land/lipgloss/v2"
	catppuccin "github.com/catppuccin/go"
	"github.com/charmbracelet/fang"
	"github.com/orange-dinosaur/squirrel/internal/style"
)

// helpColors maps Fang's help colors onto the Catppuccin flavour the rest of squirrel uses, so
// help and output never disagree. Fang's own light/dark guess is ignored.
func helpColors(lipgloss.LightDarkFunc) fang.ColorScheme {
	f := style.Flavour()
	pick := func(c catppuccin.Color) color.Color { return lipgloss.Color(c.Hex) }
	text := pick(f.Text())
	overlay := pick(f.Overlay1())
	return fang.ColorScheme{
		Base:           text,
		Title:          pick(f.Mauve()),
		Description:    text,
		Codeblock:      pick(f.Mantle()),
		Program:        pick(f.Blue()),
		DimmedArgument: overlay,
		Comment:        overlay,
		Flag:           pick(f.Subtext1()),
		FlagDefault:    overlay,
		Command:        pick(f.Pink()),
		QuotedString:   pick(f.Green()),
		Argument:       text,
		Help:           text,
		Dash:           overlay,
		ErrorHeader:    [2]color.Color{pick(f.Base()), pick(f.Red())},
		ErrorDetails:   pick(f.Red()),
	}
}

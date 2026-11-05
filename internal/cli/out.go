package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

type mark int

const (
	done mark = iota // it worked
	noop             // nothing happened, or something went away
	warn             // worth a look
	fail             // the command failed
)

var marks = map[mark]struct {
	sym   string
	style lipgloss.Style
}{
	done: {"✓", lipgloss.NewStyle().Foreground(lipgloss.Color("2"))},
	noop: {"−", lipgloss.NewStyle().Faint(true)},
	warn: {"!", lipgloss.NewStyle().Foreground(lipgloss.Color("3"))},
	fail: {"✗", lipgloss.NewStyle().Foreground(lipgloss.Color("1"))},
}

var dim = lipgloss.NewStyle().Faint(true)

// line is one result: "✓ added  ~/.zshrc  562e17c6  linux".
type line struct {
	mark   mark
	verb   string
	target string
	extras []string
}

// styled reports whether w takes color and marks. A pipe, a file, or NO_COLOR gets plain text.
func styled(w io.Writer) bool {
	return colorprofile.Detect(w, os.Environ()) > colorprofile.ASCII
}

// printLines prints lines with their verbs padded to the widest one.
func printLines(w io.Writer, lines ...line) {
	color := styled(w)
	width := 0
	for _, l := range lines {
		width = max(width, len(l.verb))
	}
	for _, l := range lines {
		verb := l.verb
		if l.target != "" {
			verb = fmt.Sprintf("%-*s", width, verb)
		}
		if color {
			m := marks[l.mark]
			verb = m.style.Render(m.sym + " " + verb)
		}
		parts := []string{verb}
		if l.target != "" {
			parts = append(parts, l.target)
		}
		for _, x := range l.extras {
			if x == "" {
				continue
			}
			if color {
				x = dim.Render(x)
			}
			parts = append(parts, x)
		}
		fmt.Fprintln(w, strings.Join(parts, "  "))
	}
}

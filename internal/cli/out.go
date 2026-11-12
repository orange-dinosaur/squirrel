package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/orange-dinosaur/squirrel/internal/style"
	"github.com/spf13/cobra"
)

type mark = style.Outcome

const (
	done = style.Done
	noop = style.Noop
	warn = style.Warn
	fail = style.Fail
)

var dim = lipgloss.NewStyle().Faint(true)

// line is one result: " ADDED  ~/.zshrc  562e17c6  linux". A line without a target is a note
// or an error, and its verb is the message: " NOTE  nothing to do".
type line struct {
	mark   mark
	verb   string
	target string
	extras []string
}

// styled reports whether w takes color and badges. A pipe, a file, or NO_COLOR gets plain text.
func styled(w io.Writer) bool {
	if cw, ok := w.(*colorprofile.Writer); ok {
		return cw.Profile > colorprofile.ASCII
	}
	return colorprofile.Detect(w, os.Environ()) > colorprofile.ASCII
}

// hintError is an error with a next step worth printing under it.
type hintError struct {
	hint    string   // "try squirrel add --help"
	details []string // lines printed between the error and the hint
	err     error
}

func (e hintError) Error() string { return e.err.Error() }
func (e hintError) Unwrap() error { return e.err }

// usageError is a mistake in how cmd was typed, so its help is worth a look.
func usageError(cmd *cobra.Command, err error) error {
	return hintError{hint: "try " + cmd.CommandPath() + " --help", err: err}
}

// printErr prints " ERROR  message", plus the details and hint of a hintError.
func printErr(w io.Writer, err error) {
	printLines(w, line{fail, err.Error(), "", nil})
	hint := ""
	if he, ok := errors.AsType[hintError](err); ok {
		for _, d := range he.details {
			fmt.Fprintln(w, "  "+d)
		}
		hint = he.hint
	} else if strings.HasPrefix(err.Error(), "unknown command") {
		hint = "try squirrel --help"
	}
	if hint == "" {
		return
	}
	if styled(w) {
		hint = dim.Render(hint)
	}
	fmt.Fprintln(w, "  "+hint)
}

// badgeWord splits l into its badge word and what follows it.
func (l line) badgeWord() (word, target string) {
	switch {
	case l.target != "":
		return l.verb, l.target
	case l.mark == fail:
		return "error", l.verb
	}
	return "note", l.verb
}

// printLines prints lines with their badges padded to the widest one. Plain output gets the
// uppercase word instead of a badge.
func printLines(w io.Writer, lines ...line) {
	color := styled(w)
	width := 0
	for _, l := range lines {
		word, _ := l.badgeWord()
		width = max(width, lipgloss.Width(word))
	}
	for _, l := range lines {
		word, target := l.badgeWord()
		if color {
			word = style.Badge(l.mark, word, width)
		} else {
			word = fmt.Sprintf("%-*s", width, strings.ToUpper(word))
		}
		parts := []string{word, target}
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

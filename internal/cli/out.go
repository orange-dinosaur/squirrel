package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
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

// printTable draws a rounded box with a line between rows, shrunk and wrapped to the terminal
// when it is too wide. Plain
// output gets aligned columns under a plain header instead. dimCols are drawn faint.
func printTable(w io.Writer, headers []string, rows [][]string, dimCols ...int) {
	if !styled(w) {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, r := range append([][]string{headers}, rows...) {
			fmt.Fprintln(tw, strings.Join(r, "\t"))
		}
		tw.Flush()
		return
	}
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderRow(true).
		BorderStyle(lipgloss.NewStyle().Foreground(style.Overlay())).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			switch {
			case row == table.HeaderRow:
				return s.Foreground(style.Mauve()).Bold(true)
			case slices.Contains(dimCols, col):
				return s.Faint(true)
			}
			return s
		})
	if cols, _, err := term.GetSize(os.Stdout.Fd()); err == nil && lipgloss.Width(t.String()) > cols {
		t.Width(cols).Wrap(true)
	}
	fmt.Fprintln(w, t)
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

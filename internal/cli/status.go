package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "status [tags...]",
		Short: "Show whether each Entry is linked",

		ValidArgsFunction: completeTags,
		RunE: func(cmd *cobra.Command, tags []string) error {
			ok, err := printStatus(cmd.OutOrStdout(), tags, all)
			if err == nil && !ok {
				return errQuiet
			}
			return err
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "also show Entries for other OSes")
	return cmd
}

// printStatus prints the status table and reports whether every row is OK.
func printStatus(w io.Writer, tags []string, all bool) (bool, error) {
	env, err := core.LoadEnv()
	if err != nil {
		return false, err
	}
	m, err := core.Load(env.Store)
	if err != nil {
		return false, err
	}
	rows, err := core.Status(env, m, tags, all)
	if err != nil {
		return false, err
	}
	if len(rows) == 0 {
		printLines(w, line{noop, "no Entries", "", nil})
		return true, nil
	}
	printStatusRows(w, rows)
	return !slices.ContainsFunc(rows, func(r core.Row) bool { return !r.OK() }), nil
}

// printStatusRows prints a header, then one row per Entry: mark, target, Tags, ID, and the state
// unless it is linked.
func printStatusRows(w io.Writer, rows []core.Row) {
	color := styled(w)
	cells := [][3]string{{"TARGET", "TAGS", "ID"}}
	for _, r := range rows {
		cells = append(cells, [3]string{r.Entry.Target, strings.Join(r.Entry.Tags, " "), r.Entry.ID})
	}
	var width [3]int
	for _, c := range cells {
		for i, s := range c {
			width[i] = max(width[i], lipgloss.Width(s))
		}
	}
	pad := func(s string, i int) string { return s + strings.Repeat(" ", width[i]-lipgloss.Width(s)) }

	for i, c := range cells {
		m, state := noop, ""
		if i > 0 {
			m, state = statusMark(rows[i-1].State)
		}
		target, tags, id := pad(c[0], 0), pad(c[1], 1), c[2]
		hasState := state != ""
		if hasState {
			id = pad(id, 2)
		}
		if color {
			style := marks[m].style
			switch {
			case i == 0:
				target, tags, id = "  "+dim.Render(target), dim.Render(tags), dim.Render(id)
			default:
				target = style.Render(marks[m].sym) + " " + target
				tags, id, state = dim.Render(tags), dim.Render(id), style.Render(state)
			}
		}
		parts := []string{target, tags, id}
		if hasState {
			parts = append(parts, state)
		}
		fmt.Fprintln(w, strings.Join(parts, "  "))
	}
}

// statusMark gives a state its mark, and the word to show beside it. Linked needs no word.
func statusMark(s core.State) (mark, string) {
	switch s {
	case core.Linked:
		return done, ""
	case core.RemovedState, core.OtherOS:
		return noop, string(s)
	}
	return warn, string(s)
}

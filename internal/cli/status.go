package cli

import (
	"io"
	"slices"
	"strings"

	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/orange-dinosaur/squirrel/internal/style"
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
	env, m, err := core.Open()
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

// printStatusRows prints one row per Entry: state, target, Tags and ID.
func printStatusRows(w io.Writer, rows []core.Row) {
	color := styled(w)
	width := 0
	for _, r := range rows {
		width = max(width, len(r.State))
	}
	cells := make([][]string, len(rows))
	for i, r := range rows {
		state := strings.ToUpper(string(r.State))
		if color {
			state = style.Badge(style.StateOutcome(r.State), state, width)
		}
		cells[i] = []string{state, r.Entry.Target, strings.Join(r.Entry.Tags, " "), r.Entry.ID}
	}
	printTable(w, []string{"STATE", "TARGET", "TAGS", "ID"}, cells, 0, 2, 3)
}

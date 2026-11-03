package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
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
		fmt.Fprintln(w, "no Entries")
		return true, nil
	}
	fmt.Fprintln(w, statusTable(rows))
	return !slices.ContainsFunc(rows, func(r core.Row) bool { return !r.OK() }), nil
}

func statusTable(rows []core.Row) string {
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		StyleFunc(func(row, col int) lipgloss.Style { return lipgloss.NewStyle().Padding(0, 1) }).
		Headers("ID", "TARGET", "TAGS", "STATE")
	for _, r := range rows {
		t.Row(r.Entry.ID, r.Entry.Target, strings.Join(r.Entry.Tags, " "), string(r.State))
	}
	return t.String()
}

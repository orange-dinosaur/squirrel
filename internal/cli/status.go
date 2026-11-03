package cli

import (
	"fmt"
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
		RunE: func(cmd *cobra.Command, tags []string) error {
			env, err := core.LoadEnv()
			if err != nil {
				return err
			}
			m, err := core.Load(env.Store)
			if err != nil {
				return err
			}
			rows, err := core.Status(env, m, tags, all)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no Entries")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), statusTable(rows))
			for _, r := range rows {
				if !r.OK() {
					return errQuiet
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "also show Entries for other OSes")
	return cmd
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

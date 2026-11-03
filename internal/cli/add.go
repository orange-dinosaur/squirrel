package cli

import (
	"fmt"
	"strings"

	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var tags []string
	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Move a file or directory into the Store and link it back",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := core.LoadEnv()
			if err != nil {
				return err
			}
			m, err := core.Load(env.Store)
			if err != nil {
				return err
			}
			e, err := core.Add(env, m, args[0], tags)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s (%s) [%s]\n", e.Target, e.ID, strings.Join(e.Tags, " "))
			return nil
		},
	}
	cmd.Flags().StringArrayVarP(&tags, "tag", "t", nil, "tag the Entry, repeatable (this OS is always added)")
	return cmd
}

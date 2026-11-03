package cli

import (
	"fmt"

	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <path|id>",
		Short: "Stop managing an Entry and put a real copy back in place of its Link",
		Args:  cobra.ExactArgs(1),

		ValidArgsFunction: targetThen(completeNothing),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := core.LoadEnv()
			if err != nil {
				return err
			}
			m, err := core.Load(env.Store)
			if err != nil {
				return err
			}
			e, replaced, err := core.Remove(env, m, args[0])
			if err != nil {
				return err
			}
			if replaced {
				fmt.Fprintf(cmd.OutOrStdout(), "removed %s (%s), a real copy replaced the Link\n", e.Target, e.ID)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "removed %s (%s), it had no Link here\n", e.Target, e.ID)
			}
			return nil
		},
	}
}

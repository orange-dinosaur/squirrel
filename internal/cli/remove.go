package cli

import (
	"io"

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
			return runRemove(cmd.OutOrStdout(), args[0])
		},
	}
}

func runRemove(w io.Writer, arg string) error {
	env, m, err := core.Open()
	if err != nil {
		return err
	}
	e, replaced, err := core.Remove(env, m, arg)
	if err != nil {
		return err
	}
	note := "it had no Link here"
	if replaced {
		note = "a real copy replaced the Link"
	}
	printLines(w, line{noop, "removed", e.Target, []string{e.ID, note}})
	return nil
}

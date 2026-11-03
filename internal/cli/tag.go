package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/spf13/cobra"
)

type retagFunc func(core.Env, *core.Manifest, string, []string) (core.Entry, error)

func newRetagCmd(use, short string, change retagFunc, completeRest completeFunc) *cobra.Command {
	return &cobra.Command{
		Use:               use + " <path|id> <tags...>",
		Short:             short,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: targetThen(completeRest),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := core.LoadEnv()
			if err != nil {
				return err
			}
			m, err := core.Load(env.Store)
			if err != nil {
				return err
			}
			e, err := change(env, m, args[0], args[1:])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", e.Target, strings.Join(e.Tags, " "))
			return nil
		},
	}
}

func newTagsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tags",
		Short: "List every Tag with the number of Entries that carry it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := core.LoadEnv()
			if err != nil {
				return err
			}
			m, err := core.Load(env.Store)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, tc := range core.Tags(m) {
				fmt.Fprintf(w, "%s\t%d\n", tc.Tag, tc.Count)
			}
			return w.Flush()
		},
	}
}

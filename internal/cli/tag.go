package cli

import (
	"fmt"
	"io"
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

func runSetTags(w io.Writer, id string, tags []string) error {
	env, err := core.LoadEnv()
	if err != nil {
		return err
	}
	m, err := core.Load(env.Store)
	if err != nil {
		return err
	}
	e, err := core.SetTags(env, m, id, tags)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%s: %s\n", e.Target, strings.Join(e.Tags, " "))
	return nil
}

func newTagsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tags",
		Short: "List every Tag with the number of Entries that carry it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printTags(cmd.OutOrStdout())
		},
	}
}

func printTags(out io.Writer) error {
	env, err := core.LoadEnv()
	if err != nil {
		return err
	}
	m, err := core.Load(env.Store)
	if err != nil {
		return err
	}
	tags := core.Tags(m)
	if len(tags) == 0 {
		fmt.Fprintln(out, "no Tags")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, tc := range tags {
		fmt.Fprintf(w, "%s\t%d\n", tc.Tag, tc.Count)
	}
	return w.Flush()
}

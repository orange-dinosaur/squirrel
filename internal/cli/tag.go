package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

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
			env, m, err := core.Open()
			if err != nil {
				return err
			}
			e, err := change(env, m, args[0], args[1:])
			if err != nil {
				return err
			}
			printTagged(cmd.OutOrStdout(), use+"ged", e) // tagged, untagged
			return nil
		},
	}
}

func runSetTags(w io.Writer, id string, tags []string) error {
	env, m, err := core.Open()
	if err != nil {
		return err
	}
	e, err := core.SetTags(env, m, id, tags)
	if err != nil {
		return err
	}
	printTagged(w, "tagged", e)
	return nil
}

// printTagged shows the Tags an Entry carries after a change.
func printTagged(w io.Writer, verb string, e core.Entry) {
	printLines(w, line{done, verb, e.Target, []string{strings.Join(e.Tags, " ")}})
}

func newTagsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "tags",
		Short: "List every Tag with the number of Entries that carry it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printTags(cmd.OutOrStdout())
		},
	}
	var untag bool
	rm := &cobra.Command{
		Use:               "remove <tag>",
		Aliases:           []string{"rm"},
		Short:             "Forget a Tag",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeDeletableTags,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeleteTag(cmd.OutOrStdout(), args[0], untag)
		},
	}
	rm.Flags().BoolVar(&untag, "untag", false, "drop the Tag from the Entries that carry it first")
	c.AddCommand(rm)
	return c
}

func runDeleteTag(w io.Writer, tag string, untag bool) error {
	env, m, err := core.Open()
	if err != nil {
		return err
	}
	untagged, err := core.DeleteTag(env, m, tag, untag)
	if ce, ok := errors.AsType[core.TagCarriedError](err); ok {
		width := 0
		for _, e := range ce.Entries {
			width = max(width, len(e.Target))
		}
		var details []string
		for _, e := range ce.Entries {
			details = append(details, fmt.Sprintf("%-*s  %s", width, e.Target, e.ID))
		}
		return hintError{"try squirrel tags remove " + tag + " --untag", details, err}
	}
	if err != nil {
		return err
	}
	var lines []line
	for _, e := range untagged {
		lines = append(lines, line{noop, "untagged", e.Target, []string{tag}})
	}
	printLines(w, append(lines, line{noop, "deleted", tag, nil})...)
	return nil
}

func printTags(out io.Writer) error {
	_, m, err := core.Open()
	if err != nil {
		return err
	}
	tags := core.Tags(m)
	if len(tags) == 0 {
		printLines(out, line{noop, "no Tags", "", nil})
		return nil
	}
	rows := make([][]string, len(tags))
	for i, tc := range tags {
		rows[i] = []string{tc.Tag, strconv.Itoa(tc.Count)}
	}
	printTable(out, []string{"TAG", "ENTRIES"}, rows, -1)
	return nil
}

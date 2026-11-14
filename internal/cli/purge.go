package cli

import (
	"errors"
	"io"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/orange-dinosaur/squirrel/internal/menu"
	"github.com/spf13/cobra"
)

func newPurgeCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "purge [path|id...]",
		Short: "Delete Removed Entries and their Store files for good",
		Long: "Delete Removed Entries and their Store files for good. With no arguments, purge\n" +
			"every Removed Entry. Run apply on every machine first, so each one has swapped\n" +
			"its Link for a real copy.",

		ValidArgsFunction: completeRemovedTargets,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPurge(cmd.OutOrStdout(), args, yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "purge without asking")
	return cmd
}

func runPurge(w io.Writer, args []string, yes bool) error {
	env, m, err := core.Open()
	if err != nil {
		return err
	}
	entries, err := core.PurgeTargets(env, m, args)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		printLines(w, line{noop, "no removed Entries to purge", "", nil})
		return nil
	}
	if !yes {
		if !stdinIsTerminal() {
			return errors.New("purging asks first: run purge in a terminal, or pass --yes to purge without asking")
		}
		var lines []line
		for _, e := range entries {
			lines = append(lines, line{noop, "to purge", e.Target, []string{e.ID, strings.Join(e.Tags, " ")}})
		}
		printLines(w, lines...)
		ok := false
		err := menu.RunForm(huh.NewGroup(huh.NewConfirm().
			Title("Has every machine run apply since these were removed?").
			Description("Purging deletes their Store files. A machine that hasn't caught up keeps a broken Link.").
			Value(&ok)))
		if err != nil || !ok {
			printLines(w, line{noop, "nothing purged", "", nil})
			return errQuiet
		}
	}
	err = core.Purge(env, m, entries)
	if err != nil && slices.ContainsFunc(m.Entries, func(e core.Entry) bool { return e.ID == entries[0].ID }) {
		return err // the Manifest was not saved, so nothing was purged
	}
	var lines []line
	for _, e := range entries {
		lines = append(lines, line{noop, "purged", e.Target, []string{e.ID}})
	}
	printLines(w, lines...)
	return err
}

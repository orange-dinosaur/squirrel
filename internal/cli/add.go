package cli

import (
	"errors"
	"io"
	"strings"

	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/orange-dinosaur/squirrel/internal/menu"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var tags []string
	cmd := &cobra.Command{
		Use:   "add [path]",
		Short: "Move a file or directory into the Store and link it back",
		Long: "Move a file or directory into the Store and link it back.\n\n" +
			"Without a path, in a terminal, pick or type one. --tag skips the Tags question.",
		Args: cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return runAdd(cmd.OutOrStdout(), args[0], tags)
			}
			if !stdinIsTerminal() {
				return usageError(cmd, errors.New("add needs a path when not run in a terminal"))
			}
			path, picked, err := menu.AskAdd(len(tags) == 0)
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			if err != nil {
				return err
			}
			return runAdd(cmd.OutOrStdout(), path, append(tags, picked...))
		},
	}
	cmd.Flags().StringArrayVarP(&tags, "tag", "t", nil, "tag the Entry, repeatable (this OS is always added)")
	return cmd
}

func runAdd(w io.Writer, path string, tags []string) error {
	env, m, err := core.Open()
	if err != nil {
		return err
	}
	e, err := core.Add(env, m, path, tags)
	if err != nil {
		return err
	}
	printLines(w, line{done, "added", e.Target, []string{e.ID, strings.Join(e.Tags, " ")}})
	return nil
}

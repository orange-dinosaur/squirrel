package cli

import (
	"errors"
	"io"

	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	var backup, overwrite bool
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Send the Store's changes to the Remote, taking in the Remote's first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if backup && overwrite {
				return errors.New("--backup and --overwrite cannot be used together")
			}
			return runPush(cmd.OutOrStdout(), backup, overwrite)
		},
	}
	cmd.Flags().BoolVar(&backup, "backup", false, "resolve every Conflict by renaming it to <target>.squirrel-backup")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "resolve every Conflict by deleting what is in the way")
	return cmd
}

func runPush(w io.Writer, backup, overwrite bool) error {
	env, err := core.LoadEnv()
	if err != nil {
		return err
	}
	res, err := core.Push(env)
	if c, ok := errors.AsType[core.ClashError](err); ok {
		g := "git -C " + storeName(env)
		hint := "combine them by hand: run " + g + " rebase origin/main, fix each file, " +
			g + " add -A, " + g + " rebase --continue, then squirrel push and squirrel apply"
		return hintError{hint, entryNames(env, c.Paths), c}
	}
	if err != nil {
		return err
	}
	if res.Pulled {
		printLines(w, line{done, "pulled", storeName(env), []string{"from the Remote"}})
	}
	if res.Pushed {
		printLines(w, line{done, "pushed", storeName(env), []string{"to the Remote"}})
	}
	if !res.Pulled && !res.Pushed {
		printLines(w, line{noop, "nothing to push", "", nil})
	}
	if res.Pulled {
		return runApply(w, nil, backup, overwrite)
	}
	return nil
}

package cli

import (
	"errors"
	"io"

	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	var f conflictFlags
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Send the Store's changes to the Remote, taking in the Remote's first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := f.check(); err != nil {
				return err
			}
			return runPush(cmd.OutOrStdout(), f.backup, f.overwrite)
		},
	}
	f.register(cmd, "")
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
		return nestedRepoHint(env, err)
	}
	if res.Pulled {
		printLines(w, line{done, "pulled", storeName(env), []string{"from the Remote"}})
		printKept(w, res.Kept)
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

// nestedRepoHint lists the Entries behind a NestedRepoError and says how to
// fix it. Any other error passes through.
func nestedRepoHint(env core.Env, err error) error {
	if n, ok := errors.AsType[core.NestedRepoError](err); ok {
		hint := "delete the .git folder inside each one in " + storeName(env) + ", or remove that Entry"
		return hintError{hint, entryNames(env, n.Paths), n}
	}
	return err
}

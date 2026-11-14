package cli

import (
	"errors"
	"io"

	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	var backup, overwrite bool
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Bring the Remote's changes into the Store and apply them",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if backup && overwrite {
				return errors.New("--backup and --overwrite cannot be used together")
			}
			return runPull(cmd.OutOrStdout(), backup, overwrite)
		},
	}
	cmd.Flags().BoolVar(&backup, "backup", false, "resolve every Conflict by renaming it to <target>.squirrel-backup")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "resolve every Conflict by deleting what is in the way")
	return cmd
}

func runPull(w io.Writer, backup, overwrite bool) error {
	env, err := core.LoadEnv()
	if err != nil {
		return err
	}
	res, err := core.Pull(env)
	if d, ok := errors.AsType[core.DirtyError](err); ok {
		return dirtyHint(env, d)
	}
	if err != nil {
		return err
	}
	if !res.Moved {
		printLines(w, line{noop, "already up to date", "", nil})
		return nil
	}
	printLines(w, line{done, "pulled", storeName(env), []string{"from the Remote"}})
	printKept(w, res.Kept)
	return runApply(w, nil, backup, overwrite)
}

// printKept lists the Entries whose Links became real copies because another
// machine purged them.
func printKept(w io.Writer, kept []core.Entry) {
	var lines []line
	for _, e := range kept {
		lines = append(lines, line{warn, "kept", e.Target, []string{"as a real copy, it was purged on another machine"}})
	}
	printLines(w, lines...)
}

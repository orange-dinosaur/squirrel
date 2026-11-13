package cli

import (
	"errors"
	"io"

	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/spf13/cobra"
)

func newRemoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Connect the Store to a git Remote",
	}
	cmd.AddCommand(newRemoteAddCmd())
	return cmd
}

func newRemoteAddCmd() *cobra.Command {
	var backup, overwrite bool
	cmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Push the Store to an empty Remote, or fill an empty Store from the Remote",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if backup && overwrite {
				return errors.New("--backup and --overwrite cannot be used together")
			}
			return runRemoteAdd(cmd.OutOrStdout(), args[0], backup, overwrite)
		},
	}
	cmd.Flags().BoolVar(&backup, "backup", false, "after a clone, resolve every Conflict by renaming it to <target>.squirrel-backup")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "after a clone, resolve every Conflict by deleting what is in the way")
	return cmd
}

func runRemoteAdd(w io.Writer, url string, backup, overwrite bool) error {
	env, err := core.LoadEnv()
	if err != nil {
		return err
	}
	res, err := core.RemoteAdd(env, url)
	if err != nil {
		return err
	}
	switch res {
	case core.RemotePushed:
		printLines(w, line{done, "pushed", url, []string{"the Store is on the Remote"}})
	case core.RemoteConnected:
		printLines(w, line{done, "connected", url, []string{"both empty, nothing to push yet"}})
	case core.RemoteUnchanged:
		printLines(w, line{noop, "the Store is already connected to " + url, "", nil})
	case core.RemoteCloned:
		store, ok := env.Contract(env.Store)
		if !ok {
			store = env.Store
		}
		printLines(w, line{done, "cloned", url, []string{"into " + store}})
		return runApply(w, nil, backup, overwrite)
	}
	return nil
}

package cli

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/spf13/cobra"
)

func newRemoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Show how the Store stands against its git Remote, or connect one",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRemote(cmd.OutOrStdout())
		},
	}
	cmd.AddCommand(newRemoteAddCmd())
	return cmd
}

func runRemote(w io.Writer) error {
	env, err := core.LoadEnv()
	if err != nil {
		return err
	}
	s, err := core.RemoteState(env)
	if errors.Is(err, core.ErrNoRemote) {
		printLines(w, line{noop, err.Error(), "", nil})
		return nil
	}
	if err != nil && s.URL == "" {
		return err
	}
	status := syncStatus(s)
	if err != nil {
		status = "unknown, could not reach the Remote"
	}
	rows := [][]string{{"Store", storeName(env)}, {"Status", status}}
	if len(s.Dirty) > 0 {
		rows = append(rows, []string{"Not pushed", strings.Join(entryNames(env, s.Dirty), ", ")})
	}
	printTable(w, []string{"Remote", s.URL}, rows, 0)
	return nil
}

// syncStatus says what push and pull would move, in commits.
func syncStatus(s core.RemoteStatus) string {
	var parts []string
	if s.Ahead > 0 {
		parts = append(parts, commits(s.Ahead)+" to push")
	}
	if s.Behind > 0 {
		parts = append(parts, commits(s.Behind)+" to pull")
	}
	switch {
	case len(parts) > 0:
		return strings.Join(parts, ", ")
	case len(s.Dirty) > 0:
		return "changes to push"
	}
	return "in sync"
}

func commits(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return strconv.Itoa(n) + " commits"
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
		printLines(w, line{done, "cloned", url, []string{"into " + storeName(env)}})
		return runApply(w, nil, backup, overwrite)
	}
	return nil
}

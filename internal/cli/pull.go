package cli

import (
	"errors"
	"io"
	"strings"

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
	return runApply(w, nil, backup, overwrite)
}

// storeName is the Store's path with the home directory as ~.
func storeName(env core.Env) string {
	if s, ok := env.Contract(env.Store); ok {
		return s
	}
	return env.Store
}

func dirtyHint(env core.Env, d core.DirtyError) error {
	return hintError{"run squirrel push, or set them aside with git -C " + storeName(env) + " stash -u", entryNames(env, d.Paths), d}
}

// entryNames names each Store path by the Entry it belongs to.
func entryNames(env core.Env, paths []string) []string {
	targets := map[string]string{}
	if m, err := core.Load(env.Store); err == nil {
		for _, e := range m.Entries {
			targets[e.ID] = e.Target
		}
	}
	var details []string
	for _, p := range paths {
		id, _, _ := strings.Cut(p, "-")
		switch {
		case p == "squirrel.toml":
			details = append(details, "the Manifest")
		case targets[id] != "":
			details = append(details, targets[id])
		default:
			details = append(details, p)
		}
	}
	return details
}

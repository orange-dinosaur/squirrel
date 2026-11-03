package cli

import (
	"errors"
	"fmt"
	"io"

	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/spf13/cobra"
)

const quit core.Resolution = "quit"

func newApplyCmd() *cobra.Command {
	var backup, overwrite bool
	cmd := &cobra.Command{
		Use:   "apply [tags...]",
		Short: "Link every Entry for this OS, or those with any of the given Tags",

		ValidArgsFunction: completeTags,
		RunE: func(cmd *cobra.Command, tags []string) error {
			if backup && overwrite {
				return errors.New("--backup and --overwrite cannot be used together")
			}
			env, err := core.LoadEnv()
			if err != nil {
				return err
			}
			m, err := core.Load(env.Store)
			if err != nil {
				return err
			}
			plan, err := core.PlanApply(env, m, tags)
			if err != nil {
				return err
			}

			resolve := map[string]core.Resolution{}
			for _, c := range plan.Conflicts {
				switch {
				case backup:
					resolve[c.Entry.ID] = core.Backup
				case overwrite:
					resolve[c.Entry.ID] = core.Overwrite
				}
			}
			if len(plan.Conflicts) > 0 && !backup && !overwrite {
				if !stdinIsTerminal() {
					n := fmt.Sprintf("%d Conflicts need", len(plan.Conflicts))
					if len(plan.Conflicts) == 1 {
						n = "1 Conflict needs"
					}
					return fmt.Errorf("%s a decision: run apply in a terminal, or pass --backup or --overwrite", n)
				}
				for _, c := range plan.Conflicts {
					r, err := askResolution(c)
					if err != nil || r == quit {
						fmt.Fprintln(cmd.OutOrStdout(), "quit, nothing changed")
						return errQuiet
					}
					resolve[c.Entry.ID] = r
				}
			}

			res := core.ExecuteApply(env, plan, resolve)
			printApply(cmd.OutOrStdout(), env, plan, res, resolve)
			if !res.OK() {
				return errQuiet
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&backup, "backup", false, "resolve every Conflict by renaming it to <target>.squirrel-backup")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "resolve every Conflict by deleting what is in the way")
	return cmd
}

func askResolution(c core.Row) (core.Resolution, error) {
	var r core.Resolution
	err := huh.NewSelect[core.Resolution]().
		Title(fmt.Sprintf("%s is in the way (%s)", c.Entry.Target, c.State)).
		Options(
			huh.NewOption("Back it up to "+c.Entry.Target+".squirrel-backup", core.Backup),
			huh.NewOption("Overwrite it", core.Overwrite),
			huh.NewOption("Skip this Entry", core.Skip),
			huh.NewOption("Quit without changing anything", quit),
		).
		Value(&r).
		Run()
	return r, err
}

func printApply(w io.Writer, env core.Env, plan core.Plan, res core.Result, resolve map[string]core.Resolution) {
	for _, dir := range plan.Parents {
		if t, ok := env.Contract(dir); ok {
			fmt.Fprintln(w, "created", t)
		}
	}
	for _, e := range res.Linked {
		switch resolve[e.ID] {
		case core.Backup:
			fmt.Fprintf(w, "linked %s, old one saved as %s.squirrel-backup\n", e.Target, e.Target)
		case core.Overwrite:
			fmt.Fprintf(w, "linked %s, old one deleted\n", e.Target)
		default:
			fmt.Fprintln(w, "linked", e.Target)
		}
	}
	for _, e := range res.Unmanaged {
		fmt.Fprintln(w, "unmanaged", e.Target)
	}
	for _, s := range res.Skipped {
		fmt.Fprintf(w, "skipped %s: %s\n", s.Entry.Target, s.Reason)
	}
	if len(res.Linked)+len(res.Unmanaged)+len(res.Skipped) == 0 {
		fmt.Fprintln(w, "nothing to do")
		return
	}
	fmt.Fprintf(w, "%d linked, %d unmanaged, %d skipped\n", len(res.Linked), len(res.Unmanaged), len(res.Skipped))
}

package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

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
			return runApply(cmd.OutOrStdout(), tags, backup, overwrite)
		},
	}
	cmd.Flags().BoolVar(&backup, "backup", false, "resolve every Conflict by renaming it to <target>.squirrel-backup")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "resolve every Conflict by deleting what is in the way")
	return cmd
}

func runApply(w io.Writer, tags []string, backup, overwrite bool) error {
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
				printLines(w, line{noop, "quit, nothing changed", "", nil})
				return errQuiet
			}
			resolve[c.Entry.ID] = r
		}
	}

	res := core.ExecuteApply(env, plan, resolve)
	printApply(w, env, plan, res, resolve)
	if !res.OK() {
		return errQuiet
	}
	return nil
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
	var lines []line
	for _, dir := range plan.Parents {
		if t, ok := env.Contract(dir); ok {
			lines = append(lines, line{done, "created", t, nil})
		}
	}
	for _, e := range res.Linked {
		switch resolve[e.ID] {
		case core.Backup:
			lines = append(lines, line{done, "linked", e.Target, []string{"old one saved as " + e.Target + ".squirrel-backup"}})
		case core.Overwrite:
			lines = append(lines, line{warn, "linked", e.Target, []string{"old one deleted"}})
		default:
			lines = append(lines, line{done, "linked", e.Target, nil})
		}
	}
	for _, e := range res.Unmanaged {
		lines = append(lines, line{noop, "unmanaged", e.Target, nil})
	}
	for _, s := range res.Skipped {
		lines = append(lines, line{warn, "skipped", s.Entry.Target, []string{s.Reason}})
	}
	if len(res.Linked)+len(res.Unmanaged)+len(res.Skipped) == 0 {
		printLines(w, line{noop, "nothing to do", "", nil})
		return
	}
	printLines(w, lines...)
	fmt.Fprintln(w, applySummary(len(res.Linked), len(res.Unmanaged), len(res.Skipped)))
}

// applySummary reads like "2 linked · 1 skipped", leaving out zero counts.
func applySummary(linked, unmanaged, skipped int) string {
	var parts []string
	for _, c := range []struct {
		n    int
		verb string
	}{{linked, "linked"}, {unmanaged, "unmanaged"}, {skipped, "skipped"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.verb))
		}
	}
	return strings.Join(parts, " · ")
}

package cli

import (
	"slices"

	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/spf13/cobra"
)

type completeFunc = func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

const noFiles = cobra.ShellCompDirectiveNoFileComp

func loadForCompletion() (core.Env, *core.Manifest, bool) {
	env, err := core.LoadEnv()
	if err != nil {
		return env, nil, false
	}
	m, err := core.Load(env.Store)
	return env, m, err == nil
}

// completeTargets offers the targets of non-removed Entries for this OS.
func completeTargets(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	env, m, ok := loadForCompletion()
	if !ok {
		return nil, noFiles
	}
	var out []string
	for _, e := range m.Entries {
		if !e.Removed && e.AppliesOn(env.OS) {
			out = append(out, e.Target)
		}
	}
	return out, noFiles
}

// completeTags offers every Tag not already typed.
func completeTags(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	_, m, ok := loadForCompletion()
	if !ok {
		return nil, noFiles
	}
	var out []string
	for _, tc := range core.Tags(m) {
		if !slices.Contains(args, tc.Tag) {
			out = append(out, tc.Tag)
		}
	}
	return out, noFiles
}

// completeDeletableTags offers the Tags tags rm can delete, for its one argument.
func completeDeletableTags(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, noFiles
	}
	all, _ := completeTags(cmd, args, toComplete)
	return slices.DeleteFunc(all, func(t string) bool { return slices.Contains(core.OSTags, t) }), noFiles
}

// completeNewTags offers the Tags the Entry in args[0] doesn't carry yet, for tag.
func completeNewTags(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	all, _ := completeTags(cmd, args, toComplete)
	have, _ := completeEntryTags(cmd, args, toComplete)
	return slices.DeleteFunc(all, func(t string) bool { return slices.Contains(have, t) }), noFiles
}

// completeEntryTags offers the Tags the Entry in args[0] carries, for untag.
func completeEntryTags(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	env, m, ok := loadForCompletion()
	if !ok {
		return nil, noFiles
	}
	i, err := m.Find(env, args[0])
	if err != nil {
		return nil, noFiles
	}
	var out []string
	for _, t := range m.Entries[i].Tags {
		if !slices.Contains(args, t) {
			out = append(out, t)
		}
	}
	return out, noFiles
}

func completeNothing(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return nil, noFiles
}

// targetThen completes a target for the first argument and uses rest after it.
func targetThen(rest completeFunc) completeFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return completeTargets(cmd, args, toComplete)
		}
		return rest(cmd, args, toComplete)
	}
}

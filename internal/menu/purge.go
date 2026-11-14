package menu

import (
	"strings"

	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

// askPurge asks which Removed Entries to purge, all ticked, then whether every
// machine has caught up. It returns their IDs, or a note when nothing is to go.
func askPurge() ([]string, string, error) {
	_, m, err := core.Open()
	if err != nil {
		return nil, "", err
	}

	var cells [][]string
	var ids []string
	for _, e := range m.Entries {
		if e.Removed {
			cells = append(cells, []string{e.Target, e.ID, strings.Join(e.Tags, " ")})
			ids = append(ids, e.ID)
		}
	}
	var opts []huh.Option[string]
	for i, label := range columns(cells) {
		opts = append(opts, huh.NewOption(label, ids[i]).Selected(true))
	}
	if len(ids) == 0 {
		return nil, "no removed Entries to purge", nil
	}

	ok := false
	err = RunForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Removed Entries to purge").
				Description("their Store files are deleted for good").
				Options(opts...).
				Height(min(len(opts), 10)+2).
				Value(&ids),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Has every machine run apply since these were removed?").
				Description("A machine that hasn't caught up keeps a broken Link.").
				Value(&ok),
		).WithHideFunc(func() bool { return len(ids) == 0 }),
	)
	if err != nil {
		return nil, "", err
	}
	if len(ids) == 0 || !ok {
		return nil, "nothing purged", nil
	}
	return ids, "", nil
}

// hasRemoved reports whether the Manifest holds a Removed Entry. A Store that
// can't be read counts as none; the actions report that error themselves.
func hasRemoved() bool {
	_, m, err := core.Open()
	if err != nil {
		return false
	}
	for _, e := range m.Entries {
		if e.Removed {
			return true
		}
	}
	return false
}

package menu

import (
	"fmt"
	"slices"

	"charm.land/huh/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

// askDeleteTag picks a Tag that isn't an OS tag and, if live Entries carry
// it, asks whether to untag them. It returns the Tag and the untag answer, or
// a note to print instead when there is nothing to delete.
func askDeleteTag() (tag string, untag bool, note string, err error) {
	_, m, err := core.Open()
	if err != nil {
		return "", false, "", err
	}
	tags := slices.DeleteFunc(core.Tags(m), func(tc core.TagCount) bool {
		return slices.Contains(core.OSTags, tc.Tag)
	})
	if len(tags) == 0 {
		return "", false, "no Tags", nil
	}
	var opts []huh.Option[string]
	for _, tc := range tags {
		opts = append(opts, huh.NewOption(tc.Tag, tc.Tag))
	}
	err = RunForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Delete a tag").
			Options(opts...).
			Value(&tag),
	))
	if err != nil {
		return "", false, "", err
	}

	n := tags[slices.IndexFunc(tags, func(tc core.TagCount) bool { return tc.Tag == tag })].Count
	if n == 0 {
		return tag, false, "", nil
	}
	entries := "entries"
	if n == 1 {
		entries = "entry"
	}
	err = RunForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("%s is on %d %s. Untag them and delete?", tag, n, entries)).
			Value(&untag),
	))
	if err == nil && !untag {
		return "", false, "nothing changed", nil
	}
	return tag, untag, "", err
}

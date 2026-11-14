package menu

import (
	"errors"
	"io"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/orange-dinosaur/squirrel/internal/style"
)

// browse lists the Entries, offers Change tags or Remove on the one picked,
// and comes back to the list until Esc.
func browse(a Actions, w io.Writer) error {
	var id string
	all := false
	for first := true; ; first = false {
		env, m, err := core.Open()
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(m.Entries, func(e core.Entry) bool { return !e.Removed }) {
			if first {
				return errors.New("no Entries yet")
			}
			return nil
		}
		if id, all, err = pickEntry(env, m, id, all); err != nil {
			return err
		}
		e := m.Entries[slices.IndexFunc(m.Entries, func(e core.Entry) bool { return e.ID == id })]

		const changeTags, remove, back = "tags", "remove", ""
		action := changeTags
		err = RunForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title(e.Target).
				Description(strings.Join(e.Tags, " ")).
				Options(
					huh.NewOption("Change tags", changeTags),
					huh.NewOption("Remove", remove),
					huh.NewOption("Back", back),
				).
				Value(&action),
		))
		switch {
		case err != nil || action == back:
			err = nil // Esc here goes back to the list, not the main menu
		case action == changeTags:
			var tags []string
			if tags, err = askTags(m, e); err == nil {
				err = a.SetTags(w, id, tags)
			}
		case action == remove:
			var ok bool
			if ok, err = askRemove(env, e); err == nil && ok {
				next := neighbour(env, m, id, all)
				if err = a.Remove(w, id); err == nil {
					id = next
				}
			}
		}
		if err != nil && !errors.Is(err, huh.ErrUserAborted) {
			a.Error(w, err)
		}
	}
}

// columns joins each row's cells two spaces apart, padding every column but the last to its
// widest cell. Widths ignore ANSI codes, so badges line up.
func columns(rows [][]string) []string {
	var widths []int
	for _, r := range rows {
		for i, c := range r {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], lipgloss.Width(c))
		}
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		var b strings.Builder
		for j, c := range r {
			if j > 0 {
				b.WriteString("  ")
			}
			b.WriteString(c)
			if j < len(r)-1 {
				b.WriteString(strings.Repeat(" ", widths[j]-lipgloss.Width(c)))
			}
		}
		out[i] = strings.TrimRight(b.String(), " ")
	}
	return out
}

// listed reports whether pickEntry shows e.
func listed(env core.Env, e core.Entry, all bool) bool {
	return !e.Removed && (all || e.AppliesOn(env.OS))
}

// neighbour is the listed Entry after id, or before it if id is last.
func neighbour(env core.Env, m *core.Manifest, id string, all bool) string {
	var prev string
	seen := false
	for _, e := range m.Entries {
		switch {
		case e.ID == id:
			seen = true
		case !listed(env, e, all):
		case seen:
			return e.ID
		default:
			prev = e.ID
		}
	}
	return prev
}

// pickEntry asks for one non-removed Entry with the cursor on start, and
// returns its ID. It lists the Entries for this OS unless all is set, with an
// option to list every Entry instead, and returns whether it ended up doing so.
func pickEntry(env core.Env, m *core.Manifest, start string, all bool) (string, bool, error) {
	const showAll = ""
	for {
		var opts []huh.Option[string]
		id, found := start, false
		if !all {
			opts = append(opts, huh.NewOption("Show Entries for other OSes", showAll))
		}
		rows, err := core.Status(env, m, nil, all)
		if err != nil {
			return "", all, err
		}
		rows = slices.DeleteFunc(rows, func(r core.Row) bool { return !listed(env, r.Entry, all) })
		width := 0
		for _, r := range rows {
			width = max(width, len(r.State))
		}
		cells := make([][]string, len(rows))
		for i, r := range rows {
			found = found || r.Entry.ID == start
			badge := style.Badge(style.StateOutcome(r.State), string(r.State), width)
			cells[i] = []string{badge, r.Entry.Target, strings.Join(r.Entry.Tags, " ")}
		}
		for i, label := range columns(cells) {
			opts = append(opts, huh.NewOption(label, rows[i].Entry.ID))
		}
		if !found {
			id = opts[0].Value
			if !all && len(opts) > 1 {
				id = opts[1].Value // the first Entry, not "Show Entries for other OSes"
			}
		}

		err = RunForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Browse Entries").
				Options(opts...).
				Value(&id),
		))
		if err != nil || id != showAll {
			return id, all, err
		}
		all = true
	}
}

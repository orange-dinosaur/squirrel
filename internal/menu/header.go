package menu

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/orange-dinosaur/squirrel/internal/style"
)

// header is the line above the menu: "squirrel  linux · 12 Entries   2 NOT LINKED ".
// Without a readable Store it is just the name.
func header() string {
	env, err := core.LoadEnv()
	if err != nil {
		return headerLine("", nil)
	}
	m, err := core.Load(env.Store)
	if err != nil {
		return headerLine("", nil)
	}
	rows, err := core.Status(env, m, nil, false)
	if err != nil {
		return headerLine("", nil)
	}
	return headerLine(env.OS, rows)
}

func headerLine(osTag string, rows []core.Row) string {
	s := lipgloss.NewStyle().Foreground(style.Mauve()).Bold(true).Render("squirrel")
	if osTag == "" {
		return s
	}
	var entries, notLinked, conflict, missing int
	for _, r := range rows {
		switch r.State {
		case core.RemovedState:
			continue
		case core.RemovedStillLinked:
			notLinked++
			continue
		case core.NotLinked:
			notLinked++
		case core.ConflictFile, core.ConflictDir, core.ConflictLink:
			conflict++
		case core.MissingFromStore:
			missing++
		}
		entries++
	}
	noun := "Entries"
	if entries == 1 {
		noun = "Entry"
	}
	s += "  " + lipgloss.NewStyle().Foreground(style.Overlay()).Render(fmt.Sprintf("%s · %d %s", osTag, entries, noun))
	for _, b := range []struct {
		n    int
		o    style.Outcome
		word string
	}{{notLinked, style.Warn, "not linked"}, {conflict, style.Warn, "conflict"}, {missing, style.Fail, "missing"}} {
		if b.n > 0 {
			s += "  " + style.Badge(b.o, fmt.Sprintf("%d %s", b.n, b.word), 0)
		}
	}
	return s
}

package menu

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

func TestHeaderLine(t *testing.T) {
	var rows []core.Row
	for _, s := range []core.State{core.Linked, core.NotLinked, core.RemovedStillLinked,
		core.ConflictFile, core.ConflictLink, core.MissingFromStore, core.RemovedState} {
		rows = append(rows, core.Row{State: s})
	}
	cases := []struct {
		rows []core.Row
		want string
	}{
		{rows, "squirrel  linux · 5 Entries   2 NOT LINKED    2 CONFLICT    1 MISSING "},
		{rows[:1], "squirrel  linux · 1 Entry"},
		{nil, "squirrel  linux · 0 Entries"},
	}
	for _, c := range cases {
		if got := ansi.Strip(headerLine("linux", c.rows)); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
	if got := ansi.Strip(headerLine("", nil)); got != "squirrel" {
		t.Errorf("no Store: got %q", got)
	}
}

package cli

import (
	"bytes"
	"testing"

	"github.com/orange-dinosaur/squirrel/internal/core"
)

func TestPrintLinesPlain(t *testing.T) {
	var b bytes.Buffer
	printLines(&b,
		line{done, "linked", "~/.zshrc", nil},
		line{warn, "skipped", "~/.ssh/config", []string{"", "not a regular file"}},
		line{noop, "nothing to do", "", nil},
	)
	want := "LINKED   ~/.zshrc\n" +
		"SKIPPED  ~/.ssh/config  not a regular file\n" +
		"NOTE     nothing to do\n"
	if got := b.String(); got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestPrintStatusRowsPlain(t *testing.T) {
	row := func(target string, tags []string, id string, s core.State) core.Row {
		return core.Row{Entry: core.Entry{Target: target, Tags: tags, ID: id}, State: s}
	}
	var b bytes.Buffer
	printStatusRows(&b, []core.Row{
		row("~/.zshrc", []string{"linux", "shell"}, "562e17c6", core.Linked),
		row("~/.config/git/config", []string{"linux"}, "b7e157db", core.NotLinked),
		row("~/.vimrc", []string{"macos"}, "0a1b2c3d", core.OtherOS),
	})
	want := "TARGET                TAGS         ID\n" +
		"~/.zshrc              linux shell  562e17c6\n" +
		"~/.config/git/config  linux        b7e157db  not linked\n" +
		"~/.vimrc              macos        0a1b2c3d  other OS\n"
	if got := b.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestApplySummary(t *testing.T) {
	for _, c := range []struct {
		linked, unmanaged, skipped int
		want                       string
	}{
		{2, 0, 1, "2 linked · 1 skipped"},
		{0, 3, 0, "3 unmanaged"},
		{1, 1, 1, "1 linked · 1 unmanaged · 1 skipped"},
	} {
		if got := applySummary(c.linked, c.unmanaged, c.skipped); got != c.want {
			t.Errorf("applySummary(%d, %d, %d) = %q, want %q", c.linked, c.unmanaged, c.skipped, got, c.want)
		}
	}
}

package menu

import (
	"slices"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/orange-dinosaur/squirrel/internal/style"
)

func TestColumns(t *testing.T) {
	got := columns([][]string{
		{style.Badge(style.Done, "linked", 10), "~/.zshrc", "linux shell"},
		{style.Badge(style.Warn, "not linked", 10), "~/.config/nvim", ""},
	})
	for i := range got {
		got[i] = ansi.Strip(got[i])
	}
	want := []string{
		" LINKED       ~/.zshrc        linux shell",
		" NOT LINKED   ~/.config/nvim",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestNeighbour(t *testing.T) {
	env := core.Env{OS: "linux"}
	m := &core.Manifest{Entries: []core.Entry{
		{ID: "a"},
		{ID: "gone", Removed: true},
		{ID: "mac", Tags: []string{"macos"}},
		{ID: "b"},
		{ID: "c"},
	}}
	for _, tc := range []struct {
		id   string
		all  bool
		want string
	}{
		{"a", false, "b"},  // skips Removed and other-OS Entries
		{"a", true, "mac"}, // other-OS Entries count when all are listed
		{"c", false, "b"},  // last falls back to the one before
		{"b", false, "c"},
	} {
		if got := neighbour(env, m, tc.id, tc.all); got != tc.want {
			t.Errorf("neighbour(%s, all=%v) = %q, want %q", tc.id, tc.all, got, tc.want)
		}
	}
	if got := neighbour(env, &core.Manifest{Entries: []core.Entry{{ID: "a"}}}, "a", false); got != "" {
		t.Errorf("only Entry: got %q, want empty", got)
	}
}

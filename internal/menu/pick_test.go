package menu

import (
	"testing"

	"github.com/orange-dinosaur/squirrel/internal/core"
)

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

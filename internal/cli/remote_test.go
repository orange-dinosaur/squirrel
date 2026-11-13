package cli

import (
	"testing"

	"github.com/orange-dinosaur/squirrel/internal/core"
)

func TestSyncStatus(t *testing.T) {
	for want, s := range map[string]core.RemoteStatus{
		"in sync":                             {},
		"changes to push":                     {Dirty: []string{"squirrel.toml"}},
		"2 commits to push":                   {Ahead: 2, Dirty: []string{"squirrel.toml"}},
		"1 commit to pull":                    {Behind: 1},
		"1 commit to push, 3 commits to pull": {Ahead: 1, Behind: 3},
	} {
		if got := syncStatus(s); got != want {
			t.Errorf("syncStatus(%+v) = %q, want %q", s, got, want)
		}
	}
}

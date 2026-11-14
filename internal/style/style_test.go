package style

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/orange-dinosaur/squirrel/internal/core"
)

func TestBadge(t *testing.T) {
	if got := ansi.Strip(Badge(Done, "added", 7)); got != " ADDED   " {
		t.Errorf("Badge = %q", got)
	}
	if got := StateOutcome(core.Linked); got != Done {
		t.Errorf("StateOutcome(Linked) = %v", got)
	}
	if got := StateOutcome(core.ConflictFile); got != Warn {
		t.Errorf("StateOutcome(ConflictFile) = %v", got)
	}
}

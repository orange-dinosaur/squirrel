package menu

import (
	"reflect"
	"testing"

	"github.com/orange-dinosaur/squirrel/internal/core"
)

func TestTagChoices(t *testing.T) {
	m := &core.Manifest{Tags: []string{"work"}}
	if got, want := tagChoices(m), []string{"linux", "macos", "work"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tagChoices = %q, want %q", got, want)
	}
}

package cli

import (
	"bytes"
	"testing"
)

func TestPrintLinesPlain(t *testing.T) {
	var b bytes.Buffer
	printLines(&b,
		line{done, "linked", "~/.zshrc", nil},
		line{warn, "skipped", "~/.ssh/config", []string{"", "not a regular file"}},
		line{noop, "nothing to do", "", nil},
	)
	want := "linked         ~/.zshrc\n" +
		"skipped        ~/.ssh/config  not a regular file\n" +
		"nothing to do\n"
	if got := b.String(); got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
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

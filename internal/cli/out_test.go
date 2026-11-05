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

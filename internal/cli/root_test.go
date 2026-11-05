package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestPrintErr(t *testing.T) {
	for args, want := range map[string]string{
		"add":         "accepts 1 arg(s), received 0\n  try squirrel add --help\n",
		"add --bogus": "unknown flag: --bogus\n  try squirrel add --help\n",
		"bogus":       "unknown command \"bogus\" for \"squirrel\"\n  try squirrel --help\n",
		"tags rm":     "accepts 1 arg(s), received 0\n  try squirrel tags rm --help\n",
	} {
		root := newRootCmd()
		root.SetArgs(strings.Fields(args))
		root.SilenceErrors, root.SilenceUsage = true, true
		var b bytes.Buffer
		printErr(&b, root.Execute())
		if b.String() != want {
			t.Errorf("%s: got %q, want %q", args, b.String(), want)
		}
	}

	var b bytes.Buffer
	printErr(&b, errors.New("/nope is not inside HOME"))
	if want := "/nope is not inside HOME\n"; b.String() != want {
		t.Errorf("got %q, want %q", b.String(), want)
	}
}

func TestCheckOS(t *testing.T) {
	for goos, ok := range map[string]bool{"darwin": true, "linux": true, "windows": false, "freebsd": false} {
		if err := checkOS(goos); (err == nil) != ok {
			t.Errorf("checkOS(%q) = %v, want ok=%v", goos, err, ok)
		}
	}
}

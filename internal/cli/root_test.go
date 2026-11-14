package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestPrintErr(t *testing.T) {
	for args, want := range map[string]string{
		"add":         "ERROR  add needs a path when not run in a terminal\n  try squirrel add --help\n",
		"add a b":     "ERROR  usage: squirrel add [path]\n  try squirrel add --help\n",
		"add --bogus": "ERROR  unknown flag: --bogus\n  try squirrel add --help\n",
		"bogus":       "ERROR  unknown command \"bogus\" for \"squirrel\"\n  try squirrel --help\n",
		"tags rm":     "ERROR  usage: squirrel tags remove <tag>\n  try squirrel tags remove --help\n",
		"remove":      "ERROR  usage: squirrel remove <path|id>\n  try squirrel remove --help\n",
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
	if want := "ERROR  /nope is not inside HOME\n"; b.String() != want {
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

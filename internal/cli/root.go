package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/x/term"
	"github.com/orange-dinosaur/squirrel/internal/core"
	"github.com/orange-dinosaur/squirrel/internal/menu"
	"github.com/spf13/cobra"
)

var errUnsupportedOS = errors.New("squirrel supports macOS and Linux only")

// errQuiet makes squirrel exit 1 without printing anything, for commands whose
// output already says what is wrong.
var errQuiet = errors.New("quiet failure")

func checkOS(goos string) error {
	if goos != "darwin" && goos != "linux" {
		return errUnsupportedOS
	}
	return nil
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "squirrel",
		Short: "Keep your configs and dotfiles in one Store and link them into your home directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinIsTerminal() || !term.IsTerminal(os.Stdout.Fd()) {
				return cmd.Help()
			}
			return menu.Run(menu.Actions{
				Add: runAdd,
				Apply: func(w io.Writer, tags []string) error {
					return quietOK(runApply(w, tags, false, false))
				},
				Status: func(w io.Writer) error {
					_, err := printStatus(w, nil, false)
					return err
				},
				Remove:    runRemove,
				SetTags:   runSetTags,
				Tags:      printTags,
				DeleteTag: runDeleteTag,
				Purge:     func(w io.Writer, ids []string) error { return runPurge(w, ids, true) },
				Note:      func(w io.Writer, msg string) { printLines(w, line{noop, msg, "", nil}) },
				Error:     printErr,
				RemoteAdd: func(w io.Writer, url string) error {
					return quietOK(runRemoteAdd(w, url, false, false))
				},
				Push:         func(w io.Writer) error { return quietOK(runPush(w, false, false)) },
				Pull:         func(w io.Writer) error { return quietOK(runPull(w, false, false)) },
				ShowRemote:   runRemote,
				RemoteRemove: runRemoteRemove,
				RemoteSetURL: runRemoteSetURL,
			})
		},
	}
	root.AddCommand(newAddCmd(), newStatusCmd(), newApplyCmd(), newRemoveCmd(), newPurgeCmd(),
		newRetagCmd("tag", "Add Tags to an Entry", core.Tag, completeNewTags),
		newRetagCmd("untag", "Drop Tags from an Entry", core.Untag, completeEntryTags),
		newTagsCmd(), newRemoteCmd(), newPullCmd(), newPushCmd(), newUpgradeCmd())

	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return usageError(c, err)
	})
	cmds := []*cobra.Command{root}
	for i := 0; i < len(cmds); i++ {
		cmds = append(cmds, cmds[i].Commands()...)
	}
	for _, c := range cmds {
		if args := c.Args; args != nil {
			c.Args = func(c *cobra.Command, a []string) error {
				err := args(c, a)
				if err != nil && !c.HasParent() {
					return usageError(c, err) // cobra's "unknown command" names the typo
				}
				if err != nil {
					return usageError(c, fmt.Errorf("usage: %s", strings.TrimSuffix(c.UseLine(), " [flags]")))
				}
				return nil
			}
		}
	}
	return root
}

// quietOK drops errQuiet: in the menu, a Quit at a Conflict and a partial apply have already
// printed why.
func quietOK(err error) error {
	if errors.Is(err, errQuiet) {
		return nil
	}
	return err
}

func stdinIsTerminal() bool {
	return term.IsTerminal(os.Stdin.Fd()) // a char-device check also passes /dev/null
}

// Set at release build time with -ldflags -X. Empty in a source build.
var (
	version string
	commit  string
)

// Execute runs the squirrel command line. Errors are already printed when it returns.
func Execute(ctx context.Context) error {
	if err := checkOS(runtime.GOOS); err != nil {
		printErr(os.Stderr, err)
		return err
	}
	opts := []fang.Option{fang.WithErrorHandler(printError), fang.WithColorSchemeFunc(helpColors)}
	if version != "" {
		opts = append(opts, fang.WithVersion(version), fang.WithCommit(commit))
	}
	return fang.Execute(ctx, newRootCmd(), opts...)
}

func printError(w io.Writer, _ fang.Styles, err error) {
	if !errors.Is(err, errQuiet) {
		printErr(w, err)
	}
}

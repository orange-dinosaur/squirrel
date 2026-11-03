package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/charmbracelet/fang"
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
			if !stdinIsTerminal() {
				return cmd.Help()
			}
			return menu.Run(menu.Actions{
				Add: func(path string, tags []string) error { return runAdd(os.Stdout, path, tags) },
				Apply: func(tags []string) error {
					// Quit at a Conflict and a partial apply have already printed why.
					if err := runApply(os.Stdout, tags, false, false); !errors.Is(err, errQuiet) {
						return err
					}
					return nil
				},
				Status: func() error {
					_, err := printStatus(os.Stdout, nil, false)
					return err
				},
				Remove:  func(id string) error { return runRemove(os.Stdout, id) },
				SetTags: func(id string, tags []string) error { return runSetTags(os.Stdout, id, tags) },
				Tags:    func() error { return printTags(os.Stdout) },
			})
		},
	}
	root.AddCommand(newAddCmd(), newStatusCmd(), newApplyCmd(), newRemoveCmd(),
		newRetagCmd("tag", "Add Tags to an Entry", core.Tag, completeNewTags),
		newRetagCmd("untag", "Drop Tags from an Entry", core.Untag, completeEntryTags),
		newTagsCmd())
	return root
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// Execute runs the squirrel command line. Errors are already printed when it returns.
func Execute(ctx context.Context) error {
	if err := checkOS(runtime.GOOS); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return fang.Execute(ctx, newRootCmd(), fang.WithErrorHandler(printError))
}

// printError keeps Fang's error style but not its capitalized first word,
// which turns ~/.zshrc into ~/.Zshrc.
func printError(w io.Writer, styles fang.Styles, err error) {
	if errors.Is(err, errQuiet) {
		return
	}
	styles.ErrorText = styles.ErrorText.UnsetTransform()
	fang.DefaultErrorHandler(w, styles, err)
}

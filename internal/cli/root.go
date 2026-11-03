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
	}
	root.AddCommand(newAddCmd(), newStatusCmd(), newApplyCmd(), newRemoveCmd(),
		newRetagCmd("tag", "Add Tags to an Entry", core.Tag),
		newRetagCmd("untag", "Drop Tags from an Entry", core.Untag),
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

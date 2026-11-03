package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

var errUnsupportedOS = errors.New("squirrel supports macOS and Linux only")

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
	root.AddCommand(newAddCmd())
	return root
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
	styles.ErrorText = styles.ErrorText.UnsetTransform()
	fang.DefaultErrorHandler(w, styles, err)
}

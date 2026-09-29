package cli

import (
	"context"
	"errors"
	"fmt"
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
	return &cobra.Command{
		Use:   "squirrel",
		Short: "Keep your configs and dotfiles in one Store and link them into your home directory",
	}
}

// Execute runs the squirrel command line. Errors are already printed when it returns.
func Execute(ctx context.Context) error {
	if err := checkOS(runtime.GOOS); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return fang.Execute(ctx, newRootCmd())
}

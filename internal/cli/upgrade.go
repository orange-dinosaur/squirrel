package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strings"

	"github.com/orange-dinosaur/squirrel/internal/upgrade"
	"github.com/spf13/cobra"
)

// Tests point these at a fake GitHub and a binary in a temp folder.
var (
	releases   = upgrade.GitHub()
	executable = os.Executable
)

func newUpgradeCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "upgrade [version]",
		Short: "Replace squirrel with the latest Release, or with the version you name",
		Long: "Replace squirrel with the latest Release, or with the version you name, even an older one.\n" +
			"The download is checked against the Release's checksums before it replaces anything.",
		Example: "  squirrel upgrade\n  squirrel upgrade 0.1.0",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			want := ""
			if len(args) == 1 {
				want = args[0]
			}
			return runUpgrade(cmd.Context(), cmd.OutOrStdout(), want, force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "replace squirrel even if it was built from source")
	return cmd
}

func runUpgrade(ctx context.Context, w io.Writer, want string, force bool) error {
	if version == "" && !force {
		target := "the latest release"
		if want != "" {
			target = upgrade.Tag(want)
		}
		return fmt.Errorf("squirrel was built from source (unknown); pass --force to replace it with %s", target)
	}
	tag := upgrade.Tag(want)
	if want == "" {
		var err error
		if tag, err = releases.Latest(ctx); err != nil {
			return err
		}
	}
	current := cmp.Or(version, "unknown")
	if version != "" && upgrade.Tag(version) == tag {
		msg := "is the latest"
		if want != "" {
			msg = "is already installed"
		}
		printLines(w, line{noop, "squirrel " + current + " " + msg, "", nil})
		return nil
	}
	bin, err := releases.Fetch(ctx, tag, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	path, err := executable()
	if err != nil {
		return fmt.Errorf("can't find the running squirrel: %w", err)
	}
	if err := upgrade.Replace(path, bin); errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("can't replace %s: permission denied, run with sudo", path)
	} else if err != nil {
		return fmt.Errorf("can't replace %s: %w", path, err)
	}
	printLines(w, line{done, "upgraded", "squirrel " + current + " → " + strings.TrimPrefix(tag, "v"), nil})
	return nil
}

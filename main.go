package main

import (
	"context"
	"os"

	"github.com/orange-dinosaur/squirrel/internal/cli"
)

func main() {
	if err := cli.Execute(context.Background()); err != nil {
		os.Exit(1)
	}
}

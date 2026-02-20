package main

import (
	"os"

	"github.com/Perttulands/senate/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args))
}

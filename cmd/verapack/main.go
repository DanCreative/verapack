package main

import (
	"os"

	"github.com/DanCreative/verapack/internal/verapack"
	cli "github.com/urfave/cli/v2"
)

var (
	Version string
)

func main() {
	check := func(err error) {
		if err != nil {
			os.Exit(1)
		}
	}

	cli.VersionPrinter = verapack.VersionPrinter

	app := verapack.NewApp()
	app.Version = Version

	err := app.Run(os.Args)
	check(err)
}

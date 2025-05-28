package main

import (
	"os"

	"github.com/DanCreative/verapack/internal/verapack"
	cli "github.com/urfave/cli/v2"
)

var (
	Version        string
	UpdateApp      func(*cli.App)
	VersionPrinter func(cCtx *cli.Context)
)

func main() {
	check := func(err error) {
		if err != nil {
			os.Exit(1)
		}
	}

	if VersionPrinter == nil {
		VersionPrinter = verapack.VersionPrinter
	}
	cli.VersionPrinter = VersionPrinter

	app := verapack.NewApp()
	app.Version = Version

	if UpdateApp != nil {
		UpdateApp(app)
	}

	err := app.Run(os.Args)
	check(err)
}

package main

import (
	"fmt"
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
			fmt.Println("### The application encountered below errors: ###")
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// cli.VersionFlag = &cli.BoolFlag{
	// 	Name:    "version",
	// 	Aliases: []string{"v"},
	// 	Usage:   "print the version",
	// }
	cli.VersionPrinter = verapack.VersionPrinter

	app := verapack.NewApp()
	app.Version = Version

	err := app.Run(os.Args)
	check(err)
}

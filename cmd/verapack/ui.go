// go: build ui

// This file with this build constraint will be used for UI testing and demos.
package main

import (
	"github.com/DanCreative/verapack/internal/verapack"
	cli "github.com/urfave/cli/v2"
)

func init() {
	UpdateApp = func(a *cli.App) {
		a.Flags = append(a.Flags, &cli.PathFlag{
			Name:      "config-file",
			Aliases:   []string{"c"},
			TakesFile: true,
			Required:  false,
		}, &cli.PathFlag{
			Name:      "version-file",
			Aliases:   []string{"y"},
			TakesFile: true,
			Required:  false,
		})

		// Setup
		a.Commands[0].Action = verapack.Setup_ui

		// Scan
		// 	Sandbox
		a.Commands[1].Subcommands[0].Action = verapack.Sandbox_ui

		// 	Policy
		a.Commands[1].Subcommands[1].Action = verapack.Policy_ui
		//  Promote
		a.Commands[1].Subcommands[2].Action = verapack.Promote_ui

		// Update
		a.Commands[2].Action = verapack.Update_ui

		// Credentials
		//  Refresh
		a.Commands[3].Subcommands[0].Action = verapack.RefreshCredentials_ui
		//  Configure
		a.Commands[3].Subcommands[1].Action = verapack.ConfigureCredentials_ui
	}

	VersionPrinter = verapack.VersionPrinter_ui
}

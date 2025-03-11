package verapack

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/urfave/cli/v2"
)

func NewApp() *cli.App {
	return &cli.App{
		Name:  "verapack",
		Usage: "combines and wraps the power of the Veracode CLI and the Java wrapper to provide an easy way to package and scan multiple applications.",
		Commands: []*cli.Command{
			{
				Name:    "setup",
				Usage:   "Configure config files and install the Java wrapper and Veracode CLI if they are not already installed. Use the update flag to force update dependencies.",
				Action:  setup,
				Aliases: []string{"s"},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "update",
						Usage:   "force update dependencies",
						Aliases: []string{"u"},
					},
				},
			},
			{
				Name:    "go",
				Usage:   "Package and/or Scan all applications in the config file.",
				Action:  run,
				Aliases: []string{"r"},
			},
		},
	}
}

func setup(cCtx *cli.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	fmt.Println("[Config File]")

	appDir, err := SetupConfig(homeDir)
	if err != nil {
		return err
	}

	fmt.Println("[Configuring Credentials]")

	err = SetupCredentials(homeDir)
	if err != nil {
		return err
	}

	fmt.Println("[Installing Dependencies]")

	packagerPath := getPackagerLocation()

	// if the veracode folder path does not exist or the update flag is passed,
	// run the installation.
	if _, err = os.Stat(packagerPath); err != nil || cCtx.Bool("update") {
		// TODO: Currently only using the partial installation.
		// If required, will add a path for full installation.
		err = InstallPackager(false, packagerPath)
		if err != nil {
			return err
		}
		fmt.Println("\tSuccessfully installed the Veracode Packager")
	} else {
		fmt.Println("\tSkipping Veracode Packager")
	}

	// if VeracodeJavaAPI.jar does not exist or the update flag is passed,
	// run the installation.
	if _, err = os.Stat(filepath.Join(appDir, "VeracodeJavaAPI.jar")); err != nil || cCtx.Bool("update") {
		err = InstallUploader(appDir)
		if err != nil {
			return err
		}
		fmt.Println("\tSuccessfully installed the Veracode Uploader")
	} else {
		fmt.Println("\tSkipping Veracode Uploader")
	}

	fmt.Println("[Success]")
	return nil
}

func run(cCtx *cli.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	uploaderPath := filepath.Join(homeDir, ".veracode", "verapack", "VeracodeJavaAPI.jar")

	c, err := ReadConfig(filepath.Join(homeDir, ".veracode", "verapack", "config.yaml"))
	if err != nil {
		return err
	}

	path := os.Getenv("PATH")
	os.Setenv("PATH", path+";"+getPackagerLocation())

	reportCard := NewReportCard()
	reportCard.addApplications(c.Applications)
	reportCard.Start()

	// Running Packaging and scanning goes here
	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	for k, app := range c.Applications {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = packageAndUploadApplication(uploaderPath, app, k, reportCard)
			errCh <- err
		}()
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	var errs error
	for err := range errCh {
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	reportCard.Stop()

	if errs != nil {
		return errs
	}
	// Running Packaging and scanning goes here

	return nil
}

func VersionPrinter(cCtx *cli.Context) {
	var vUploader string
	homeDir, err := os.UserHomeDir()
	if err != nil {
		vUploader = "Can't access Uploader"
	}
	vUploader = versionUploader(filepath.Join(homeDir, ".veracode", "verapack", "VeracodeJavaAPI.jar"))

	path := os.Getenv("PATH")
	os.Setenv("PATH", path+";"+getPackagerLocation())

	fmt.Printf("%s version %s\n%s%s", cCtx.App.Name, cCtx.App.Version, versionPackager(), vUploader)
}

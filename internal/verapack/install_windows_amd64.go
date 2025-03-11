package verapack

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func InstallCli() error {
	cmd := exec.Command("powershell", "-nologo", "-noprofile")

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		fmt.Fprintln(stdin, `Set-ExecutionPolicy AllSigned -Scope Process -Force;$ProgressPreference = "silentlyContinue"; iex ((New-Object System.Net.WebClient).DownloadString('https://tools.veracode.com/veracode-cli/install.ps1'))`)
		fmt.Fprintln(stdin, "exit")
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

// getPackagerFileName takes the latest version of the cli, and returns the full
// file name containing the version, os and architecture. It also returns the
// archive file extension.
//
// NOTE: This is the windows zip implementation.
func getPackagerFileName(version string) (string, string) {
	return "veracode-cli_" + version + "_windows_x86.zip", "zip"
	// return "veracode-cli_" + version + "_windows_x86.tar.gz"
}

// extractPackagerArchive takes a source file path and a destination dir path and
// decompresses the archive to the destination. It also flattens the filepaths from
// the source archive.
//
// NOTE: This is the windows zip implementation.
//
// Credit to: https://gist.github.com/paulerickson/6d8650947ee4e3f3dbcc28fde10eaae7
func extractPackagerArchive(source, destination string) error {
	archive, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer archive.Close()

	err = os.MkdirAll(destination, os.ModePerm)
	if err != nil {
		return err
	}

	for _, file := range archive.Reader.File {
		reader, err := file.Open()
		if err != nil {
			return err
		}
		defer reader.Close()
		// calling filepath.Base() to flatten file structure into a single depth folder.
		path := filepath.Join(destination, filepath.Base(file.Name))

		// If file is _supposed_ to be a directory, we're done
		if file.FileInfo().IsDir() {
			continue
		}

		// Remove file if it already exists; no problem if it doesn't; other cases can error out below
		_ = os.Remove(path)

		// and create the actual file.  This ensures that the parent directories exist!
		// An archive may have a single file with a nested path, rather than a file for each parent dir
		writer, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		defer writer.Close()
		_, err = io.Copy(writer, reader)
		if err != nil {
			return err
		}
	}
	return nil
}

// fullPackagerInstall runs the Powershell installation script.
//
// NOTE: This is the windows x86_64 implementation.
func fullPackagerInstall() error {
	cmd := exec.Command("powershell", "-nologo", "-noprofile")

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		fmt.Fprintln(stdin, `Set-ExecutionPolicy AllSigned -Scope Process -Force;$ProgressPreference = "silentlyContinue"; iex ((New-Object System.Net.WebClient).DownloadString('https://tools.veracode.com/veracode-cli/install.ps1'))`)
		fmt.Fprintln(stdin, "exit")
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

// getPackagerLocation gets the directory path of the packager executable.
//
// NOTE: This is the windows implementation.
func getPackagerLocation() string {
	return filepath.Join(os.Getenv("AppData"), "veracode")
}

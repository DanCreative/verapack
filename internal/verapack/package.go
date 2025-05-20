package verapack

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/runeutil"
)

var (
	errNoArtifacts  = errors.New("no artefacts created")
	errPackagingErr = errors.New("packaging error")
)

func packageOptionsToArgs(options Options) []string {
	r := []string{"package"}

	if *options.Verbose {
		r = append(r, "-v")
	}

	if *options.Trust {
		r = append(r, "-a")
	}

	if options.Type != "" {
		r = append(r, "-t", string(options.Type))
	}

	if options.OutputDir != "" {
		r = append(r, "-o", options.OutputDir)
	}

	if options.PackageSource != "" {
		r = append(r, "-s", options.PackageSource)
	}

	return r
}

// PackageApplication runs the Veracode auto-packager using the provided PackageOptions,
// and returns a list of the artefact paths and any errors encountered.
//
// writer can optionally be provided to write log output to an additional location.
func PackageApplication(options Options, writer io.Writer) ([]string, string, error) {
	path, err := exec.LookPath("veracode")
	if err != nil {
		return nil, "", err
	}

	cmd := exec.Command(path, packageOptionsToArgs(options)...)

	var outBuffer bytes.Buffer

	if writer != nil {
		cmd.Stderr = io.MultiWriter(&outBuffer, writer)
		cmd.Stdout = io.MultiWriter(&outBuffer, writer)
	} else {
		cmd.Stderr, cmd.Stdout = &outBuffer, &outBuffer
	}

	err = cmd.Run()

	sanitizer := runeutil.NewSanitizer()
	out := string(sanitizer.Sanitize([]rune(outBuffer.String())))

	if err != nil {
		return nil, err.Error() + "\n" + out, errPackagingErr
	}

	artefacts, err := getArtefactPath(options.OutputDir)
	if err != nil {
		return nil, err.Error() + "\n" + out, err
	}

	return artefacts, out, nil
}

func versionPackager() string {
	path, err := exec.LookPath("veracode")
	if err != nil {
		return "Packager not installed"
	}

	cmd := exec.Command(path, "version")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "Packager not installed"
	}
	return string(out)
}

// getArtefactPath takes a directory string and returns a []string of the artefact paths
// in that directory.
func getArtefactPath(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	if len(entries) < 1 {
		return nil, errNoArtifacts
	}

	r := make([]string, len(entries))

	for i, entry := range entries {
		r[i] = filepath.Join(dirPath, entry.Name())
	}
	return r, nil
}

// NOTE: baseDir is the temp dir + app folder
// Creates the path and returns said path
func createAppPackagingOutputDir(appName string) (string, error) {
	path := filepath.Join(os.TempDir(), "verapack", appName, strconv.FormatInt(time.Now().Unix(), 10))
	err := os.MkdirAll(path, 0600)
	return path, err
}

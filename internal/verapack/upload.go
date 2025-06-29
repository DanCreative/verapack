package verapack

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strconv"

	"github.com/charmbracelet/bubbles/runeutil"
)

var (
	errScanningErr = errors.New("scanning error")
)

func uploadOptionsToArgs(options Options) []string {
	// TODO: Change for other non-windows platforms
	// Forces the jar to use the Windows trust store.
	// This is to fix a Java sun.security.provider.certpath.SunCertPathBuilderException
	// when running the application behind a corporate proxy with its own cert.

	r := make([]string, 0, 30) // capacity is set to 1.5x max number of possible options. Remember to change when adding options.

	r = append(r,
		"-Djavax.net.ssl.trustStoreType=WINDOWS-ROOT",
		"-jar", options.UploaderFilePath,
		"-action", "UploadAndScan",
		"-appname", options.AppName, // Required field
		"-version", options.Version, // Required field
		"-createprofile", strconv.FormatBool(*options.CreateProfile), // Required field
	)

	// Required fields
	for _, filepath := range options.ArtefactPaths {
		r = append(r, "-filepath", filepath)
	}

	// Optional fields
	if options.ScanType == ScanTypeSandbox {
		r = append(r, "-sandboxid", strconv.Itoa(options.SandboxId))
	}

	if *options.Verbose {
		r = append(r, "-debug")
	}

	return r
}

func UploadAndScanApplication(options Options, writer io.Writer) (string, error) {
	path, err := exec.LookPath("java")
	if err != nil {
		return err.Error(), err
	}

	cmd := exec.Command(path, uploadOptionsToArgs(options)...)

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
		return err.Error() + "\n" + out, errScanningErr
	}

	return out, nil
}

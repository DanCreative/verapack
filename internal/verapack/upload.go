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
	r := []string{"-Djavax.net.ssl.trustStoreType=WINDOWS-ROOT", "-jar", options.UploaderFilePath, "-action", "UploadAndScan"}

	// Required fields
	for _, filepath := range options.ArtefactPaths {
		r = append(r, "-filepath", filepath)
	}

	r = append(r,
		"-appname", options.AppName,
		"-version", options.Version,
		"-createprofile", strconv.FormatBool(*options.CreateProfile),
	)

	// Optional fields
	if options.ScanTimeout != 0 {
		r = append(r, "-scantimeout", strconv.Itoa(options.ScanTimeout))
	}
	if options.ScanPollingInterval != 0 {
		r = append(r, "-scanpollinginterval", strconv.Itoa(options.ScanPollingInterval))
	}

	if options.ScanType == ScanTypeSandbox {
		r = append(r, "-sandboxid", strconv.Itoa(options.SandboxId))
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

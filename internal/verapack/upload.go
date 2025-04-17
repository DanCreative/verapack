package verapack

import (
	"errors"
	"os/exec"
	"strconv"
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
		"-createprofile", strconv.FormatBool(options.CreateProfile),
	)

	// Optional fields
	if options.ScanTimeout != 0 {
		r = append(r, "-scantimeout", strconv.Itoa(options.ScanTimeout))
	}
	if options.ScanPollingInterval != 0 {
		r = append(r, "-scanpollinginterval", strconv.Itoa(options.ScanPollingInterval))
	}

	return r
}

func UploadAndScanApplication(options Options) (string, error) {
	path, err := exec.LookPath("java")
	if err != nil {
		return err.Error(), err
	}

	cmd := exec.Command(path, uploadOptionsToArgs(options)...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), errScanningErr
	}

	return string(out), nil
}

func versionUploader(uploaderPath string) string {
	path, err := exec.LookPath("java")
	if err != nil {
		return "Java and Uploader not installed"
	}

	cmd := exec.Command(path, "-jar", uploaderPath, "-wrapperversion")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "Uploader not installed"
	} else {
		return string(out)
	}
}

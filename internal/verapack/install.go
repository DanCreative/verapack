package verapack

import (
	"bytes"
	_ "embed"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

//go:embed vosp-api-wrappers-java-*.jar
var uploaderFileBytes []byte

// InstallUploader writes the embedded Veracode API Java wrapper jar to the provided path
// so that it can be used in a system call.
//
// I made the decision to embed the jar instead of downloading the latest version from Maven,
// because I couldn't see a way of doing it without adding unnecessary complexity.
func InstallUploader(dirpath string) error {
	err := os.MkdirAll(dirpath, os.ModePerm)
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dirpath, "VeracodeJavaAPI.jar"))
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, bytes.NewReader(uploaderFileBytes)); err != nil {
		return err
	}
	return nil
}

// InstallPackager installs the packager to the user's application directory. If shouldFullInstall
// is true, it runs fullPackagerInstall otherwise it runs partialPackagerInstall. Check the docs for
// those functions for more information.
//
// NOTE: This function is OS/ARCH agnostic, but the functions it calls are not. In order to build this
// application for different environment, implement the required functions for that environment/tech.
func InstallPackager(shouldFullyInstall bool, dirPath string) error {
	if shouldFullyInstall {
		return fullPackagerInstall()
	} else {
		return partialPackagerInstall(dirPath)
	}
}

// partialPackagerInstall downloads and extracts the packager to the user's application directory.
// This function is used if the user installs without admin permissions. The only difference between
// this installation type and the full installation, is that this path does not set the env variables.
//
// On windows the directory is: %AppData%\veracode
//
// NOTE: This function is OS/ARCH agnostic, but the functions it calls are not. In order to build this
// application for different environment, implement the required functions for that environment/tech.
func partialPackagerInstall(dirPath string) error {
	jar, _ := cookiejar.New(&cookiejar.Options{})
	client := &http.Client{
		Jar: jar,
	}
	baseURL, _ := url.Parse("https://tools.veracode.com/veracode-cli")

	fileVersion, err := getLatestPackagerVersion(client, baseURL)
	if err != nil {
		return err
	}
	fileName, ext := getPackagerFileName(fileVersion)

	downloadedPath, err := downloadPackagerArchive(client, baseURL, ext, fileName)
	if err != nil {
		log.Fatal(err)
	}

	defer os.Remove(downloadedPath)

	err = extractPackagerArchive(downloadedPath, dirPath)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

// getLatestPackagerVersion gets the latest version of the packager.
//
// Typical Dev comment;)
func getLatestPackagerVersion(client *http.Client, baseURL *url.URL) (string, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL.JoinPath("/LATEST_VERSION").String(), nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(bytes)), nil
}

// downloadPackagerArchive streams the archive from the remote source to
// a temporarily local file.
func downloadPackagerArchive(client *http.Client, baseURL *url.URL, extension, fileName string) (string, error) {
	file, err := os.CreateTemp("", "veracode_*."+extension)
	if err != nil {
		return "", err
	}

	defer file.Close()

	resp, err := http.Get(baseURL.JoinPath(fileName).String())
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return file.Name(), nil
}

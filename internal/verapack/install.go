package verapack

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InstallUploader installs the uploader jar file to the latest version.
//
// It automatically updates the existing install to the latest version.
func InstallUploader(dirPath string) (string, error) {
	jar, _ := cookiejar.New(&cookiejar.Options{})
	client := &http.Client{
		Jar: jar,
	}

	version, err := GetLatestUploaderVersion(client)
	if err != nil {
		return "", err
	}

	downloadPath, err := downloadUploaderArchive(client, version)
	if err != nil {
		return "", err
	}

	defer os.Remove(downloadPath)

	// Only include the VeracodeJavaAPI.jar file (archive contains help content as well)
	err = extractZipArchive(downloadPath, dirPath, map[string]bool{"VeracodeJavaAPI.jar": true})
	if err != nil {
		return "", err
	}

	file, err := os.Create(filepath.Join(dirPath, "VERSION"))
	if err != nil {
		return "", err
	}

	defer file.Close()

	_, err = file.WriteString(version)
	if err != nil {
		return "", err
	}

	return version, nil
}

// GetLatestUploaderVersion returns the latest version of the Veracode API wrapper jar.
func GetLatestUploaderVersion(client *http.Client) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://repo1.maven.org/maven2/com/veracode/vosp/api/wrappers/vosp-api-wrappers-java/maven-metadata.xml", nil)
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

	type versioning struct {
		Latest string `xml:"latest"`
	}

	type meta struct {
		Versioning versioning `xml:"versioning"`
	}

	var p meta

	err = xml.Unmarshal(bytes, &p)
	if err != nil {
		return "", err
	}

	return p.Versioning.Latest, nil
}

// downloadUploaderArchive streams the archive for the provided version from the remote source to
// a temporarily local file.
func downloadUploaderArchive(client *http.Client, version string) (string, error) {
	file, err := os.CreateTemp("", "wrapper_*.zip")
	if err != nil {
		return "", err
	}

	defer file.Close()

	resp, err := client.Get(fmt.Sprintf("https://repo1.maven.org/maven2/com/veracode/vosp/api/wrappers/vosp-api-wrappers-java/%s/vosp-api-wrappers-java-%s-dist.zip", version, version))
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

// InstallPackager installs the packager to the user's application directory. If shouldFullInstall
// is true, it runs fullPackagerInstall otherwise it runs partialPackagerInstall. Check the docs for
// those functions for more information.
//
// InstallPackager returns the version of the newly installed veracode CLI as well as an error if one
// occurred.
//
// NOTE: This function is OS/ARCH agnostic, but the functions it calls are not. In order to build this
// application for different environment, implement the required functions for that environment/tech.
func InstallPackager(shouldFullyInstall bool, dirPath string) (string, error) {
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
func partialPackagerInstall(dirPath string) (string, error) {
	jar, _ := cookiejar.New(&cookiejar.Options{})
	client := &http.Client{
		Jar: jar,
	}
	baseURL, _ := url.Parse("https://tools.veracode.com/veracode-cli")

	fileVersion, err := GetLatestPackagerVersion(client, baseURL)
	if err != nil {
		return "", err
	}
	fileName, ext := getPackagerFileName(fileVersion)

	downloadedPath, err := downloadPackagerArchive(client, baseURL, ext, fileName)
	if err != nil {
		return "", err
	}

	defer os.Remove(downloadedPath)

	err = extractZipArchive(downloadedPath, dirPath, nil)
	if err != nil {
		return "", err
	}

	return fileVersion, nil
}

// GetLatestPackagerVersion gets the latest version of the packager.
//
// Typical Dev comment;)
func GetLatestPackagerVersion(client *http.Client, baseURL *url.URL) (string, error) {
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

	resp, err := client.Get(baseURL.JoinPath(fileName).String())
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

// InstallScaAgent runs a package command with the CLI in order to install the SCA agent for the first time.
// It is run in a folder that has nothing to package and therefore won't produce any artefacts.
func InstallScaAgent(packagerPath string) error {
	cmd := exec.Command(filepath.Join(packagerPath, "veracode"), "package", "--source", packagerPath, "-a")

	out, err := cmd.CombinedOutput()
	s := string(out)
	if err != nil {
		// exit status 4 is a build failure, but means that the packager ran successfully.
		// We only care about other errors.
		if err.Error() != "exit status 4" {
			return fmt.Errorf("%s\n%s", err.Error(), s)
		}
	}

	return nil
}

func GetLocalVersion(path string) string {
	file, _ := os.ReadFile(path)
	if len(file) < 1 {
		return "na"
	}

	return string(file)
}

package verapack

import (
	"bytes"
	_ "embed"
	"errors"
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

//go:embed vosp-api-wrappers-java-*.jar
var uploaderFileBytes []byte

// InstallUploader installs the uploader. If Maven is installed, it will use Maven to download the latest jar file.
// Otherwise, it will write the embedded jar file to the destination folder.
//
// InstallUploader takes dirpath and version string arguments. dirpath is the target directory and version sets which
// version to install. If version is empty, the value will be set to 'LATEST'. If Maven is not installed, then version
// is ignored.
func InstallUploader(dirpath string, version string) error {
	execPath, err := exec.LookPath("mvn")
	if err != nil {
		// If maven is not installed, use the embedded backup jar.
		return installEmbeddedUploader(dirpath)
	}

	// Otherwise, use Maven to install the jar.

	if version == "" {
		version = "LATEST"
	}

	// 1. Download the wrapper to a temp directory.
	tempDir, tempFilePath, err := downloadTempUploader(execPath, version)
	if err != nil {
		return err
	}

	// Remove the temp dir regardless of outcome.
	defer os.RemoveAll(tempDir)

	// 2. If wrapper is already installed, create a backup in case of installation failure.
	_, existingErr := os.Stat(filepath.Join(dirpath, "VeracodeJavaAPI.jar"))
	if existingErr == nil {
		err = os.Rename(filepath.Join(dirpath, "VeracodeJavaAPI.jar"), filepath.Join(dirpath, "VeracodeJavaAPI_bup.jar"))
		if err != nil {
			return err
		}
	}

	// 3. Copy newly installed wrapper in the temp folder to the destination folder.
	err = copyTempUploader(tempFilePath, dirpath)
	if err != nil {
		// If copy fails, restore backup file.
		if existingErr == nil {
			err = os.Rename(filepath.Join(dirpath, "VeracodeJavaAPI_bup.jar"), filepath.Join(dirpath, "VeracodeJavaAPI.jar"))
			if err != nil {
				return err
			}
		}
		return err
	}

	// 4. Remove backup file.
	if existingErr == nil {
		os.Remove(filepath.Join(dirpath, "VeracodeJavaAPI_bup.jar"))
	}

	return nil
}

// downloadTempUploader downloads the latest wrapper to a temp directory and returns the temp dir path and the file path.
func downloadTempUploader(execPath string, version string) (string, string, error) {
	tempDir, err := os.MkdirTemp("", "vosp-api-wrapper-java_*")
	if err != nil {
		return "", "", err
	}

	cmd := exec.Command(execPath,
		"dependency:copy",
		fmt.Sprintf("-Dartifact=com.veracode.vosp.api.wrappers:vosp-api-wrappers-java:%s", version),
		fmt.Sprintf("-DoutputDirectory=%s", tempDir),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", errors.New(string(out))
	}

	fileList, err := os.ReadDir(tempDir)
	if err != nil {
		return "", "", err
	}

	return tempDir, filepath.Join(tempDir, fileList[0].Name()), nil
}

// copyTempUploader copies the downloaded wrapper from the temp directory to the destination directory.
func copyTempUploader(tempFilePath, dirPath string) error {
	inFile, err := os.Open(tempFilePath)
	if err != nil {
		return err
	}
	defer inFile.Close()

	outFile, err := os.Create(filepath.Join(dirPath, "VeracodeJavaAPI.jar"))
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, inFile)
	if err != nil {
		return err
	}

	inFile.Close()

	return nil
}

func installEmbeddedUploader(dirpath string) error {
	err := os.MkdirAll(dirpath, 0600)
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
		return err
	}

	defer os.Remove(downloadedPath)

	err = extractPackagerArchive(downloadedPath, dirPath)
	if err != nil {
		return err
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

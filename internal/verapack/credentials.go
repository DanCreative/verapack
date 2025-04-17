package verapack

import (
	"fmt"
	"os"
	"path/filepath"
)

// setLegacyCredentialsFile creates/truncates the credential file: %home%/.veracode/credential
func setLegacyCredentialsFile(homeDir, apiKey, apiSecret string) error {
	file, err := os.Create(filepath.Join(homeDir, ".veracode", "credentials"))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf(credentialFileLegacyFormat, apiKey, apiSecret))
	if err != nil {
		return err
	}

	return nil
}

// setCredentialsFile creates/truncates the credential file: %home%/.veracode/veracode.yml
func setCredentialsFile(homeDir, apiKey, apiSecret string) error {
	file, err := os.Create(filepath.Join(homeDir, ".veracode", "veracode.yml"))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf(credentialFileFormat, apiKey, apiSecret))
	if err != nil {
		return err
	}

	return nil
}

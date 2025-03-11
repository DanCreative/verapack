package verapack

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"dario.cat/mergo"
	"github.com/go-playground/validator"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

type SourceType string

const (
	Repo      SourceType = "repo"
	Directory SourceType = "directory"
)

const (
	credentialFileLegacyFormat = "[default]\nveracode_api_key_id     = %s\nveracode_api_key_secret = %s"
	credentialFileFormat       = "api:\n  key-id: %s\n  key-secret: %s"
)

//go:embed config.yaml
var configFileBytes []byte

type Options struct {
	// Packaging Options

	Verbose       bool       `yaml:"verbose"`
	AutoCleanup   bool       `yaml:"auto_cleanup"`
	OutputDir     string     `yaml:"-"`
	PackageSource string     `yaml:"package_source" validate:"required_without=ArtefactPaths,omitempty,url|dir"`
	Trust         bool       `yaml:"trust"`
	Type          SourceType `yaml:"type" validate:"oneof=directory repo"`

	// Upload Options

	// UploaderFilePath is the path to the Veracode Java wrapper jar file.
	UploaderFilePath string `yaml:"-"`
	// Name of the application profile.
	AppName string `yaml:"app_name" validate:"required"`
	// Create a application profile if the one provided in AppName does not exist.
	CreateProfile bool `yaml:"create_profile"`
	// FilePath is a []string of the filepaths for the application's artefacts.
	ArtefactPaths []string `yaml:"artefact_paths" validate:"required_without=PackageSource,omitempty,dive,file|dir"`
	// Name or version of the build that you want to scan.
	Version string `yaml:"version"`
	// Number of minutes to wait for the scan to complete and pass policy.
	// If the scan does not complete or fails policy, the build fails.
	//
	// Set the value to 0 to "fire-and-forget".
	ScanTimeout int `yaml:"scan_timeout"`

	// Interval, in seconds, to poll for the status of a running scan.
	// Value range is 30 to 120 (two minutes). Default is 120.
	ScanPollingInterval int `yaml:"scan_polling_interval" validate:"omitempty,min=30,max=120"`

	// Other options:
}

type Config struct {
	Default      Options   `yaml:"default" validate:"-"`
	Applications []Options `yaml:"applications" validate:"required,gt=0,dive"`
}

var validate *validator.Validate

// ReadConfig loads the config from a file, sets all of the defaults/overrides and validates the input.
func ReadConfig(filePath string) (Config, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return Config{}, err
	}

	c := Config{}

	if err = yaml.Unmarshal(content, &c); err != nil {
		return Config{}, err
	}

	setDynamicDefaults(&c)

	for i := range c.Applications {
		if err = mergo.Merge(&c.Applications[i], c.Default); err != nil {
			return Config{}, err
		}
	}

	if err = validateConfig(&c); err != nil {
		return Config{}, err
	}

	return c, nil
}

// validateConfig takes a *Config and uses the validator package to check that the config meets all
// required rules.
func validateConfig(config *Config) error {
	validate = validator.New()

	err := validate.Struct(config)
	if err != nil {
		var errs error

		var validateErrs validator.ValidationErrors
		if errors.As(err, &validateErrs) {
			for _, e := range validateErrs {
				switch e.Tag() {
				case "required":
					errs = errors.Join(errs, fmt.Errorf("config validation error at %s: field is required", e.Namespace()))
				case "required_without":
					errs = errors.Join(errs, fmt.Errorf("config validation error at %s: either field '%s' or field '%s' is required", e.Namespace(), e.Field(), e.Param()))
				case "oneof":
					errs = errors.Join(errs, fmt.Errorf("config validation error at %s: field value must be one of: [%v]", e.Namespace(), e.Param()))
				case "gt":
					switch e.Kind() {
					case reflect.Ptr:
						fallthrough
					case reflect.Slice:
						errs = errors.Join(errs, fmt.Errorf("config validation error at %s: list field requires more than %s entries", e.Namespace(), e.Param()))
					case reflect.Int:
						errs = errors.Join(errs, fmt.Errorf("config validation error at %s: number field needs to be bigger than %s", e.Namespace(), e.Param()))
					default:
						fmt.Println("default")
					}
				case "file":
					errs = errors.Join(errs, fmt.Errorf("config validation error at %s: file '%s' does not exist", e.Namespace(), e.Value()))
				case "dir":
					errs = errors.Join(errs, fmt.Errorf("config validation error at %s: directory '%s' does not exist", e.Namespace(), e.Value()))
				case "file|dir":
					errs = errors.Join(errs, fmt.Errorf("config validation error at %s: file or directory '%s' does not exist", e.Namespace(), e.Value()))
				case "min":
					errs = errors.Join(errs, fmt.Errorf("config validation error at %s: field must be greater or equal to: '%s'", e.Namespace(), e.Param()))
				case "max":
					errs = errors.Join(errs, fmt.Errorf("config validation error at %s: field must be equal to or smaller than: '%s'", e.Namespace(), e.Param()))
				default:
					errs = errors.Join(errs, fmt.Errorf("config validation error at %s: unspecified error with field, tag=%s,param=%s", e.Namespace(), e.Tag(), e.Param()))
				}
			}
		}
		return errs
	}

	return nil
}

// SetupConfig sets up all of the directories, credential files and config files that the application requires.
// It returns the application directory as a string in the first return value.
func SetupConfig(homeDir string) (string, error) {
	var err error

	appDir := filepath.Join(homeDir, ".veracode", "verapack")

	if err = os.MkdirAll(appDir, os.ModePerm); err != nil {
		return "", err
	}

	_, err = os.Stat(filepath.Join(appDir, "config.yaml"))
	if err == nil {
		fmt.Println("\tSkipping config file as it already exists")
		return appDir, nil
	}
	file, err := os.Create(filepath.Join(appDir, "config.yaml"))
	if err != nil {
		return "", err
	}

	defer file.Close()

	_, err = io.Copy(file, bytes.NewReader(configFileBytes))
	if err != nil {
		return "", err
	}

	fmt.Printf("\tCreated config file template here: %s. Please open it to finish the configuration before running the CLI\n", filepath.Join(appDir, "config.yaml"))

	return appDir, nil
}

func SetupCredentials(homeDir string) error {
	_, cerr := os.Stat(filepath.Join(homeDir, ".veracode", "credentials"))
	_, lerr := os.Stat(filepath.Join(homeDir, ".veracode", "veracode.yml"))

	if cerr == nil && lerr == nil {
		fmt.Println("\tSkipping credentials file as it already exists")
		fmt.Println("\tSkipping legacy credentials file as it already exists")
		return nil
	}

	fmt.Println("\tIf you do not have a Veracode API ID and Secret Key, \n\tnavigate to https://analysiscenter.veracode.eu/auth/index.jsp#APICredentialsGenerator \n\tto generate your API credentials")
	fmt.Print("\n\tPlease enter your API ID:\n\t")
	keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	fmt.Print("\n\tPlease enter your API Secret:\n\t")
	secretBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}

	if cerr != nil {
		file, err := os.Create(filepath.Join(homeDir, ".veracode", "veracode.yml"))
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = file.WriteString(fmt.Sprintf(credentialFileFormat, string(keyBytes), string(secretBytes)))
		if err != nil {
			return err
		}
	} else {
		fmt.Println("\n\tSkipping credentials file as it already exists")
	}

	if lerr != nil {
		file, err := os.Create(filepath.Join(homeDir, ".veracode", "credentials"))
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = file.WriteString(fmt.Sprintf(credentialFileLegacyFormat, string(keyBytes), string(secretBytes)))
		if err != nil {
			return err
		}
	} else {
		fmt.Println("\n\tSkipping legacy credentials file as it already exists")
	}

	fmt.Println("\n\tSuccessfully created credentials files")
	return nil
}

// setDynamicDefaults sets any default values that are based on dynamic values.
func setDynamicDefaults(config *Config) {
	if config.Default.Version == "" {
		config.Default.Version = time.Now().Format("02 Jan 2006 Static")
	}
}

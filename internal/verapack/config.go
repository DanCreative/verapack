package verapack

import (
	_ "embed"
	"os"
	"time"

	"dario.cat/mergo"
	"github.com/go-playground/validator/v10"
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

	// Upload Options
	// Name of the application profile.
	AppName string `yaml:"app_name" validate:"required"`
	// UploaderFilePath is the path to the Veracode Java wrapper jar file.
	UploaderFilePath string `yaml:"-"`
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

	// Packaging Options

	Verbose       bool       `yaml:"verbose"`
	AutoCleanup   bool       `yaml:"auto_cleanup"`
	OutputDir     string     `yaml:"-"`
	PackageSource string     `yaml:"package_source" validate:"required_without=ArtefactPaths,omitempty,url|dir"`
	Trust         bool       `yaml:"trust"`
	Type          SourceType `yaml:"type" validate:"oneof=directory repo"`

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

	validate = validator.New()

	if err = validate.Struct(&c); err != nil {
		return Config{}, err
	}

	return c, nil
}

// setDynamicDefaults sets any default values that are based on dynamic values.
func setDynamicDefaults(config *Config) {
	if config.Default.Version == "" {
		config.Default.Version = time.Now().Format("02 Jan 2006 Static")
	}
}

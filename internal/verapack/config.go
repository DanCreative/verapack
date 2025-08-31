package verapack

import (
	_ "embed"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"dario.cat/mergo"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
)

type SourceType string
type ScanType string

const (
	Repo      SourceType = "repo"
	Directory SourceType = "directory"

	ScanTypeSandbox ScanType = "sandbox"
	ScanTypePolicy  ScanType = "policy"
	ScanTypePromote ScanType = "promote"
)

var validate *validator.Validate

//go:embed config.yaml
var configFileBytes []byte

type Options struct {
	// Upload Options
	// Name of the application profile.
	AppName string `yaml:"app_name" validate:"required"`
	// UploaderFilePath is the path to the Veracode Java wrapper jar file.
	UploaderFilePath string `yaml:"-"`
	// Create a application profile if the one provided in AppName does not exist.
	CreateProfile *bool `yaml:"create_profile"`
	// FilePath is a []string of the filepaths for the application's artefacts.
	ArtefactPaths []string `yaml:"artefact_paths" validate:"required_without=PackageSource,omitempty,dive,file|dir"`
	// Name or version of the build that you want to scan.
	Version string `yaml:"version" validate:"required"`

	SandboxName string `yaml:"sandbox_name"` // Name of the sandbox in which to run the scan. This is what the user will provide in the yaml file.
	SandboxId   int    `yaml:"-"`            // ID of the sandbox in which to run the scan. Application will determine the sandbox id from the provided sandbox name.
	SandboxGuid string `yaml:"-"`            // GUID of the sandbox in which to run the scan.
	AppGuid     string `yaml:"-"`            // GUID of the application profile.
	AppId       int    `yaml:"-"`
	AutoPromote bool   `yaml:"auto_promote"`

	WaitForResult       bool `yaml:"wait_for_result"`       // Wait for the results of the scan.
	ScanTimeout         int  `yaml:"scan_timeout"`          // Number of minutes to wait for the scan to complete and pass policy.
	ScanPollingInterval int  `yaml:"scan_polling_interval"` // Interval, in seconds, to poll for the status of a running scan.

	// Packaging Options

	Verbose       *bool      `yaml:"verbose"`
	AutoCleanup   *bool      `yaml:"auto_cleanup"`
	PackageSource string     `yaml:"package_source" validate:"required_without=ArtefactPaths,omitempty"`
	Trust         *bool      `yaml:"-"`
	Strict        bool       `yaml:"strict"`
	Type          SourceType `yaml:"type" validate:"oneof=directory repo"`

	// Other options:
	ScanType ScanType `yaml:"-"` // The type of scan to run. Can be either policy or sandbox at this stage.
	Branch   string   `yaml:"branch"`
}

type Config struct {
	Default      Options   `yaml:"default" validate:"-"`
	Applications []Options `yaml:"applications" validate:"required,gt=0,dive"`
}

// NewConfig returns a new Config and sets all pointer values to avoid nil pointer errors downstream.
func NewConfig() Config {
	var b bool
	a := true
	return Config{
		Default: Options{
			CreateProfile: &b,
			Verbose:       &b,
			AutoCleanup:   &b,

			// Setting trust to true because when it is false, it requires user input and that is not
			// supporter/required by this application.
			Trust: &a,
		},
	}
}

func NewValidator() *validator.Validate {
	validate = validator.New()

	validate.RegisterStructValidation(optionsStructLevelValidation, Options{})

	return validate
}

// ReadConfig loads the config from a file, sets all of the defaults/overrides and validates the input.
func ReadConfig(filePath string, includeAppNames ...string) (Config, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return Config{}, err
	}

	c, err := SetDefaults(content)
	if err != nil {
		return Config{}, err
	}

	if len(includeAppNames) > 0 {
		filteredApps := make([]Options, 0, len(c.Applications))

		for _, app1 := range c.Applications {
			for _, app2 := range includeAppNames {
				if strings.EqualFold(app1.AppName, app2) {
					filteredApps = append(filteredApps, app1)
				}
			}
		}

		c.Applications = filteredApps
	}

	NewValidator()

	if err = validate.Struct(&c); err != nil {
		return Config{}, err
	}

	return c, nil
}

// SetDefaults merges the default values into the application configurations and sets any
// dynamic defaults.
func SetDefaults(configBytes []byte) (Config, error) {
	c := NewConfig()
	var err error

	if err = yaml.Unmarshal(configBytes, &c); err != nil {
		return Config{}, err
	}

	setDynamicDefaults(&c)

	for i := range c.Applications {
		if err = mergo.Merge(&c.Applications[i], c.Default, mergo.WithoutDereference); err != nil {
			return Config{}, err
		}

		setPostMergeDefaults(&c.Applications[i])
	}

	return c, nil
}

func setPostMergeDefaults(options *Options) {
	if options.WaitForResult || options.AutoPromote {
		if options.ScanTimeout <= 0 {
			options.ScanTimeout = 120
		}

		if options.ScanPollingInterval < 30 {
			options.ScanPollingInterval = 30
		}

		if options.ScanPollingInterval > 120 {
			options.ScanPollingInterval = 120
		}
	}
}

// setDynamicDefaults sets any default values that are based on dynamic values.
func setDynamicDefaults(config *Config) {
	if config.Default.Version == "" {
		config.Default.Version = time.Now().Format("02 Jan 2006 15:04PM Static")
	}
}

func optionsStructLevelValidation(sl validator.StructLevel) {
	options := sl.Current().Interface().(Options)

	switch options.Type {
	case Directory:
		if !isDir(options.PackageSource) {
			sl.ReportError(options.PackageSource, "PackageSource", "PackageSource", "package_source", "directory")
		}

	case Repo:
		if !isURL(options.PackageSource) && !isSCPLikeUrl(options.PackageSource) {
			sl.ReportError(options.PackageSource, "PackageSource", "PackageSource", "package_source", "repo")
		}
	}
}

// isDir is the validation function for validating if the current field's value is a valid existing directory.
func isDir(value string) bool {
	fileInfo, err := os.Stat(value)
	if err != nil {
		return false
	}

	return fileInfo.IsDir()
}

// isURL is the validation function for validating if the current field's value is a valid URL.
//
// Example from validator: https://github.com/go-playground/validator/blob/master/baked_in.go#L1474
func isURL(value string) bool {
	s := strings.ToLower(value)

	if len(s) == 0 {
		return false
	}

	// if isFileURL(s) {
	// 	return true
	// }

	url, err := url.Parse(s)
	if err != nil || url.Scheme == "" {
		return false
	}

	if url.Host == "" && url.Fragment == "" && url.Opaque == "" {
		return false
	}

	return true
}

// isSCPLikeUrl validates whether the URL is in a SCP-like format. This format is typically used when
// cloning a remote git repo using ssh.
//
// Credit to: https://github.com/whilp/git-urls/blob/master/urls.go and https://golang.org/src/cmd/go/vcs.go.
func isSCPLikeUrl(value string) bool {
	scpSyntax := regexp.MustCompile(`^([a-zA-Z0-9-._~]+@)?([a-zA-Z0-9._-]+):([a-zA-Z0-9./._-]+)(?:\?||$)(.*)$`)
	match := scpSyntax.FindAllStringSubmatch(value, -1)

	if len(match) == 0 {
		return false
	}

	return true
}

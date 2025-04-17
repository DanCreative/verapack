package verapack

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"

	"github.com/DanCreative/veracode-go/veracode"
	"github.com/DanCreative/verapack/internal/components/multistagesetup"
	"github.com/DanCreative/verapack/internal/components/reportcard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v2"
)

func NewApp() *cli.App {
	return &cli.App{
		Name:  "verapack",
		Usage: "Verapack is a utility that automates and simplifies running Veracode SAST scans for multiple applications from your local machine.",
		Commands: []*cli.Command{
			{
				Name:    "setup",
				Usage:   "Configure config files and install the Java wrapper and Veracode CLI if they are not already installed.",
				Action:  setup,
				Aliases: []string{"s"},
			},
			{
				Name:    "go",
				Usage:   "Package and/or Scan all applications in the config file.",
				Action:  run,
				Aliases: []string{"r"},
			},
			{
				Name:    "credentials",
				Aliases: []string{"c"},
				Usage:   "Options for managing your credentials.",
				Subcommands: []*cli.Command{
					{
						Name:   "refresh",
						Usage:  "Automatically re-generate your API credentials and update the credential files.",
						Action: refreshCredentials,
					},
					{
						Name:   "configure",
						Usage:  "Configure new credentials manually (Used for when existing credentials have expired or for when switching accounts).",
						Action: configureCredentials,
					},
				},
			},
		},
	}
}

func setup(cCtx *cli.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	appDir := filepath.Join(homeDir, ".veracode", "verapack")

	p := tea.NewProgram(multistagesetup.NewModel(
		multistagesetup.WithSpinner(defaultSpinnerOpts...),
		multistagesetup.WithStyles(multistagesetup.Styles{
			StatusFailure:    multistagesetup.SummaryStyle{Symbol: '✗', Colour: red},
			StatusSuccess:    multistagesetup.SummaryStyle{Symbol: '✓', Colour: green},
			StatusSkipped:    multistagesetup.SummaryStyle{Symbol: '✓', Colour: green, Style: lipgloss.NewStyle().Strikethrough(true)},
			StatusTodo:       multistagesetup.SummaryStyle{Symbol: '!'},
			StatusInProgress: lipgloss.NewStyle().Foreground(lightBlue),
			StageBlock: lipgloss.NewStyle().Padding(0, 1).Margin(0, 0, 0, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(darkGray),
			MsgText: darkGrayForeground,
			FinalMessage: lipgloss.NewStyle().Padding(0, 1, 1, 1).Margin(0, 0, 0, 2).
				Align(lipgloss.Center).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lightBlue),
		}),
		multistagesetup.WithTasks(append(
			SetupCredentials(homeDir),
			SetupConfig(homeDir, appDir),
			InstallDependancyPackager(),
			InstallDependancyWrapper(appDir),
		)...),
		multistagesetup.WithFinalMessage(fmt.Sprintf("%s\n\n%s", "Initial setup has been successfully completed. To complete the setup, please open below config file and add your applications with their scan settings:", lightBlueForeground.Render(filepath.Join(appDir, "config.yaml")))),
	))
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func run(cCtx *cli.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	uploaderPath := filepath.Join(homeDir, ".veracode", "verapack", "VeracodeJavaAPI.jar")

	c, err := ReadConfig(filepath.Join(homeDir, ".veracode", "verapack", "config.yaml"))
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	path := os.Getenv("PATH")
	os.Setenv("PATH", path+";"+getPackagerLocation())

	p := tea.NewProgram(reportcard.NewModel(
		reportcard.WithSpinner(defaultSpinnerOpts...),
		reportcard.WithStyles(reportcard.Styles{
			NameHeader:  lipgloss.NewStyle().Bold(true).Padding(0, 1),
			TaskHeaders: lipgloss.NewStyle().Bold(true).Padding(0, 1),
			Cell:        lipgloss.NewStyle().Padding(0, 1),
			Border:      lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(darkGray).Padding(0, 2).MarginLeft(2),
			Selected:    lipgloss.NewStyle().Foreground(lightBlue),
		}),
		reportcard.WithData(appsToRows(c.Applications)),
		reportcard.WithTasks([]reportcard.Column{{Name: "Package", Width: 7}, {Name: "Scan", Width: 4}, {Name: "Cleanup", Width: 7}}),
	))

	for k, app := range c.Applications {
		go func() {
			packageAndUploadApplication(uploaderPath, app, k, p)
		}()
	}

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func VersionPrinter(cCtx *cli.Context) {
	var vUploader string
	homeDir, err := os.UserHomeDir()
	if err != nil {
		vUploader = "Can't access Uploader"
	}
	vUploader = versionUploader(filepath.Join(homeDir, ".veracode", "verapack", "VeracodeJavaAPI.jar"))

	path := os.Getenv("PATH")
	os.Setenv("PATH", path+";"+getPackagerLocation())

	fmt.Printf("%s version %s\n%s%s", cCtx.App.Name, cCtx.App.Version, versionPackager(), vUploader)
}

func refreshCredentials(cCtx *cli.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	key, secret, err := veracode.LoadVeracodeCredentials()
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	httpClient := &http.Client{
		Jar: jar,
	}

	client, err := veracode.NewClient(httpClient, key, secret)
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	p := tea.NewProgram(NewCredentialsRefreshModel(client, homeDir))
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func configureCredentials(cCtx *cli.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}
	var apiKey, apiSecret string
	p := tea.NewProgram(NewCredentialsConfigureModel(NewCredentialsTask(&apiKey, &apiSecret, nil, nil), homeDir))
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

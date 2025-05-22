package verapack

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/DanCreative/veracode-go/veracode"
	sand "github.com/DanCreative/verapack/internal/components/middleware/sandbox"
	"github.com/DanCreative/verapack/internal/components/middleware/singleselect"
	"github.com/DanCreative/verapack/internal/components/multistagesetup"
	"github.com/DanCreative/verapack/internal/components/reportcard"
	"github.com/DanCreative/verapack/internal/components/version"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v2"
)

func NewApp() *cli.App {
	return &cli.App{
		Name:  "verapack",
		Usage: "Verapack is a utility that automates and simplifies running Veracode SAST scans for multiple applications from your local machine",
		Commands: []*cli.Command{
			{
				Name:    "setup",
				Usage:   "Configure config files and install the Java wrapper and Veracode CLI if they are not already installed",
				Action:  setup,
				Aliases: []string{"s"},
			},
			{
				Name:    "scan",
				Usage:   "Package and Scan applications defined in the config file",
				Aliases: []string{"r"},
				Subcommands: []*cli.Command{
					{
						Name:   "sandbox",
						Usage:  "Run a sandbox scan for the applications defined in the config file",
						Action: sandbox,
					},
					{
						Name:   "policy",
						Usage:  "Run a policy scan for the applications defined in the config file",
						Action: policy,
					},
					{
						Name:   "promote",
						Usage:  "Promote the latest sandbox scan for the applications defined in the config file",
						Action: promote,
					},
				},
			},
			{
				Name:    "update",
				Usage:   "Update all dependencies to the latest versions",
				Action:  update,
				Aliases: []string{"u"},
			},
			{
				Name:    "credentials",
				Aliases: []string{"c"},
				Usage:   "Options for managing your credentials",
				Subcommands: []*cli.Command{
					{
						Name:   "refresh",
						Usage:  "Automatically re-generate your API credentials and update the credential files",
						Action: refreshCredentials,
					},
					{
						Name:   "configure",
						Usage:  "Configure new credentials manually (Used for when existing credentials have expired or for when switching accounts)",
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

	var tasks []multistagesetup.SetupTask
	tasks = append(tasks, Prerequisites())
	tasks = append(tasks, SetupCredentials(homeDir)...)
	tasks = append(tasks, SetupConfig(homeDir, appDir), InstallDependencyPackager(), InstallDependencyWrapper(), SetupInstallScaAgent())

	p := tea.NewProgram(multistagesetup.NewModel(
		multistagesetup.WithSpinner(defaultSpinnerOpts...),
		multistagesetup.WithStyles(multistagesetup.Styles{
			StatusFailure:    multistagesetup.SummaryStyle{Symbol: '✗', Colour: red},
			StatusSuccess:    multistagesetup.SummaryStyle{Symbol: '✓', Colour: green},
			StatusWarning:    multistagesetup.SummaryStyle{Symbol: '⚠', Colour: orange},
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
		multistagesetup.WithTasks(tasks...),
		multistagesetup.WithFinalMessage(fmt.Sprintf("%s\n\n%s", "Initial setup has been successfully completed. To complete the setup, please open below config file and add your applications with their scan settings:", lightBlueForeground.Render(filepath.Join(appDir, "config.yaml")))),
	))
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func sandbox(cCtx *cli.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	// 1. Load & validate config and handle sandbox edge cases

	c, err := ReadConfig(filepath.Join(homeDir, ".veracode", "verapack", "config.yaml"))
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	badApps := HandleSandboxNotProvided(c.Applications, ScanTypeSandbox)

	if len(badApps) == len(c.Applications) {
		err = errors.New("there are no application with field sandbox_name")
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

	startChan := make(chan struct{})
	var m tea.Model
	ctx := context.Background()
	uploaderPath := filepath.Join(getWrapperLocation(), "VeracodeJavaAPI.jar")
	path := os.Getenv("PATH")
	os.Setenv("PATH", path+";"+getPackagerLocation())

	if len(badApps) > 0 {
		// There are apps that do not have the SandboxName field set.
		// Prompt the user for what they would like to do.
		m = singleselect.NewModel(
			singleselect.WithBodyText(renderBodyText(badApps)),
			singleselect.WithOptions(
				"Only scan the applications with the provided field",
				"Cancel the scan",
			),
			singleselect.WithStyles(singleselect.Styles{
				Highlight: lightBlueForeground,
				Border: lipgloss.NewStyle().
					Padding(0, 1, 1, 1).
					Margin(0, 0, 0, 2).
					BorderForeground(darkGray).
					Border(lipgloss.RoundedBorder()),
			}),
			singleselect.WithPostFunc(func(selection int, model singleselect.Model) (tea.Model, tea.Cmd) {
				switch selection {
				case 1:
					// The user has opted to cancel the scan
					return model, tea.Quit

				case 0:
					// The user has opted to only run scans for apps with the provided field
					RemoveBadApps(&c, badApps)

					m := sand.NewModel(
						appsToSandboxOptions(c.Applications),
						client,
						ctx,
						sand.WithSpinner(defaultSpinnerOpts...),
						sand.WithErrorRenderFunc(rawRenderErrors),
						sand.WithStyles(sand.Styles{
							Border: lipgloss.NewStyle().Padding(0, 1, 1, 1).Margin(0, 0, 0, 2).BorderForeground(darkGray).Border(lipgloss.RoundedBorder()),
							Counts: darkGrayForeground,
						}),
						sand.WithPostFunc(func(model sand.Model, size tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
							if len(model.GetErrors()) > 0 {
								return model, tea.Quit
							}

							startChan <- struct{}{}

							m := PrepareReportCard(c)

							return m, m.Init()
						}),
					)

					cmd := make([]tea.Cmd, 2)
					var mi tea.Model

					cmd[0] = m.Init()
					mi, cmd[1] = m.Update(struct{}{})

					return mi, tea.Sequence(cmd...)

				}
				return nil, nil
			}),
		)

	} else {
		// All apps are correct.
		m = sand.NewModel(
			appsToSandboxOptions(c.Applications),
			client,
			ctx,
			sand.WithSpinner(defaultSpinnerOpts...),
			sand.WithErrorRenderFunc(rawRenderErrors),
			sand.WithStyles(sand.Styles{
				Border: lipgloss.NewStyle().Padding(0, 1, 1, 1).Margin(0, 0, 0, 2).BorderForeground(darkGray).Border(lipgloss.RoundedBorder()),
				Counts: darkGrayForeground,
			}),
			sand.WithPostFunc(func(model sand.Model, size tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
				if len(model.GetErrors()) > 0 {
					return model, tea.Quit
				}

				startChan <- struct{}{}

				m := PrepareReportCard(c)

				return m, m.Init()
			}),
		)
	}

	p := tea.NewProgram(m)

	go func() {
		<-startChan
		for k, app := range c.Applications {
			go func() {
				if app.ScanType == ScanTypePromote {
					promoteSandbox(client, ctx, app, k, p)
				} else {
					packageAndUploadApplication(uploaderPath, app, k, p)
				}
			}()
		}
	}()

	m, err = p.Run()
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	if s, ok := m.(sand.Model); ok {
		if errs := s.GetErrors(); len(errs) > 0 {
			return err
		}
	}

	return nil
}

func promote(cCtx *cli.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	c, err := ReadConfig(filepath.Join(homeDir, ".veracode", "verapack", "config.yaml"))
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

	startChan := make(chan struct{})
	var m tea.Model
	ctx := context.Background()
	uploaderPath := filepath.Join(getWrapperLocation(), "VeracodeJavaAPI.jar")
	path := os.Getenv("PATH")
	os.Setenv("PATH", path+";"+getPackagerLocation())

	badApps := HandleSandboxNotProvided(c.Applications, ScanTypePromote)

	if len(badApps) > 0 {
		var opts []string
		var afterFunc singleselect.PostFunc

		if len(badApps) == len(c.Applications) {
			opts = []string{
				"Run policy scans where the field was not provided",
				"Cancel the scan",
			}

			afterFunc = func(selection int, model singleselect.Model) (tea.Model, tea.Cmd) {
				switch selection {
				case 1:
					// The user has opted to cancel the scan
					return model, tea.Quit
				case 0:
					// The user has selected to run policy scans instead of promotions for apps that do not have the sandbox_name field
					for k := range badApps {
						badApps[k].ScanType = ScanTypePolicy
					}

					startChan <- struct{}{}

					m := PrepareReportCard(c)

					return m, m.Init()
				}
				return nil, nil
			}

		} else {
			opts = []string{
				"Run policy scans where the field was not provided",
				"Only scan the applications with the provided field",
				"Cancel the scan",
			}

			afterFunc = func(selection int, model singleselect.Model) (tea.Model, tea.Cmd) {
				switch selection {
				case 2:
					// The user has opted to cancel the scan
					return model, tea.Quit

				case 1:
					// The user has opted to only run scans for apps with the provided field
					RemoveBadApps(&c, badApps)

					m := sand.NewModel(
						appsToSandboxOptions(c.Applications),
						client,
						ctx,
						sand.WithSpinner(defaultSpinnerOpts...),
						sand.WithErrorRenderFunc(rawRenderErrors),
						sand.WithStyles(sand.Styles{
							Border: lipgloss.NewStyle().Padding(0, 1, 1, 1).Margin(0, 0, 0, 2).BorderForeground(darkGray).Border(lipgloss.RoundedBorder()),
							Counts: darkGrayForeground,
						}),
						sand.WithPostFunc(func(model sand.Model, size tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
							if len(model.GetErrors()) > 0 {
								return model, tea.Quit
							}

							startChan <- struct{}{}

							m := PrepareReportCard(c)

							return m, m.Init()
						}),
					)

					cmd := make([]tea.Cmd, 2)
					var mi tea.Model

					cmd[0] = m.Init()
					mi, cmd[1] = m.Update(struct{}{})

					return mi, tea.Sequence(cmd...)

				case 0:
					// The user has selected to run policy scans instead of promotions for apps that do not have the sandbox_name field
					for k := range badApps {
						badApps[k].ScanType = ScanTypePolicy
					}

					m := sand.NewModel(
						appsToSandboxOptions(c.Applications),
						client,
						ctx,
						sand.WithSpinner(defaultSpinnerOpts...),
						sand.WithErrorRenderFunc(rawRenderErrors),
						sand.WithStyles(sand.Styles{
							Border: lipgloss.NewStyle().Padding(0, 1, 1, 1).Margin(0, 0, 0, 2).BorderForeground(darkGray).Border(lipgloss.RoundedBorder()),
							Counts: darkGrayForeground,
						}),
						sand.WithPostFunc(func(model sand.Model, size tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
							if len(model.GetErrors()) > 0 {
								return model, tea.Quit
							}

							startChan <- struct{}{}

							m := PrepareReportCard(c)

							return m, m.Init()
						}),
					)

					cmd := make([]tea.Cmd, 2)
					var mi tea.Model

					cmd[0] = m.Init()
					mi, cmd[1] = m.Update(struct{}{})

					return mi, tea.Sequence(cmd...)
				}
				return nil, nil
			}
		}

		m = singleselect.NewModel(
			singleselect.WithBodyText(renderBodyText(badApps)),
			singleselect.WithOptions(opts...),
			singleselect.WithStyles(singleselect.Styles{
				Highlight: lightBlueForeground,
				Border: lipgloss.NewStyle().
					Padding(0, 1, 1, 1).
					Margin(0, 0, 0, 2).
					BorderForeground(darkGray).
					Border(lipgloss.RoundedBorder()),
			}),
			singleselect.WithPostFunc(afterFunc),
		)
	} else {
		// m = sand.NewModel(appsToSandboxOptions(c.Applications), client, ctx)
		m = sand.NewModel(
			appsToSandboxOptions(c.Applications),
			client,
			ctx,
			sand.WithSpinner(defaultSpinnerOpts...),
			sand.WithErrorRenderFunc(rawRenderErrors),
			sand.WithStyles(sand.Styles{
				Border: lipgloss.NewStyle().Padding(0, 1, 1, 1).BorderForeground(darkGray).Margin(0, 0, 0, 2).Border(lipgloss.RoundedBorder()),
				Counts: darkGrayForeground,
			}),
			sand.WithPostFunc(func(model sand.Model, size tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
				if len(model.GetErrors()) > 0 {
					return model, tea.Quit
				}

				startChan <- struct{}{}

				m := PrepareReportCard(c)

				return m, m.Init()
			}),
		)
	}

	p := tea.NewProgram(m)

	go func() {
		<-startChan
		for k, app := range c.Applications {
			go func() {
				if app.ScanType == ScanTypePromote {
					promoteSandbox(client, ctx, app, k, p)
				} else {
					packageAndUploadApplication(uploaderPath, app, k, p)
				}
			}()
		}
	}()

	m, err = p.Run()
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	if s, ok := m.(sand.Model); ok {
		if errs := s.GetErrors(); len(errs) > 0 {
			return err
		}
	}

	return nil
}

func policy(cCtx *cli.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	uploaderPath := filepath.Join(getWrapperLocation(), "VeracodeJavaAPI.jar")

	c, err := ReadConfig(filepath.Join(homeDir, ".veracode", "verapack", "config.yaml"))
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	for k := range c.Applications {
		c.Applications[k].ScanType = ScanTypePolicy
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
		reportcard.WithData(appsToRows(c.Applications, columnOptionsStandard)),
		reportcard.WithTasks([]reportcard.Column{{Name: "Package", Width: 7}, {Name: "Scan", Width: 4}, {Name: "Cleanup", Width: 7}}),
		reportcard.WithPrefixColumns([]reportcard.Column{{Name: "Scan Type", Width: 9}}),
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

func update(cCtx *cli.Context) error {
	p := tea.NewProgram(multistagesetup.NewModel(
		multistagesetup.WithSpinner(defaultSpinnerOpts...),
		multistagesetup.WithStyles(multistagesetup.Styles{
			StatusFailure:    multistagesetup.SummaryStyle{Symbol: '✗', Colour: red},
			StatusSuccess:    multistagesetup.SummaryStyle{Symbol: '✓', Colour: green},
			StatusWarning:    multistagesetup.SummaryStyle{Symbol: '⚠', Colour: orange},
			StatusSkipped:    multistagesetup.SummaryStyle{Symbol: '✓', Colour: green, Style: lipgloss.NewStyle().Strikethrough(true)},
			StatusTodo:       multistagesetup.SummaryStyle{Symbol: '!'},
			StatusInProgress: lipgloss.NewStyle().Foreground(lightBlue),
			StageBlock: lipgloss.NewStyle().Padding(0, 1).Margin(0, 0, 0, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(darkGray),
			MsgText: darkGrayForeground,
		}),
		multistagesetup.WithTasks(
			Prerequisites(),
			UpdateDependencyPackager(),
			UpdateDependencyWrapper(),
			SetupInstallScaAgent(),
		),
	))
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func VersionPrinter(cCtx *cli.Context) {
	jar, _ := cookiejar.New(&cookiejar.Options{})

	httpClient := &http.Client{
		Jar: jar,
	}

	m := version.NewModel(
		GetLocalVersion(filepath.Join(getWrapperLocation(), "VERSION")),
		GetLocalVersion(filepath.Join(getPackagerLocation(), "VERSION")),
		cCtx.App.Version,
		cCtx.App.Name,
		func() (string, error) {
			return GetLatestUploaderVersion(httpClient)
		},
		func() (string, error) {
			baseURL, _ := url.Parse("https://tools.veracode.com/veracode-cli")
			return GetLatestPackagerVersion(httpClient, baseURL)
		},
		version.WithSpinner([]spinner.Option{
			spinner.WithSpinner(spinner.Spinner{
				Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
				FPS:    time.Second / 10,
			}),
			spinner.WithStyle(darkGrayForeground),
		}...),
		version.WithStyles(version.Styles{
			Muted: darkGrayForeground,
			Loud:  lipgloss.NewStyle().Foreground(orange),
		}),
	)

	tea.NewProgram(m).Run()
}

func PrepareReportCard(c Config) reportcard.Model {
	var totalPromoting, totalPolicy int

	for k := range c.Applications {
		switch c.Applications[k].ScanType {
		case ScanTypePolicy, ScanTypeSandbox:
			totalPolicy++
		case ScanTypePromote:
			totalPromoting++
		}
	}

	var columnsOption []reportcard.Column
	var rowsOption []reportcard.Row

	if totalPromoting > 0 && totalPolicy == 0 {
		columnsOption = []reportcard.Column{{Name: "Promote", Width: 7}}
		rowsOption = appsToRows(c.Applications, columnOptionsPromote)

	} else if totalPromoting == 0 && totalPolicy > 0 {
		columnsOption = []reportcard.Column{{Name: "Package", Width: 7}, {Name: "Scan", Width: 4}, {Name: "Cleanup", Width: 7}}
		rowsOption = appsToRows(c.Applications, columnOptionsStandard)

	} else {
		columnsOption = []reportcard.Column{{Name: "Package", Width: 7}, {Name: "Scan", Width: 4}, {Name: "Cleanup", Width: 7}, {Name: "Promote", Width: 7}}
		rowsOption = appsToRows(c.Applications, columnOptionsMixed)
	}

	return reportcard.NewModel(
		reportcard.WithSpinner(defaultSpinnerOpts...),
		reportcard.WithStyles(reportcard.Styles{
			NameHeader:  lipgloss.NewStyle().Bold(true).Padding(0, 1),
			TaskHeaders: lipgloss.NewStyle().Bold(true).Padding(0, 1),
			Cell:        lipgloss.NewStyle().Padding(0, 1),
			Border:      lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(darkGray).Padding(0, 2).MarginLeft(2),
			Selected:    lipgloss.NewStyle().Foreground(lightBlue),
		}),
		reportcard.WithData(rowsOption),
		reportcard.WithTasks(columnsOption),
		reportcard.WithPrefixColumns([]reportcard.Column{{Name: "Scan Type", Width: 9}}),
	)
}

func RemoveBadApps(c *Config, badApps []*Options) {
	newApplications := make([]Options, 0, len(c.Applications)-len(badApps))

	for j := range c.Applications {
		var isBad bool

		for k := range badApps {
			if badApps[k].AppName == c.Applications[j].AppName {
				isBad = true
			}
		}
		if !isBad {
			newApplications = append(newApplications, c.Applications[j])
		}
	}

	c.Applications = newApplications
}

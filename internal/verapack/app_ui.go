//go:build ui

package verapack

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "embed"

	"github.com/DanCreative/veracode-go/veracode"
	sand "github.com/DanCreative/verapack/internal/components/middleware/sandbox"
	"github.com/DanCreative/verapack/internal/components/middleware/singleselect"
	"github.com/DanCreative/verapack/internal/components/multistagesetup"
	"github.com/DanCreative/verapack/internal/components/reportcard"
	"github.com/DanCreative/verapack/internal/components/version"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/goccy/go-yaml"
	"github.com/urfave/cli/v2"
)

func Setup_ui(cCtx *cli.Context) error {
	tasks := []multistagesetup.SetupTask{
		multistagesetup.NewSetupTask("Check prerequisites", NewSimpleTask(func(values map[string]any) tea.Cmd {
			time.Sleep(300 * time.Millisecond)
			return func() tea.Msg { return multistagesetup.NewSuccessfulTaskResult("", nil) }
		})),
		SetupCredentialsUserPrompt(func() (string, string, error) { return "", "", errors.New("credential file not set") }),
		multistagesetup.NewSetupTask("Create credential file", NewSimpleTask(func(values map[string]any) tea.Cmd {
			time.Sleep(300 * time.Millisecond)
			return func() tea.Msg { return multistagesetup.NewSuccessfulTaskResult("", nil) }
		})),
		multistagesetup.NewSetupTask("Create legacy credential file", NewSimpleTask(func(values map[string]any) tea.Cmd {
			time.Sleep(300 * time.Millisecond)
			return func() tea.Msg { return multistagesetup.NewSuccessfulTaskResult("", nil) }
		})),
		multistagesetup.NewSetupTask("Set up initial config template", NewSimpleTask(func(values map[string]any) tea.Cmd {
			time.Sleep(300 * time.Millisecond)
			return func() tea.Msg { return multistagesetup.NewSuccessfulTaskResult("", nil) }
		})),
		multistagesetup.NewSetupTask("Install Veracode CLI", NewSimpleTask(func(values map[string]any) tea.Cmd {
			return func() tea.Msg {
				time.Sleep(2 * time.Second)
				return multistagesetup.NewSuccessfulTaskResult("successfully installed version: 2.40.0", nil)
			}
		})),
		multistagesetup.NewSetupTask("Install Veracode Uploader", NewSimpleTask(func(values map[string]any) tea.Cmd {
			return func() tea.Msg {
				time.Sleep(2 * time.Second)
				return multistagesetup.NewSuccessfulTaskResult("successfully installed version: 24.10.15.0", nil)
			}
		})),
		multistagesetup.NewSetupTask("Install SCA Agent", NewSimpleTask(func(values map[string]any) tea.Cmd {
			return func() tea.Msg {
				time.Sleep(1 * time.Second)
				return multistagesetup.NewSuccessfulTaskResult("successfully installed", nil)
			}
		})),
	}

	p := tea.NewProgram(PrepareSetup("C:\\Users\\user\\.veracode\\verapack\\config.yaml", tasks))
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func checkConfigPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}

	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml", nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".veracode", "verapack", "config.yaml"), nil
}

func Sandbox_ui(cCtx *cli.Context) error {
	// Load & validate config and handle sandbox edge cases
	configPath, err := checkConfigPath(cCtx.Path("c"))
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	c, err := ReadConfig(configPath)
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

	// Setup mock http server and client

	server := newVeracodeMockServer(c)
	defer server.Close()

	client, err := newVeracodeMockClient(strings.Replace(server.URL, "https://", "", 1))
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	startChan := make(chan struct{})
	var m tea.Model
	ctx := context.Background()

	if len(badApps) > 0 {
		// There are apps that do not have the SandboxName field set.
		// Prompt the user for what they would like to do.
		m = singleselect.NewModel(
			singleselect.WithBodyText(renderBodyText(badApps)),
			singleselect.WithHelp(defaultHelp),
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
					// TODO
					// promoteSandbox(client, ctx, app, k, p)
				} else {
					packageAndUploadApplication_ui(app, k, p, client, ctx)
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

func packageAndUploadApplication_ui(options Options, appId int, reporter reporter, client *veracode.Client, ctx context.Context) {
	// Package
	if options.PackageSource != "" {
		if appId > 1 {
			time.Sleep(time.Duration(rand.IntN(5)+1) * time.Second)
		}

		if appId > 1 && rand.IntN(4)+1 == 1 {
			reporter.Send(reportcard.TaskResultMsg{
				Status: reportcard.Failure,
				Index:  appId,
				Output: `Veracode CLI v2.40.0 -- 8823f61
	Please ensure your project builds successfully without any errors.

Packaging code for project verademo. Please wait; this may take a while...
Verifying source project language ...
Copying Java artifacts for MavenPackager project. Build failed, please run with --verbose flag for more details.
Packaging generic Javascript artifact for no-pm project.
Javascript project verademo packaged to: path\to\out\veracode-auto-pack-verademo-js-no-pm.zip
[GenericPackagerSQL] Packaging succeeded for the path path\to\temp-clone_448303049\verademo.
. Please check the verbose information for more details.ject path\to\temp-clone_448303049\verademo\app
This may result in reduced analysis scope, quality or performance.
Successfully created 2 artifact(s).
[INFO] SUMMARY - Javascript (GenericPackagerJS): "path\to\out\veracode-auto-pack-verademo-js-no-pm.zip"  (Size: 123.0KB, SHA2: 124d7a22689101b8327e9b73b96f83d12e36a4cba391i621a690878a871967ed)
[INFO] SUMMARY - SQL (GenericPackagerSQL): "path\to\out\veracode-auto-pack-verademo-sql.zip"  (Size: 12.6KB, SHA2: 8ab8eedd60fc0353152473e68bf190c8bc4d3444cd068dc95a06269fd4a79d72)
Total time taken to complete command: 46.724s`,
			})
		} else {
			reporter.Send(reportcard.TaskResultMsg{
				Status: reportcard.Success,
				Index:  appId,
				Output: `Veracode CLI v2.40.0 -- 8823f61
Please ensure your project builds successfully without any errors.

Packaging code for project verademo. Please wait; this may take a while...

Verifying source project language ...
Copying Java artifacts for MavenPackager project. Build failed, please run with --verbose flag for more details.
Packaging generic Javascript artifact for no-pm project.
Javascript project verademo packaged to: path\to\out\veracode-auto-pack-verademo-js-no-pm.zip
[GenericPackagerSQL] Packaging succeeded for the path path\to\temp-clone_448303049\verademo.
. Please check the verbose information for more details.ject path\to\temp-clone_448303049\verademo\app
This may result in reduced analysis scope, quality or performance.
Successfully created 2 artifact(s).
[INFO] SUMMARY - Javascript (GenericPackagerJS): "path\to\out\veracode-auto-pack-verademo-js-no-pm.zip"  (Size: 123.0KB, SHA2: 124d7a22689101b8327e9c73b96f83d12e36a4cba391f621a690878a871967ed)
[INFO] SUMMARY - SQL (GenericPackagerSQL): "path\to\out\veracode-auto-pack-verademo-sql.zip"  (Size: 12.6KB, SHA2: 8ab8eedd60fc0353152473e68bf190a8ba4d3444cd068dc95a06269fd4b79d72)
Total time taken to complete command: 46.724s`,
			})
		}
	}

	if appId > 0 {
		time.Sleep(time.Duration(rand.IntN(3)) * time.Second)
	}

	// Upload
	if appId > 0 && rand.IntN(4)+1 == 1 {
		reporter.Send(reportcard.TaskResultMsg{
			Status: reportcard.Failure,
			Index:  appId,
			Output: `[2025.06.01 19:12:26.380] Transaction ID: [123-123-123-124]
[2025.06.01 19:12:28.940]
[2025.06.01 19:12:28.940] Application profile "Verademo" (appid=0) was located.
[2025.06.01 19:12:28.940]
[2025.06.01 19:12:28.940] Creating a new analysis with name "25.0.0".
[2025.06.01 19:12:29.707]
[2025.06.01 19:12:29.707] * Action "UploadAndScan" returned the following message:
[2025.06.01 19:12:29.707] * The version 25.0.0 already exists
[2025.06.01 19:12:29.707]
[2025.06.01 19:12:31.055] Scan status is Results Ready
[2025.06.01 19:12:31.057]
[2025.06.01 19:12:31.057] * A scan has failed to complete successfully. Delete the failed scan from the Veracode Platform and try again.`,
		})
	} else {
		reporter.Send(reportcard.TaskResultMsg{
			Status: reportcard.Success,
			Index:  appId,
			Output: `[2025.06.01 19:10:36.383] Transaction ID: [123-123-123-123]
[2025.06.01 19:10:39.543]
[2025.06.01 19:10:39.543] Application profile "Verademo" (appid=0) was located.
[2025.06.01 19:10:39.544]
[2025.06.01 19:10:39.544] Creating a new analysis with name "25.0.0".
[2025.06.01 19:10:41.788]
[2025.06.01 19:10:41.788] The analysis id of the new analysis is "0".
[2025.06.01 19:10:41.789]
[2025.06.01 19:10:41.789] Uploading: path\to\out\veracode-auto-pack-verademo-js-no-pm.zip
[2025.06.01 19:10:43.311]
[2025.06.01 19:10:43.311] Starting pre-scan verification for application "Verademo" analysis "25.0.0".
[2025.06.01 19:10:44.948]
[2025.06.01 19:10:44.948] Scan polling interval is set to the default of 120 seconds.
[2025.06.01 19:10:44.949]
[2025.06.01 19:10:44.949] Application "Verademo" analysis "25.0.0" will be automatically submitted for scanning if the pre-scan verification is successful.`,
		})
	}

	// Cleanup
	if *options.AutoCleanup {
		reporter.Send(reportcard.TaskResultMsg{
			Status: reportcard.Success,
			Index:  appId,
		})
	}

	shouldAutoPromote := options.AutoPromote && options.ScanType == ScanTypeSandbox

	// Result, Promote & Policy
	if shouldAutoPromote {
		time.Sleep(time.Duration(rand.IntN(5)+1) * time.Second)
		res := result{PassedPolicy: appId == 0 || rand.IntN(4) > 0}

		if p := rand.IntN(3); p == 0 {
			res.PolicyStatus = "Did Not Pass"
		} else if p == 1 {
			res.PolicyStatus = "Pass"
		} else {
			res.PolicyStatus = "Conditional Pass"
		}

		// Result
		reporter.Send(reportcard.TaskResultMsg{
			Status:              reportcard.Success,
			Index:               appId,
			CustomSuccessStatus: createCustomTaskStatusFromResult(res, false),
		})

		// Promote
		if res.PassedPolicy {
			reporter.Send(reportcard.TaskResultMsg{
				Status: reportcard.Success,
				Index:  appId,
			})
		} else {
			reporter.Send(reportcard.TaskResultMsg{
				Status: reportcard.Failure,
				Index:  appId,
				Output: "The application did not pass the policy rules. Therefore auto-promotion was cancelled.",
			})
		}

		// Policy
		reporter.Send(reportcard.TaskResultMsg{
			Status:              reportcard.Success,
			Index:               appId,
			CustomSuccessStatus: createCustomTaskStatusFromResult(res, true),
		})
	}

	// Result & Policy
	if options.WaitForResult && !shouldAutoPromote {
		time.Sleep(time.Duration(rand.IntN(5)+1) * time.Second)
		if appId > 0 && rand.IntN(2) == 1 {
			reporter.Send(reportcard.TaskResultMsg{
				Status:              reportcard.Success,
				Index:               appId,
				CustomSuccessStatus: createCustomTaskStatusFromResult(result{PassedPolicy: true, PolicyStatus: ""}, false),
			})

			if options.ScanType == ScanTypePolicy {
				reporter.Send(reportcard.TaskResultMsg{
					Status:              reportcard.Success,
					Index:               appId,
					CustomSuccessStatus: createCustomTaskStatusFromResult(result{PassedPolicy: false, PolicyStatus: "Pass"}, true),
				})
			}

		} else {
			reporter.Send(reportcard.TaskResultMsg{
				Status:              reportcard.Success,
				Index:               appId,
				CustomSuccessStatus: createCustomTaskStatusFromResult(result{PassedPolicy: false, PolicyStatus: ""}, false),
			})

			if options.ScanType == ScanTypePolicy {
				reporter.Send(reportcard.TaskResultMsg{
					Status:              reportcard.Success,
					Index:               appId,
					CustomSuccessStatus: createCustomTaskStatusFromResult(result{PassedPolicy: false, PolicyStatus: "Conditional Pass"}, true),
				})
			}
		}
	}
}

func Promote_ui(cCtx *cli.Context) error {
	c, err := ReadConfig(cCtx.Path("c"))
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	// Setup mock http server and client

	server := newVeracodeMockServer(c)
	defer server.Close()

	client, err := newVeracodeMockClient(strings.Replace(server.URL, "https://", "", 1))
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	startChan := make(chan struct{})
	var m tea.Model
	ctx := context.Background()

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
			singleselect.WithHelp(defaultHelp),
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
					packageAndUploadApplication_ui(app, k, p, client, ctx)
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

func Policy_ui(cCtx *cli.Context) error {
	// Load & validate config and handle sandbox edge cases
	configPath, err := checkConfigPath(cCtx.Path("c"))
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	c, err := ReadConfig(configPath)
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	ctx := context.Background()

	// Setup mock http server and client
	server := newVeracodeMockServer(c)
	defer server.Close()

	client, err := newVeracodeMockClient(strings.Replace(server.URL, "https://", "", 1))
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	for k := range c.Applications {
		c.Applications[k].ScanType = ScanTypePolicy
	}

	p := tea.NewProgram(PrepareReportCard(c))

	for k, app := range c.Applications {
		go func() {
			packageAndUploadApplication_ui(app, k, p, client, ctx)
		}()
	}

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func RefreshCredentials_ui(cCtx *cli.Context) error {
	configPath, err := checkConfigPath(cCtx.Path("c"))
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	c, err := ReadConfig(configPath)
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	server := newVeracodeMockServer(c)
	defer server.Close()

	client, err := newVeracodeMockClient(strings.Replace(server.URL, "https://", "", 1))
	if err != nil {
		fmt.Print(renderErrors(err))
		return err
	}

	p := tea.NewProgram(NewCredentialsRefreshModel(client, ""))
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func ConfigureCredentials_ui(cCtx *cli.Context) error {
	p := tea.NewProgram(NewCredentialsConfigureModel(NewCredentialsTask(func() (string, string, error) { return "", "", nil }), ""))
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func Update_ui(cCtx *cli.Context) error {
	p := tea.NewProgram(PrepareUpdate([]multistagesetup.SetupTask{
		multistagesetup.NewSetupTask("Check prerequisites", NewSimpleTask(func(values map[string]any) tea.Cmd {
			time.Sleep(200 * time.Millisecond)
			return func() tea.Msg { return multistagesetup.NewSuccessfulTaskResult("", nil) }
		})),
		multistagesetup.NewSetupTask("Update Veracode CLI", NewSimpleTask(func(values map[string]any) tea.Cmd {
			return func() tea.Msg {
				time.Sleep(2 * time.Second)
				return multistagesetup.NewSuccessfulTaskResult("successfully updated: 2.39.0 -> 2.40.0", nil)
			}
		})),
		multistagesetup.NewSetupTask("Update Veracode Uploader", NewSimpleTask(func(values map[string]any) tea.Cmd {
			return func() tea.Msg {
				time.Sleep(400 * time.Millisecond)
				return multistagesetup.NewSkippedTaskResult("already on the latest version: 24.10.15.0", nil)
			}
		})),
		multistagesetup.NewSetupTask("Install SCA Agent", NewSimpleTask(func(values map[string]any) tea.Cmd {
			return func() tea.Msg {
				time.Sleep(1 * time.Second)
				return multistagesetup.NewSuccessfulTaskResult("successfully installed", nil)
			}
		})),
	},
	),
	)
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

type versions struct {
	PackagerLocalVersion      string `yaml:"packager_local_version"`
	PackagerLatestVersion     string `yaml:"packager_latest_version"`
	PackagerLatestVersionFunc func() (string, error)

	UploaderLocalVersion      string `yaml:"uploader_local_version"`
	UploaderLatestVersion     string `yaml:"uploader_latest_version"`
	UploaderLatestVersionFunc func() (string, error)
}

func loadVersions(argPath string) versions {
	v := versions{
		PackagerLocalVersion: "na",
		UploaderLocalVersion: "na",
	}

	if argPath != "" {
		f, err := os.ReadFile(argPath)
		if err == nil {
			yaml.Unmarshal(f, &v)
		}
	} else if _, err := os.Stat("versions.yaml"); err == nil {
		f, err := os.ReadFile("versions.yaml")
		if err == nil {
			err = yaml.Unmarshal(f, &v)
		}
	} else {
		v.PackagerLocalVersion = GetLocalVersion(filepath.Join(getPackagerLocation(), "VERSION"))
		v.UploaderLocalVersion = GetLocalVersion(filepath.Join(getWrapperLocation(), "VERSION"))
	}

	jar, _ := cookiejar.New(&cookiejar.Options{})

	httpClient := &http.Client{
		Jar: jar,
	}

	v.UploaderLatestVersionFunc = func() (string, error) {
		if v.UploaderLatestVersion == "" {
			return GetLatestUploaderVersion(httpClient)
		} else {
			time.Sleep(600 * time.Millisecond)
			return v.UploaderLatestVersion, nil
		}
	}

	v.PackagerLatestVersionFunc = func() (string, error) {
		if v.PackagerLatestVersion == "" {
			baseURL, _ := url.Parse("https://tools.veracode.com/veracode-cli")
			return GetLatestPackagerVersion(httpClient, baseURL)
		} else {
			time.Sleep(600 * time.Millisecond)
			return v.PackagerLatestVersion, nil
		}
	}

	return v
}

func VersionPrinter_ui(cCtx *cli.Context) {
	v := loadVersions(cCtx.Path("y"))

	if v.UploaderLatestVersionFunc == nil || v.PackagerLatestVersionFunc == nil {
		fmt.Print("funcs are nil")
		return
	}

	m := version.NewModel(
		v.UploaderLocalVersion,
		v.PackagerLocalVersion,
		cCtx.App.Version,
		cCtx.App.Name,
		v.UploaderLatestVersionFunc,
		v.PackagerLatestVersionFunc,
		version.WithSpinner([]spinner.Option{
			spinner.WithSpinner(spinner.Spinner{
				Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
				FPS:    time.Second / 10,
			}),
			spinner.WithStyle(darkGrayForeground),
		}...),
		version.WithHelp(defaultHelp),
		version.WithStyles(version.Styles{
			Muted: darkGrayForeground,
			Loud:  lipgloss.NewStyle().Foreground(orange),
		}),
	)

	tea.NewProgram(m).Run()
}

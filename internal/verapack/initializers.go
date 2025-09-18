package verapack

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"path/filepath"

	"github.com/DanCreative/veracode-go/veracode"
	"github.com/DanCreative/verapack/internal/components/multistagesetup"
	"github.com/DanCreative/verapack/internal/components/reportcard"
	"github.com/charmbracelet/lipgloss"
)

const (
	columnPackage string = "Package"
	columnUpload  string = "Upload"
	columnCleanup string = "Cleanup"
	columnResult  string = "Result"
	columnPolicy  string = "Policy"
	columnPromote string = "Promote"
)

func NewVeracodeClient() (*veracode.Client, error) {
	key, secret, err := veracode.LoadVeracodeCredentials()
	if err != nil {
		return nil, err
	}

	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Jar: jar,
	}

	client, err := veracode.NewClient(httpClient, key, secret)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func PrepareReportCard(c Config) reportcard.Model {
	columnOptions := getColumns(c)
	rowOptions := getRows(c, columnOptions)

	return reportcard.NewModel(
		reportcard.WithSpinner(defaultSpinnerOpts...),
		reportcard.WithStyles(reportcard.Styles{
			Headers:  lipgloss.NewStyle().Bold(true).Padding(0, 1),
			Cell:     lipgloss.NewStyle().Padding(0, 1),
			Border:   lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(darkGray).Padding(0, 2).MarginLeft(2),
			Selected: lipgloss.NewStyle().Foreground(lightBlue),
		}),
		reportcard.WithHelp(defaultHelp),
		reportcard.WithViewportDimensions(0.6, 0.3),
		reportcard.WithData(rowOptions...),
		reportcard.WithTasks(columnOptions),
		reportcard.WithPrefixColumns([]reportcard.Column{{Name: "Scan Type", Width: 9}}),
	)
}

func PrepareSetup(appDir string, tasks []multistagesetup.SetupTask) multistagesetup.Model {
	return multistagesetup.NewModel(
		multistagesetup.WithSpinner(defaultSpinnerOpts...),
		multistagesetup.WithHelp(defaultHelp),
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
	)
}

func PrepareUpdate(tasks []multistagesetup.SetupTask) multistagesetup.Model {
	return multistagesetup.NewModel(
		multistagesetup.WithHelp(defaultHelp),
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
		multistagesetup.WithTasks(tasks...),
	)
}

func hasPromoteTask(c Options) bool {
	return c.ScanType == ScanTypePromote || (c.ScanType == ScanTypeSandbox && c.AutoPromote)
}

func hasPackageTask(c Options) bool {
	return (c.ScanType == ScanTypePolicy || c.ScanType == ScanTypeSandbox) && c.PackageSource != ""
}

func hasCleanupTask(c Options) bool {
	return (c.ScanType == ScanTypePolicy || c.ScanType == ScanTypeSandbox) && c.PackageSource != "" && *c.AutoCleanup
}

func hasResultTask(c Options) bool {
	return (c.ScanType == ScanTypePolicy || c.ScanType == ScanTypeSandbox) && c.WaitForResult || c.AutoPromote
}

func hasUploadTask(c Options) bool {
	return c.ScanType == ScanTypePolicy || c.ScanType == ScanTypeSandbox
}

func hasPolicyTask(c Options) bool {
	return (c.ScanType == ScanTypePolicy && c.WaitForResult) || (c.ScanType == ScanTypeSandbox && c.AutoPromote)
}

func getColumns(c Config) []reportcard.Column {
	columnsOption := make([]reportcard.Column, 0, 6)
	var columnPromoteAdd, columnPackageAdd, columnCleanupAdd, columnResultAdd, columnUploadAdd, columnPolicyAdd bool

	for _, app := range c.Applications {
		columnPackageAdd = columnPackageAdd || hasPackageTask(app)
		columnUploadAdd = columnUploadAdd || hasUploadTask(app)
		columnCleanupAdd = columnCleanupAdd || hasCleanupTask(app)
		columnResultAdd = columnResultAdd || hasResultTask(app)
		columnPolicyAdd = columnPolicyAdd || hasPolicyTask(app)
		columnPromoteAdd = columnPromoteAdd || hasPromoteTask(app)
	}

	if columnPackageAdd {
		columnsOption = append(columnsOption, reportcard.Column{Name: columnPackage, Width: 7})
	}

	if columnUploadAdd {
		columnsOption = append(columnsOption, reportcard.Column{Name: columnUpload, Width: 6})
	}

	if columnCleanupAdd {
		columnsOption = append(columnsOption, reportcard.Column{Name: columnCleanup, Width: 7})
	}

	if columnResultAdd {
		columnsOption = append(columnsOption, reportcard.Column{Name: columnResult, Width: 6})
	}

	if columnPromoteAdd {
		columnsOption = append(columnsOption, reportcard.Column{Name: columnPromote, Width: 7})
	}

	if columnPolicyAdd {
		columnsOption = append(columnsOption, reportcard.Column{Name: columnPolicy, Width: 8})
	}

	return columnsOption
}

func getRows(c Config, columns []reportcard.Column) []reportcard.Row {
	rowOptions := make([]reportcard.Row, 0, len(c.Applications))

	for _, app := range c.Applications {
		tasks := make([]reportcard.Task, 0, 6)

		if hasPackageTask(app) {
			tasks = append(tasks, reportcard.NewTask(columnPackage))
		}

		if hasUploadTask(app) {
			tasks = append(tasks, reportcard.NewTask(columnUpload))
		}

		if hasCleanupTask(app) {
			tasks = append(tasks, reportcard.NewTask(columnCleanup, columnPackage, columnUpload))
		}

		if hasResultTask(app) {
			tasks = append(tasks, reportcard.NewTask(columnResult))
		}

		if hasPolicyTask(app) {
			tasks = append(tasks, reportcard.NewTask(columnPolicy))
		}

		if hasPromoteTask(app) {
			tasks = append(tasks, reportcard.NewTask(columnPromote))
		}

		rowOptions = append(rowOptions, reportcard.NewRow(app.AppName, tasks, []string{string(app.ScanType)}, columns))
	}

	return rowOptions
}

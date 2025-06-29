package verapack

import (
	"net/http"
	"net/http/cookiejar"

	"github.com/DanCreative/veracode-go/veracode"
	"github.com/DanCreative/verapack/internal/components/reportcard"
	"github.com/charmbracelet/lipgloss"
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
	var somePromoting, someWaiting, somePolicy, someSandbox bool
	// var totalPromoting, totalPolicy, totalWaiting int

	for k := range c.Applications {
		switch c.Applications[k].ScanType {
		case ScanTypeSandbox:
			someSandbox = true
		case ScanTypePolicy:
			// totalPolicy++
			somePolicy = true
		case ScanTypePromote:
			// totalPromoting++
			somePromoting = true
		}

		if c.Applications[k].WaitForResult {
			someWaiting = true
			// totalWaiting++
		}
	}

	// var columnsOption []reportcard.Column
	var rowsOption []reportcard.Row
	columnsOption := make([]reportcard.Column, 0, 5)

	if somePolicy || someSandbox {
		columnsOption = append(columnsOption, reportcard.Column{Name: columnPackage, Width: 7}, reportcard.Column{Name: columnUpload, Width: 6}, reportcard.Column{Name: columnCleanup, Width: 7})
	}

	if someWaiting {
		columnsOption = append(columnsOption, reportcard.Column{Name: columnResult, Width: 6})
	}

	if somePromoting {
		columnsOption = append(columnsOption, reportcard.Column{Name: columnPromote, Width: 7})
	}

	if somePolicy && someWaiting {
		columnsOption = append(columnsOption, reportcard.Column{Name: columnPolicy, Width: 8})
	}

	rowsOption = appsToRows(c.Applications, columnsOption)

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

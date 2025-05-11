package verapack

import (
	"github.com/DanCreative/verapack/internal/components/reportcard"
)

type columnOptions int

const (
	columnOptionsStandard columnOptions = iota
	columnOptionsPromote
	columnOptionsMixed
)

func appsToRows(applications []Options, columnOptions columnOptions) []reportcard.Row {
	rows := make([]reportcard.Row, 0, len(applications))

	for _, application := range applications {
		var row reportcard.Row
		switch columnOptions {
		case columnOptionsStandard:
			row = standardRow(application)
		case columnOptionsPromote:
			row = promoteRow(application)
		case columnOptionsMixed:
			row = mixedRow(application)
		}
		rows = append(rows, row)
	}

	return rows
}

func standardRow(application Options) reportcard.Row {
	row := reportcard.Row{
		Name: application.AppName,
		// Package, Scan, Cleanup
		Tasks: []reportcard.Task{
			{Status: reportcard.NotStarted},
			{Status: reportcard.NotStarted},
			// Cleanup should show "running" even if scan fails, but not if packaging fails.
			{Status: reportcard.NotStarted, ShouldRunAnywayFor: map[int]bool{1: true}},
		},
		PrefixValues: []string{string(application.ScanType)},
	}

	if application.PackageSource == "" {
		// If PackageSource is empty, indicate that packaging task will be skipped
		row.Tasks[0].Status = reportcard.Skip
	}

	if !*application.AutoCleanup || application.PackageSource == "" {
		// If AutoCleanup is false, indicate that cleanup task will be skipped
		row.Tasks[2].Status = reportcard.Skip
	}
	return row
}

func promoteRow(application Options) reportcard.Row {
	row := reportcard.Row{
		Name: application.AppName,
		// Promote
		Tasks: []reportcard.Task{
			{Status: reportcard.NotStarted},
		},
		PrefixValues: []string{string(application.ScanType)},
	}

	return row
}

func mixedRow(application Options) reportcard.Row {
	var row reportcard.Row

	switch application.ScanType {
	case ScanTypeSandbox, ScanTypePolicy:
		row = reportcard.Row{
			Name: application.AppName,
			// Package, Scan, Cleanup, Promote
			Tasks: []reportcard.Task{
				{Status: reportcard.NotStarted},
				{Status: reportcard.NotStarted},
				// Cleanup should show "running" even if scan fails, but not if packaging fails.
				{Status: reportcard.NotStarted, ShouldRunAnywayFor: map[int]bool{1: true}},
				{Status: reportcard.Skip},
			},
			PrefixValues: []string{string(application.ScanType)},
		}

		if application.PackageSource == "" {
			// If PackageSource is empty, indicate that packaging task will be skipped
			row.Tasks[0].Status = reportcard.Skip
		}

		if !*application.AutoCleanup || application.PackageSource == "" {
			// If AutoCleanup is false, indicate that cleanup task will be skipped
			row.Tasks[2].Status = reportcard.Skip
		}
	case ScanTypePromote:
		row = reportcard.Row{
			Name: application.AppName,
			// Package, Scan, Cleanup, Promote
			Tasks: []reportcard.Task{
				{Status: reportcard.Skip},
				{Status: reportcard.Skip},
				// Cleanup should show "running" even if scan fails, but not if packaging fails.
				{Status: reportcard.Skip},
				{Status: reportcard.NotStarted},
			},
			PrefixValues: []string{string(application.ScanType)},
		}
	}

	return row
}

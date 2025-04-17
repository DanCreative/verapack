package verapack

import (
	"github.com/DanCreative/verapack/internal/components/reportcard"
)

func appsToRows(applications []Options) []reportcard.Row {
	rows := make([]reportcard.Row, 0, len(applications))

	for _, application := range applications {
		row := reportcard.Row{
			Name: application.AppName,
			// Package, Scan, Cleanup
			Tasks: []reportcard.Task{
				{Status: reportcard.NotStarted},
				{Status: reportcard.NotStarted},
				{Status: reportcard.NotStarted, ShouldRunAnyway: true},
			},
		}

		if application.PackageSource == "" {
			// If PackageSource is empty, indicate that packaging task will be skipped
			row.Tasks[0].Status = reportcard.Skip
		}

		if !application.AutoCleanup || application.PackageSource == "" {
			// If AutoCleanup is false, indicate that cleanup task will be skipped
			row.Tasks[2].Status = reportcard.Skip
		}

		rows = append(rows, row)
	}

	return rows
}

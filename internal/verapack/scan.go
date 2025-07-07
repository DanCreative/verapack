package verapack

import (
	"github.com/DanCreative/verapack/internal/components/reportcard"
)

const (
	columnPromote string = "Promote"
	columnPackage string = "Package"
	columnCleanup string = "Cleanup"
	columnResult  string = "Result"
	columnUpload  string = "Upload"
	columnPolicy  string = "Policy"
)

func appsToRows(applications []Options, columns []reportcard.Column) []reportcard.Row {
	rows := make([]reportcard.Row, 0, len(applications))

	for _, application := range applications {
		row := reportcard.Row{
			Name:         application.AppName,
			PrefixValues: []string{string(application.ScanType)},
			Tasks:        setTaskStatuses(application, columns),
		}

		rows = append(rows, row)
	}

	return rows
}

func setTaskStatuses(application Options, columns []reportcard.Column) []reportcard.Task {
	tasks := make([]reportcard.Task, len(columns))

	for k := range tasks {
		tasks[k].Status = reportcard.NotStarted
	}

	for k, col := range columns {
		switch application.ScanType {
		case ScanTypeSandbox, ScanTypePolicy:
			if col.Name == columnPackage && application.PackageSource == "" {
				// If PackageSource is empty, indicate that packaging task will be skipped.
				tasks[k].Status = reportcard.Skip
			}

			if col.Name == columnCleanup {
				if !*application.AutoCleanup || application.PackageSource == "" {
					// If AutoCleanup is false or user is not using the auto-packager, indicate that cleanup task will be skipped.
					tasks[k].Status = reportcard.Skip
				} else {
					// Cleanup is run after the packaging and upload completes, regardless of whether those tasks are successful.
					tasks[k].ShouldRunAnywayFor = map[int]bool{0: true, 1: true}
				}
			}

			if col.Name == columnResult && !application.WaitForResult && !application.AutoPromote {
				// For applications where wait_for_results is set to false, indicate that it will be skipped.
				tasks[k].Status = reportcard.Skip
			}

			if col.Name == columnPolicy && (!application.WaitForResult || application.ScanType == ScanTypeSandbox) && (!application.AutoPromote || application.ScanType == ScanTypePolicy) {
				// For applications where wait_for_results is set to false or that are Policy scans, indicate that it will be skipped.
				tasks[k].Status = reportcard.Skip
			}

			if col.Name == columnPromote && (!application.AutoPromote || application.ScanType == ScanTypePolicy) {
				// For applications where auto_promote is set to false or sandbox scans, indicate that it will be skipped.
				tasks[k].Status = reportcard.Skip
			}

		case ScanTypePromote:
			if col.Name == columnPackage || col.Name == columnCleanup || col.Name == columnResult || col.Name == columnUpload || col.Name == columnPolicy {
				tasks[k].Status = reportcard.Skip
			}
		}
	}

	return tasks
}

package verapack

import (
	"context"
	"fmt"

	"github.com/DanCreative/veracode-go/veracode"
	sand "github.com/DanCreative/verapack/internal/components/middleware/sandbox"
	"github.com/DanCreative/verapack/internal/components/reportcard"
	"github.com/charmbracelet/lipgloss"
)

// HandleSandboxNotProvided returns a []*Options of applications that do not have SandboxName set,
// it also mutates the provided applications, and sets a default scan type for them.
func HandleSandboxNotProvided(applications []Options, defaultScanType ScanType) []*Options {
	badApps := make([]*Options, 0)

	for k := range applications {
		if applications[k].SandboxName == "" {
			badApps = append(badApps, &applications[k])
		} else {
			applications[k].ScanType = defaultScanType
		}
	}

	return badApps
}

// renderBodyText returns the body text specifically for the sandbox loading tea component.
func renderBodyText(applications []*Options) string {
	var apps string
	for k := range applications {
		apps += darkGrayForeground.Render("\n\t•  " + applications[k].AppName)
	}

	return fmt.Sprintf("Below applications are missing field: %s\n%s\n\nWhat would you like to do?",
		lipgloss.NewStyle().Bold(true).Inline(true).Render("sandbox_name"),
		apps,
	)
}

// promoteSandbox finds the application profile and sandbox details
func promoteSandbox(client *veracode.Client, ctx context.Context, app Options, appId int, reporter reporter) {
	// // Find application profile with name
	// profiles, _, err := client.Application.ListApplications(ctx, veracode.ListApplicationOptions{Name: app.AppName})
	// if err != nil {
	// 	reporter.Send(reportcard.TaskResultMsg{
	// 		Status:  reportcard.Failure,
	// 		Output:  err.Error(),
	// 		Index:   appId,
	// 		IsFatal: true,
	// 	})
	// 	return
	// }

	// if len(profiles) == 0 {
	// 	// Could not find an application profile with name
	// 	reporter.Send(reportcard.TaskResultMsg{
	// 		Status:  reportcard.Failure,
	// 		Output:  fmt.Sprintf("no application profile found with name: '%s'", app.AppName),
	// 		Index:   appId,
	// 		IsFatal: true,
	// 	})
	// 	return
	// }

	// if len(profiles) > 1 {
	// 	reporter.Send(reportcard.TaskResultMsg{
	// 		Status:  reportcard.Failure,
	// 		Output:  fmt.Sprintf("more than 1 application profile found with name: '%s'", app.AppName),
	// 		Index:   appId,
	// 		IsFatal: true,
	// 	})
	// 	return
	// }

	// sandbox, _, err := client.Sandbox.GetSandbox(ctx, profiles[0].Guid, app.SandboxGuid)
	// if err != nil {
	// 	reporter.Send(reportcard.TaskResultMsg{
	// 		Status:  reportcard.Failure,
	// 		Output:  err.Error(),
	// 		Index:   appId,
	// 		IsFatal: true,
	// 	})
	// 	return
	// }

	_, _, err := client.Sandbox.PromoteSandbox(ctx, app.AppGuid, app.SandboxGuid, true)
	if err != nil {
		reporter.Send(reportcard.TaskResultMsg{
			Status:  reportcard.Failure,
			Output:  err.Error(),
			Index:   appId,
			IsFatal: true,
		})
		return
	}

	reporter.Send(reportcard.TaskResultMsg{
		Status: reportcard.Success,
		Index:  appId,
		Output: "",
	})
}

// appsToSandboxOptions creates new sandbox.SandboxOptions for the provided application that have ScanType ScanTypeSandbox or ScanTypePromote.
//
// Certain SandboxOptions fields are pointers that will directly mutate the original config values.
func appsToSandboxOptions(applications []Options) []sand.SandboxOptions {
	r := make([]sand.SandboxOptions, 0, len(applications))
	for k := range applications {
		if applications[k].ScanType != ScanTypePolicy {
			r = append(r, sand.SandboxOptions{
				AppName:     applications[k].AppName,
				AppGuid:     &applications[k].AppGuid,
				SandboxName: applications[k].SandboxName,
				SandboxId:   &applications[k].SandboxId,
				SandboxGuid: &applications[k].SandboxGuid,
			})
		}
	}

	return r
}

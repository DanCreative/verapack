package verapack

import (
	"context"
	"os"
	"path/filepath"

	"github.com/DanCreative/veracode-go/veracode"
	"github.com/DanCreative/verapack/internal/components/reportcard"
	tea "github.com/charmbracelet/bubbletea"
)

type reporter interface {
	Send(msg tea.Msg)
}

// packageAndUploadApplication combines the packaging and uploading into one function.
//
// If the PackageSource is set, then the packager will be run and artefactsPath will be set.
//
// UploadAndScanApplication requires ArtefactPaths to be set. Either it or PackageSource needs
// to be set in the config. If PackageSource is set, PackageApplication will be run and set it.
//
// TODO: Implement log output
func packageAndUploadApplication(uploaderPath string, options Options, appId int, reporter reporter, client *veracode.Client, ctx context.Context) error {
	var err error

	// packageOutputBaseDirectory is the path to the individual apps' temp folder.
	// It will contain a source clone folder and an artefact output folder.
	var packageOutputBaseDirectory string

	if options.PackageSource != "" {
		// Run the auto-packager

		packageOutputBaseDirectory, err = createAppPackagingOutputDir(options.AppName)
		if err != nil {
			reporter.Send(reportcard.TaskResultMsg{
				Status:  reportcard.Failure,
				Output:  err.Error(),
				Index:   appId,
				IsFatal: true,
			})
			return err
		}

		cloneOut, err := CloneRepository(options, filepath.Join(packageOutputBaseDirectory, "source"), nil)
		if err != nil {
			reporter.Send(reportcard.TaskResultMsg{
				Status:  reportcard.Failure,
				Output:  cloneOut,
				Index:   appId,
				IsFatal: true,
			})
			cleanupTask(options, packageOutputBaseDirectory, appId, reporter)
			return err
		}

		// The source code retrieved from the git shallow clone will be used as the input
		// for the packaging.
		// Regardless of whether the original source was repo or dir, the packager is told
		// to use dir.
		options.PackageSource = filepath.Join(packageOutputBaseDirectory, "source")
		options.Type = Directory

		artefactPaths, out, err := PackageApplication(options, filepath.Join(packageOutputBaseDirectory, "out"), nil)

		if *options.Verbose {
			out = cloneOut + out
		}

		if err != nil {
			reporter.Send(reportcard.TaskResultMsg{
				Status:  reportcard.Failure,
				Output:  out,
				Index:   appId,
				IsFatal: true,
			})
			cleanupTask(options, packageOutputBaseDirectory, appId, reporter)
			return err
		}

		reporter.Send(reportcard.TaskResultMsg{
			Status: reportcard.Success,
			Index:  appId,
			Output: out,
		})

		options.ArtefactPaths = artefactPaths
	}

	options.UploaderFilePath = uploaderPath

	out, err := UploadAndScanApplication(options, nil)
	if err != nil {
		reporter.Send(reportcard.TaskResultMsg{
			Status:  reportcard.Failure,
			Output:  out,
			Index:   appId,
			IsFatal: true,
		})
		cleanupTask(options, packageOutputBaseDirectory, appId, reporter)
		return err
	}

	reporter.Send(reportcard.TaskResultMsg{
		Status: reportcard.Success,
		Index:  appId,
		Output: out,
	})

	cleanupTask(options, packageOutputBaseDirectory, appId, reporter)

	if options.WaitForResult {
		err = waitForResultTask(ctx, client, options, appId, reporter)
		if err != nil {
			return err
		}
	}

	return nil
}

func cleanupTask(options Options, packageOutputBaseDirectory string, appId int, reporter reporter) {
	if *options.AutoCleanup && options.PackageSource != "" {
		err := os.RemoveAll(packageOutputBaseDirectory)
		if err != nil {
			reporter.Send(reportcard.TaskResultMsg{
				Status:  reportcard.Failure,
				Output:  err.Error(),
				Index:   appId,
				IsFatal: false,
			})
		} else {
			reporter.Send(reportcard.TaskResultMsg{
				Status: reportcard.Success,
				Index:  appId,
			})
		}
	}
}

func waitForResultTask(ctx context.Context, client *veracode.Client, options Options, appId int, reporter reporter) error {
	result, out, err := WaitForResult(ctx, client, options, reporter)
	if err != nil {
		reporter.Send(reportcard.TaskResultMsg{
			Status:  reportcard.Failure,
			Output:  out,
			Index:   appId,
			IsFatal: true,
		})

		return err
	}

	// taskResult is re-used to send both the custom status for the Result column and the Policy column (if it is a policy scan)
	taskResult := reportcard.TaskResultMsg{
		Status: reportcard.Success,
		Output: out,
		Index:  appId,
	}

	taskResult.CustomSuccessStatus = createCustomTaskStatusFromResult(result, false)
	reporter.Send(taskResult)

	if options.ScanType == ScanTypePolicy {
		taskResult.CustomSuccessStatus = createCustomTaskStatusFromResult(result, true)
		reporter.Send(taskResult)
	}

	return nil
}

func createCustomTaskStatusFromResult(result result, isPolicyStatus bool) reportcard.CustomTaskStatus {
	if isPolicyStatus {
		switch result.PolicyStatus {
		case "Conditional Pass":
			return reportcard.CustomTaskStatus{Message: "⛊ C.PASS", ForegroundColour: "#ff7c01"}
		case "Pass":
			return reportcard.CustomTaskStatus{Message: "⛊ PASS", ForegroundColour: "#20BA44"}
		case "Did Not Pass":
			return reportcard.CustomTaskStatus{Message: "⛊ FAIL", ForegroundColour: "#DD3A34"}
		}
	} else {
		if result.PassedPolicy {
			return reportcard.CustomTaskStatus{Message: "⛊ PASS", ForegroundColour: "#20BA44"}
		} else {
			return reportcard.CustomTaskStatus{Message: "⛊ FAIL", ForegroundColour: "#DD3A34"}
		}
	}

	return reportcard.CustomTaskStatus{}
}

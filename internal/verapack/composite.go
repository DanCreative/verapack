package verapack

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/DanCreative/veracode-go/veracode"
	"github.com/DanCreative/verapack/internal/components/reportcard"
	"github.com/charmbracelet/bubbles/runeutil"
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
func packageAndUploadApplication(uploaderPath string, options Options, appId int, reporter reporter, client *veracode.Client, ctx context.Context) error {
	var err error
	sanitizer := runeutil.NewSanitizer()

	// packageOutputBaseDirectory is the path to the individual apps' temp folder.
	// It will contain a source clone folder and an artefact output folder.
	var packageOutputBaseDirectory string

	logWriter, closeFunc, err := initializeLogWriter(options.AppName)
	if err != nil {
		reporter.Send(reportcard.TaskResultMsg{
			Status: reportcard.Failure,
			Output: err.Error(),
			Index:  appId,
		})
		return err
	}

	defer closeFunc()

	if options.PackageSource != "" {
		// Run the auto-packager

		fmt.Fprintf(logWriter, "BEGIN (%s)\n", columnPackage)

		packageOutputBaseDirectory, err = createAppPackagingOutputDir(options.AppName)
		if err != nil {
			reporter.Send(reportcard.TaskResultMsg{
				Status: reportcard.Failure,
				Output: err.Error(),
				Index:  appId,
			})
			return err
		}

		var cloneOut string

		if options.Type == Repo || options.Branch != "" {
			// Perform a shallow clone of a git repository.
			// If the git repo is remote, this will always run.
			// If the git repo is local, it will only be run if [Options].Branch is set.
			cloneOut, err = CloneRepository(options, filepath.Join(packageOutputBaseDirectory, "source"), logWriter)
			if err != nil {
				reporter.Send(reportcard.TaskResultMsg{
					Status: reportcard.Failure,
					Output: string(sanitizer.Sanitize([]rune(cloneOut))),
					Index:  appId,
				})
				cleanupTask(options, packageOutputBaseDirectory, appId, reporter, logWriter)
				return err
			}

			// Regardless of whether the original source was repo or dir, the packager is told
			// to use dir.
			options.PackageSource = filepath.Join(packageOutputBaseDirectory, "source")
			options.Type = Directory
		}

		artefactPaths, out, err := PackageApplication(options, filepath.Join(packageOutputBaseDirectory, "out"), logWriter)
		fmt.Fprintf(logWriter, "END (%s)\n", columnPackage)

		if *options.Verbose {
			out = cloneOut + out
		}

		if err != nil {
			reporter.Send(reportcard.TaskResultMsg{
				Status: reportcard.Failure,
				Output: string(sanitizer.Sanitize([]rune(out))),
				Index:  appId,
			})
			cleanupTask(options, packageOutputBaseDirectory, appId, reporter, logWriter)
			return err
		}
		var packageStatus reportcard.TaskStatus

		if logWriter.ContainsWarning() {
			packageStatus = reportcard.Warning
		} else {
			packageStatus = reportcard.Success
		}

		reporter.Send(reportcard.TaskResultMsg{
			Status: packageStatus,
			Index:  appId,
			Output: out,
		})

		options.ArtefactPaths = artefactPaths
	}

	options.UploaderFilePath = uploaderPath

	out, err := UploadAndScanApplication(options, logWriter)
	if err != nil {
		reporter.Send(reportcard.TaskResultMsg{
			Status: reportcard.Failure,
			Output: string(sanitizer.Sanitize([]rune(out))),
			Index:  appId,
		})
		cleanupTask(options, packageOutputBaseDirectory, appId, reporter, logWriter)
		return err
	}

	reporter.Send(reportcard.TaskResultMsg{
		Status: reportcard.Success,
		Index:  appId,
		Output: string(sanitizer.Sanitize([]rune(out))),
	})

	cleanupTask(options, packageOutputBaseDirectory, appId, reporter, logWriter)

	shouldAutoPromote := options.AutoPromote && options.ScanType == ScanTypeSandbox

	if shouldAutoPromote {
		err = autoPromoteTask(ctx, client, options, appId, reporter, logWriter)
		if err != nil {
			return err
		}
	}

	if options.WaitForResult && !shouldAutoPromote {
		err = waitForResultTask(ctx, client, options, appId, reporter)
		if err != nil {
			fmt.Fprintf(logWriter, "BEGIN (%s)\n%s\nEND (%s)\n", columnResult, err, columnResult)
			return err
		}
	}

	return nil
}

func cleanupTask(options Options, packageOutputBaseDirectory string, appId int, reporter reporter, writer io.Writer) {
	if *options.AutoCleanup && options.PackageSource != "" {
		err := os.RemoveAll(packageOutputBaseDirectory)
		if err != nil {
			fmt.Fprintf(writer, "BEGIN (%s)\n%s\nEND (%s)\n", columnCleanup, err, columnCleanup)

			reporter.Send(reportcard.TaskResultMsg{
				Status: reportcard.Failure,
				Output: err.Error(),
				Index:  appId,
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
			Status: reportcard.Failure,
			Output: out,
			Index:  appId,
		})

		return err
	}

	// taskResult is re-used to send both the custom status for the Result column and the Policy column (if it is a policy scan)
	taskResult := reportcard.TaskResultMsg{
		Status: reportcard.Success,
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

func autoPromoteTask(ctx context.Context, client *veracode.Client, options Options, appId int, reporter reporter, writer io.Writer) error {
	res, out, err := WaitForResult(ctx, client, options, reporter)
	if err != nil {
		fmt.Fprintf(writer, "BEGIN (%s)\n%s\nEND (%s)\n", columnResult, err, columnResult)
		reporter.Send(reportcard.TaskResultMsg{
			Status: reportcard.Failure,
			Output: out,
			Index:  appId,
		})

		return err
	}

	// Result column
	taskResult := reportcard.TaskResultMsg{
		Status:              reportcard.Success,
		Index:               appId,
		CustomSuccessStatus: createCustomTaskStatusFromResult(res, false),
	}

	reporter.Send(taskResult)

	// Promote column
	if res.PassedPolicy {
		_, _, err := client.Sandbox.PromoteSandbox(ctx, options.AppGuid, options.SandboxGuid, true)
		if err != nil {
			fmt.Fprintf(writer, "BEGIN (%s)\n%s\nEND (%s)\n", columnPromote, err, columnPromote)
			reporter.Send(reportcard.TaskResultMsg{
				Status: reportcard.Failure,
				Index:  appId,
				Output: err.Error(),
			})
			return err
		}

		reporter.Send(reportcard.TaskResultMsg{
			Status: reportcard.Success,
			Index:  appId,
		})

	} else {
		fmt.Fprintf(writer, "BEGIN (%s)\n%s\nEND (%s)\n", columnPromote, err, columnPromote)
		reporter.Send(reportcard.TaskResultMsg{
			Status: reportcard.Failure,
			Index:  appId,
			Output: "The application did not pass the policy rules. Therefore auto-promotion was cancelled.",
		})

		return nil
	}

	// Policy column
	summaryReport, _, err := client.Application.GetSummaryReport(ctx, options.AppGuid, veracode.SummaryReportOptions{})
	if err != nil {
		fmt.Fprintf(writer, "BEGIN (%s)\n%s\nEND (%s)\n", columnPolicy, err, columnPolicy)
		reporter.Send(reportcard.TaskResultMsg{
			Status: reportcard.Failure,
			Index:  appId,
			Output: err.Error(),
		})
		return err
	}

	res = result{PassedPolicy: summaryReport.PolicyRulesStatus == "Pass", PolicyStatus: summaryReport.PolicyComplianceStatus}

	reporter.Send(reportcard.TaskResultMsg{
		Status:              reportcard.Success,
		Index:               appId,
		CustomSuccessStatus: createCustomTaskStatusFromResult(res, true),
	})

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

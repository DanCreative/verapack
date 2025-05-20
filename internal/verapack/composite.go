package verapack

import (
	"os"

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
func packageAndUploadApplication(uploaderPath string, options Options, appId int, reporter reporter) error {
	var err error
	if options.PackageSource != "" {
		// Options.OutputDir is only set here
		options.OutputDir, err = createAppPackagingOutputDir(options.AppName)
		if err != nil {
			reporter.Send(reportcard.TaskResultMsg{
				Status:  reportcard.Failure,
				Output:  err.Error(),
				Index:   appId,
				IsFatal: true,
			})
			return err
		}

		artefactPaths, out, err := PackageApplication(options, nil)
		if err != nil {

			reporter.Send(reportcard.TaskResultMsg{
				Status:  reportcard.Failure,
				Output:  out,
				Index:   appId,
				IsFatal: true,
			})
			return err
		}

		// Cleanup is only required if packager is run successfully, then it should be run
		// at the end.
		defer func() {
			if *options.AutoCleanup {
				os.RemoveAll(options.OutputDir)
			}

			reporter.Send(reportcard.TaskResultMsg{
				Status: reportcard.Success,
				Index:  appId,
			})
		}()

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
		return err
	}

	reporter.Send(reportcard.TaskResultMsg{
		Status: reportcard.Success,
		Index:  appId,
		Output: out,
	})
	return nil
}

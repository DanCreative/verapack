package verapack

import (
	"os"
	"path/filepath"

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
		// Run the auto-packager

		// packageOutputBaseDirectory is the path to the individual apps' temp folder.
		// It will contain a source clone folder and an artefact output folder.
		var packageOutputBaseDirectory string

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
			return err
		}

		// Cleanup is only required if packager is run successfully, then it should be run
		// at the end.
		defer func() {
			if *options.AutoCleanup {
				os.RemoveAll(packageOutputBaseDirectory)
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

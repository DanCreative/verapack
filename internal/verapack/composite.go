package verapack

// packageAndUploadApplication combines the packaging and uploading into one function.
//
// If the PackageSource is set, then the packager will be run and artefactsPath will be set.
//
// UploadAndScanApplication requires ArtefactPaths to be set. Either it or PackageSource needs
// to be set in the config. If PackageSource is set, PackageApplication will be run and set it.
func packageAndUploadApplication(uploaderPath string, options Options, appId int, reporter *ReportCard) error {
	var err error
	if options.PackageSource != "" {
		// Options.OutputDir is only set here
		options.OutputDir, err = createAppPackagingOutputDir(options.AppName)
		if err != nil {
			return err
		}

		reporter.Update(appId, "Package", InProgress, false)

		artefactPaths, err := PackageApplication(options)
		if err != nil {
			reporter.Update(appId, "Package", Failure, false)
			reporter.Update(appId, "Scan", Skip, true)
			return err
		}

		// Cleanup is only required if packager is run successfully, then it should be run
		// at the end.
		defer cleanup(options)

		reporter.Update(appId, "Package", Success, false)

		options.ArtefactPaths = artefactPaths
	}

	options.UploaderFilePath = uploaderPath

	reporter.Update(appId, "Scan", InProgress, false)

	err = UploadAndScanApplication(options)
	if err != nil {
		reporter.Update(appId, "Scan", Failure, true)
		return err
	}

	reporter.Update(appId, "Scan", Success, true)
	return nil
}

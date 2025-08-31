$ErrorActionPreference = 'Stop' # stop on all errors
$toolsDir   = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$version    = $env:chocolateyPackageVersion
$zipArchive = Join-Path $toolsDir -ChildPath "verapack-windows-amd64-v$version.zip"

$packageArgs = @{
  Destination    = $toolsDir
  FileFullPath   = $zipArchive
}

Get-ChocolateyUnzip @packageArgs
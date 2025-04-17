# Verapack

## 🏗 Building

To build the application, run below Powershell command from a Windows machine at the root of the repository.

``` Powershell
$env:VERSION = $(git describe --tags --abbrev=0);$env:GOOS = "windows";$env:GOARCH = "amd64";$env:PACKAGENAME = "verapack-$env:GOOS-$env:GOARCH-$env:VERSION";go build -o out\$env:PACKAGENAME\ -ldflags='-s -w -X "main.Version='$env:VERSION'"' .\cmd\verapack\;Compress-Archive -Path out\$env:PACKAGENAME\* -DestinationPath out\$env:PACKAGENAME.zip;Remove-Item -Path out\$env:PACKAGENAME\ -Recurse
```

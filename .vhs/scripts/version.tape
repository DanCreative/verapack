# README
# 
# This script produces the .gif recording for the version print action.
# Run this command from the root of the directory to run this script: ```vhs .vhs/scripts/version.tape```


# Setup
Output .vhs/output/version-print.gif
Set Width 1500
Set Height 350
Set FontSize 30
Set Shell "powershell"

Require go

# Build
Hide
Type `go build "-tags=ui" -ldflags='-X "main.Version='0.5.2-dev'"' -o . ./cmd/verapack`
Enter

# Add test data
Type "Copy-Item -Path .vhs\testdata\versions.yaml -Destination ."
Enter

Type "clear"
Enter

Sleep 1.5s
Show

# Run
Type ".\verapack -v"
Sleep 750ms
Enter

Sleep 5s

# Teardown

Hide
Type "Remove-Item -Path .\verapack.exe"
Enter

Type "Remove-Item -Path .\versions.yaml"
Enter
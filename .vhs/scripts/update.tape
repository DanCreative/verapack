# README
# 
# This script produces the .gif recording for the update action.
# Run this command from the root of the directory to run this script: ```vhs .vhs/scripts/update.tape```


# Setup
Output .vhs/output/update.gif
Set Width 1500
Set Height 450
Set FontSize 30
Set Shell "powershell"

Require go

# Build
Hide
Type `go build "-tags=ui" -ldflags='-X "main.Version='0.5.2-dev'"' -o . ./cmd/verapack`
Enter

Type "clear"
Enter

Sleep 1.5s
Show

# Run
Type ".\verapack update"
Sleep 750ms
Enter

Sleep 8s

# Teardown

Hide
Type "Remove-Item -Path verapack.exe"
Enter
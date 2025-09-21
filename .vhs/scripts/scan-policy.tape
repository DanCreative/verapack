# README
# 
# This script produces the .gif recording for the scan policy action.
# Run this command from the root of the directory to run this script: ```vhs .vhs/scripts/scan-policy.tape```


# Setup
Output .vhs/output/scan-policy.gif
Set Width 1700
Set Height 1000
Set FontSize 30
Set Shell "powershell"

Require go

# Build
Hide
Type `go build "-tags=ui" -ldflags='-X "main.Version='x.y.z-dev'"' -o . ./cmd/verapack`
Enter

# Add test data
Type "Copy-Item -Path .vhs\testdata\config.yaml -Destination ."
Enter

Type "clear"
Enter

Sleep 1.5s
Show

# Run
Type ".\verapack scan policy"
Sleep 750ms
Enter

Sleep 2s

Down

Sleep 1s

Up

Sleep 1s

Wait+Screen /⛊ C.PASS/

Type "s"
Sleep 1s

Type@500ms "jjjjj"

PageUp 1

Sleep 500ms 

Left

Sleep 1s

Down 

Sleep 1s

Down 

Sleep 1s

Left

Sleep 1s

Down 

Sleep 15s

# Teardown

Hide
Type "Remove-Item -Path .\verapack.exe"
Enter

Type "Remove-Item -Path .\versions.yaml"
Enter
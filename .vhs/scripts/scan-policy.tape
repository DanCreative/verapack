# README
# 
# This script produces the .gif recording for the scan policy action.
# Run this command from the root of the directory to run this script: ```vhs .vhs/scripts/scan-policy.tape```


# Setup
Output .vhs/output/scan-policy.gif
Set Width 1600
Set Height 1000
Set FontSize 30
Set Shell "powershell"

Require go

# Build
Hide
Type `go build "-tags=ui" -ldflags='-X "main.Version='0.5.2-dev'"' -o . ./cmd/verapack`
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

Type@2s "gt"

Sleep 2s

Wait+Screen /✓/

Type "s"
Sleep 1s

Type@500ms "jjjjj"

PageUp 1

Sleep 500ms 

Right

Sleep 1s

Down 

Sleep 1s

Left 

Sleep 1s

Up

Sleep 1s

Type "s"

Sleep 15s

# Teardown

Hide
Type "Remove-Item -Path verapack.exe"
Enter

Type "Remove-Item -Path config.yaml"
Enter
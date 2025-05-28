# README
# 
# This script produces the .gif recording for the setup action.
# Run this command from the root of the directory to run this script: ```vhs .vhs/scripts/setup.tape```


# Setup
Output .vhs/output/setup.gif
Set Width 1700
Set Height 850
Set FontSize 25
Set Shell "powershell"

Require go

# Build
Hide
Type `go build "-tags=ui" -o . ./cmd/verapack`
Enter
Type "clear"
Enter
Sleep 1.5s
Show

# Run
Type ".\verapack setup"
Sleep 750ms
Enter

# Setup: Enter credentials
Sleep 500ms
Type@300ms "vera-demo" 
Sleep 500ms
Enter
Sleep 500ms
Type@300ms "vera-demo" 
Sleep 500ms
Enter

# Setup: Remaining steps
Sleep 10s


# Teardown

Hide
Type "Remove-Item -Path verapack.exe"
Enter
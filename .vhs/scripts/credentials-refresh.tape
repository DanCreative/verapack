# README
# 
# This script produces the .gif recording for the credentials refresh action.
# Run this command from the root of the directory to run this script: ```vhs .vhs/scripts/credentials-refresh.tape```


# Setup
Output .vhs/output/credentials-refresh.gif
Set Width 1700
Set Height 850
Set FontSize 25
Set Shell "powershell"

Require go

# Build
Hide
Type `go build "-tags=ui" -o . ./cmd/verapack`
Enter

# Add test credentials folder
Type "Copy-Item -Path .vhs\testdata\.veracode -Destination . -Recurse"
Enter 

Type "clear"
Enter

Sleep 1.5s
Show

# Run
Type ".\verapack credentials refresh"
Sleep 750ms
Enter

Sleep 5s

# Teardown
Hide
Type "Remove-Item -Path verapack.exe"
Enter

Type "Remove-Item -Path .veracode -Recurse"
Enter
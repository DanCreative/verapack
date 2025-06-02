# README
# 
# This script produces the .gif recording for the credentials configure action.
# Run this command from the root of the directory to run this script: ```vhs .vhs/scripts/credentials-configure.tape```


# Setup
Output .vhs/output/credentials-configure.gif
Set Width 1700
Set Height 700
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
Type ".\verapack credentials configure"
Sleep 750ms
Enter

# User Input
Sleep 500ms
Type@300ms "vera-demo" 
Sleep 500ms
Enter
Sleep 500ms
Type@300ms "vera-demo" 
Sleep 500ms
Enter

Sleep 5s

# Teardown

Hide
Type "Remove-Item -Path verapack.exe"
Enter

Type "Remove-Item -Path .veracode -Recurse"
Enter
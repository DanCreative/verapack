# Please note:
#   - This PowerShell script is not signed. You will not be able to run it in a strict environment.
#   - Therefore, this script acts like a template that will be minified into a "one-liner" and added to the README during the release pipeline.
#   - That "one-liner" can then be run from the user's terminal to perform the installation.

$version="${version}";$arch="";$scope="";$path="";$ProgressPreference = 'SilentlyContinue';

# Determine host machine architecture
$osArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
switch ($osArch) {
    arm64 {echo "arm64 is not currently supported";Return;}
    x86 {"32-bit is not currently supported";Return;}
    x64 {$arch = "amd64";}
}

# Determine the installation scope
if (([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator) -eq $true) {$scope = "Machine";$path = "C:\Program Files";} else {$scope = "User";$path = ([Environment]::GetEnvironmentVariable("appdata"));}
$homeFolderPath = $("{0}\verapack" -f $path);

# Download ZIP
Invoke-WebRequest -OutFile $("{0}\{1}" -f ([Environment]::GetEnvironmentVariable("temp")), "verapack.zip") -Uri $('https://github.com/DanCreative/verapack/releases/latest/download/verapack-windows-{0}-{1}.zip' -f $arch,$version) -UseBasicParsing;

# Remove the old app
if (Test-Path $homeFolderPath) {
    [System.IO.Directory]::Delete($homeFolderPath, $true)
}

# Unzip new app in temp dir
Expand-Archive -Path $("{0}\{1}" -f ([Environment]::GetEnvironmentVariable("temp"), "verapack.zip")) -DestinationPath ([Environment]::GetEnvironmentVariable("temp")) -Force;
Rename-Item $("{0}\verapack-windows-{1}-{2}" -f ([Environment]::GetEnvironmentVariable("temp"), $arch, $version)) -NewName "verapack" -Force;

# Move new app to install location
Move-Item -Force -Path $("{0}\verapack" -f ([Environment]::GetEnvironmentVariable("temp"))) -Destination $path;

# Remove install files from temp
[System.IO.File]::Delete($("{0}\{1}" -f ([Environment]::GetEnvironmentVariable("temp")), "verapack.zip"))

# Set the environment variable if it has not been set already
$oldPath = [Environment]::GetEnvironmentVariable('Path', [System.EnvironmentVariableTarget]::$scope);
if ($oldPath.Split(';') -inotcontains $path+'\verapack') {
    [Environment]::SetEnvironmentVariable('Path', $('{0};{1}' -f $oldPath,$path+'\verapack'), [EnvironmentVariableTarget]::$scope);
}
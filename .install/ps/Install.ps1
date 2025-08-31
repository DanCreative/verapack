$ProgressPreference = 'SilentlyContinue';$ErrorActionPreference = 'Stop';$arch="";$scope="";$path="";

# Determine host machine architecture
$OSArch = (Get-CimInstance Win32_operatingsystem).OSArchitecture;switch ($OSArch) {"64-bit"{$arch = "amd64";}default {echo "$OSArch is not currently supported";Return;}}

# Determine the installation scope
if (([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator) -eq $true) {$scope = "Machine";$path = "C:\Program Files";} else {$scope = "User";$path = ([Environment]::GetEnvironmentVariable("appdata"));}
$homeFolderPath = $("{0}\verapack" -f $path);

# Retrieve the download URL for the latest package
$req = Invoke-RestMethod -Uri "https://api.github.com/repos/DanCreative/verapack/releases/latest" -Method Get -ContentType "application/json";

foreach ($asset in $req.assets) {
    if ($asset.name -match "windows-$arch"){
        $downloadUrl = $asset.browser_download_url;
    }
}

# Download ZIP
Invoke-WebRequest -OutFile $("{0}\{1}" -f ([Environment]::GetEnvironmentVariable("temp")), "verapack.zip") -Uri $downloadUrl -UseBasicParsing;

# Remove the old app
if (Test-Path $homeFolderPath) {
    [System.IO.Directory]::Delete($homeFolderPath, $true);
}

New-Item -Path $homeFolderPath -ItemType Directory | Out-Null

# Unzip new app in temp dir
Expand-Archive -Path $("{0}\{1}" -f ([Environment]::GetEnvironmentVariable("temp"), "verapack.zip")) -DestinationPath ([Environment]::GetEnvironmentVariable("temp")) -Force;

# Move new app to install location
Move-Item -Force -Path $("{0}\verapack.exe" -f ([Environment]::GetEnvironmentVariable("temp"))) -Destination "$homeFolderPath\verapack.exe";

# Remove install files from temp
[System.IO.File]::Delete($("{0}\{1}" -f ([Environment]::GetEnvironmentVariable("temp")), "verapack.zip"));

# Set the environment variable if it has not been set already
$oldPath = [Environment]::GetEnvironmentVariable('Path', [System.EnvironmentVariableTarget]::$scope);
if ($oldPath.Split(';') -inotcontains $path+'\verapack') {
    [Environment]::SetEnvironmentVariable('Path', $('{0};{1}' -f $oldPath,$path+'\verapack'), [EnvironmentVariableTarget]::$scope);
}
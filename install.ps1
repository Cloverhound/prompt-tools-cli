# Prompt Tools CLI installer for Windows
# Usage: irm https://raw.githubusercontent.com/Cloverhound/prompt-tools-cli/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "Cloverhound/prompt-tools-cli"
$Binary = "prompt-tools"
$InstallDir = "$env:LOCALAPPDATA\prompt-tools"

# Detect architecture
$Arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Error "32-bit systems are not supported"
    exit 1
}

# Get latest version
Write-Host "Fetching latest release..."
$Release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
$Version = $Release.tag_name -replace "^v", ""
Write-Host "Latest version: v$Version"

# Download
$ZipName = "${Binary}-cli_${Version}_windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/v$Version/$ZipName"

$TmpDir = New-TemporaryFile | ForEach-Object { Remove-Item $_; New-Item -ItemType Directory -Path $_ }

Write-Host "Downloading $Url..."
Invoke-WebRequest -Uri $Url -OutFile "$TmpDir\$ZipName"

# Extract
Expand-Archive -Path "$TmpDir\$ZipName" -DestinationPath $TmpDir -Force

# Install
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Move-Item -Path "$TmpDir\$Binary.exe" -Destination "$InstallDir\$Binary.exe" -Force

# Clean up
Remove-Item -Path $TmpDir -Recurse -Force

# Add to PATH if needed
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    Write-Host ""
    Write-Host "Added $InstallDir to your PATH. Restart your terminal for changes to take effect."
}

Write-Host ""
Write-Host "Installed $Binary v$Version to $InstallDir\$Binary.exe"
Write-Host ""
Write-Host "Get started:"
Write-Host "  prompt-tools setup                       # Interactive setup wizard"
Write-Host "  prompt-tools speak `"Hello`" -o hello.wav  # Generate a prompt"

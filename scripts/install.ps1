# Bootstrap installer (irm|iex). Kept as PowerShell because no Go binary exists yet.
# Devlayer installer for Windows
# Usage: irm https://raw.githubusercontent.com/stefanpenner/devlayer/master/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = if ($env:DEVLAYER_REPO) { $env:DEVLAYER_REPO } else { "stefanpenner/devlayer" }
$Version = if ($env:DEVLAYER_VERSION) { $env:DEVLAYER_VERSION } else { "latest" }
$InstallDir = if ($env:DEVLAYER_DIR) { $env:DEVLAYER_DIR } else { Join-Path $env:LOCALAPPDATA "devlayer" }
$Arch = "x86_64"

$Asset = "devlayer-windows-$Arch.zip"

if ($Version -eq "latest") {
    $BaseUrl = "https://github.com/$Repo/releases/latest/download"
} else {
    $BaseUrl = "https://github.com/$Repo/releases/download/$Version"
}

Write-Host "devlayer: installing $Version for windows/$Arch"
Write-Host "  target: $InstallDir"

# Download
$TmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "devlayer-install-$([System.Guid]::NewGuid())"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    Write-Host "  downloading $Asset..."
    $ZipPath = Join-Path $TmpDir $Asset
    Invoke-WebRequest -Uri "$BaseUrl/$Asset" -OutFile $ZipPath -UseBasicParsing

    # Verify checksum if available
    $ChecksumAsset = "$Asset.sha256"
    $ChecksumPath = Join-Path $TmpDir $ChecksumAsset
    try {
        Invoke-WebRequest -Uri "$BaseUrl/$ChecksumAsset" -OutFile $ChecksumPath -UseBasicParsing
        Write-Host "  verifying checksum..."
        $Expected = (Get-Content $ChecksumPath).Split(" ")[0]
        $Actual = (Get-FileHash -Path $ZipPath -Algorithm SHA256).Hash.ToLower()
        if ($Expected -ne $Actual) {
            throw "Checksum mismatch: expected $Expected, got $Actual"
        }
    } catch [System.Net.WebException] {
        Write-Host "  warning: no checksum available, skipping verification"
    }

    # Extract
    Write-Host "  extracting..."
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Expand-Archive -Path $ZipPath -DestinationPath $InstallDir -Force
} finally {
    Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "devlayer installed to $InstallDir"
Write-Host ""

# Add to PATH if not already there
$BinDir = Join-Path $InstallDir "bin"
$CurrentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($CurrentPath -notlike "*$BinDir*") {
    Write-Host "  adding $BinDir to user PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$BinDir;$CurrentPath", "User")
    $env:Path = "$BinDir;$env:Path"
}

Write-Host "Restart your terminal to pick up PATH changes."

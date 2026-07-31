#Requires -Version 5.1

[CmdletBinding()]
param(
    [string]$Version = $env:AGENTRA_VERSION,
    [string]$InstallDir = $env:AGENTRA_INSTALL_DIR,
    [string]$ReleaseBaseUrl = $env:AGENTRA_RELEASE_BASE_URL,
    [string]$Architecture = $env:AGENTRA_ARCH,
    [switch]$NoPathUpdate
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$Repository = "agentra-ai/agentra"
$DefaultReleaseBaseUrl = "https://github.com/$Repository/releases/download"

function Resolve-AgentraArchitecture {
    param([string]$Requested)

    $RawArchitecture = $Requested
    if ([string]::IsNullOrWhiteSpace($RawArchitecture)) {
        $RawArchitecture = $env:PROCESSOR_ARCHITEW6432
    }
    if ([string]::IsNullOrWhiteSpace($RawArchitecture)) {
        $RawArchitecture = $env:PROCESSOR_ARCHITECTURE
    }
    if ([string]::IsNullOrWhiteSpace($RawArchitecture)) {
        throw "Unable to detect the Windows architecture; pass -Architecture amd64 or arm64"
    }
    switch ($RawArchitecture.ToLowerInvariant()) {
        { $_ -in @("amd64", "x86_64") } { return "amd64" }
        { $_ -in @("arm64", "aarch64") } { return "arm64" }
        default { throw "Unsupported Windows architecture: $RawArchitecture" }
    }
}

function Get-AgentraFile {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [Parameter(Mandatory = $true)][string]$Destination
    )
    if (Test-Path -LiteralPath $Uri -PathType Leaf) {
        Copy-Item -LiteralPath $Uri -Destination $Destination
        return
    }
    Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $Destination
}

function Test-PathEntry {
    param(
        [string]$PathValue,
        [string]$Entry
    )
    if ([string]::IsNullOrWhiteSpace($PathValue)) {
        return $false
    }
    foreach ($Part in $PathValue.Split([IO.Path]::PathSeparator)) {
        if ($Part.TrimEnd([char[]]@('\')) -ieq $Entry.TrimEnd([char[]]@('\'))) {
            return $true
        }
    }
    return $false
}

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    if (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\Agentra"
    } else {
        $InstallDir = Join-Path $HOME ".local\bin"
    }
}
$InstallDir = [IO.Path]::GetFullPath($InstallDir)
$ResolvedArchitecture = Resolve-AgentraArchitecture $Architecture

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Latest = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repository/releases/latest"
    $Version = [string]$Latest.tag_name
}
$Version = $Version.Trim()
if ($Version.StartsWith("v")) {
    $Version = $Version.Substring(1)
}
if ($Version -notmatch '^[0-9A-Za-z._-]+$') {
    throw "Invalid release version: $Version"
}
$Tag = "v$Version"

if ([string]::IsNullOrWhiteSpace($ReleaseBaseUrl)) {
    $ReleaseBaseUrl = $DefaultReleaseBaseUrl
}
$ReleaseBaseUrl = $ReleaseBaseUrl.TrimEnd([char[]]@('/', '\'))
$ReleaseUrl = if (Test-Path -LiteralPath $ReleaseBaseUrl -PathType Container) {
    Join-Path $ReleaseBaseUrl $Tag
} else {
    "$ReleaseBaseUrl/$Tag"
}
$Asset = "agentra_windows_$ResolvedArchitecture.zip"

$Temporary = Join-Path ([IO.Path]::GetTempPath()) ("agentra-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $Temporary | Out-Null
try {
    $Archive = Join-Path $Temporary $Asset
    $Checksums = Join-Path $Temporary "checksums.txt"
    Write-Host "Installing Agentra $Tag for windows/$ResolvedArchitecture..."
    $ArchiveSource = if (Test-Path -LiteralPath $ReleaseUrl -PathType Container) { Join-Path $ReleaseUrl $Asset } else { "$ReleaseUrl/$Asset" }
    $ChecksumsSource = if (Test-Path -LiteralPath $ReleaseUrl -PathType Container) { Join-Path $ReleaseUrl "checksums.txt" } else { "$ReleaseUrl/checksums.txt" }
    Get-AgentraFile $ArchiveSource $Archive
    Get-AgentraFile $ChecksumsSource $Checksums

    $ChecksumEntries = @()
    foreach ($Line in Get-Content $Checksums) {
        if ($Line -match '^([0-9a-fA-F]{64})\s+(.+)$' -and $Matches[2] -ceq $Asset) {
            $ChecksumEntries += $Matches[1]
        }
    }
    if ($ChecksumEntries.Count -ne 1) {
        throw "checksums.txt does not contain exactly one entry for $Asset"
    }
    $ActualChecksum = (Get-FileHash -Algorithm SHA256 -Path $Archive).Hash
    if ($ActualChecksum -ine $ChecksumEntries[0]) {
        throw "SHA-256 checksum mismatch for $Asset"
    }

    $Expanded = Join-Path $Temporary "expanded"
    Expand-Archive -Path $Archive -DestinationPath $Expanded
    $SourceBinary = Join-Path $Expanded "agentra.exe"
    if (-not (Test-Path -Path $SourceBinary -PathType Leaf)) {
        throw "Release archive does not contain agentra.exe"
    }

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $Target = Join-Path $InstallDir "agentra.exe"
    $Staged = Join-Path $InstallDir (".agentra.install." + [Guid]::NewGuid().ToString("N") + ".exe")
    Copy-Item -Path $SourceBinary -Destination $Staged
    try {
        Move-Item -Force -Path $Staged -Destination $Target
    } catch {
        Remove-Item -Force -ErrorAction SilentlyContinue $Staged
        throw "Could not replace $Target. Stop a running Agentra daemon and retry. $($_.Exception.Message)"
    }

    if (-not $NoPathUpdate) {
        $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if (-not (Test-PathEntry $UserPath $InstallDir)) {
            $UpdatedUserPath = if ([string]::IsNullOrWhiteSpace($UserPath)) { $InstallDir } else { "$UserPath$([IO.Path]::PathSeparator)$InstallDir" }
            [Environment]::SetEnvironmentVariable("Path", $UpdatedUserPath, "User")
            Write-Host "Added $InstallDir to the user PATH."
        }
    }
    if (-not (Test-PathEntry $env:Path $InstallDir)) {
        $env:Path = "$InstallDir$([IO.Path]::PathSeparator)$env:Path"
    }

    $VersionOutput = & $Target version 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "Installed binary did not pass 'agentra version'"
    }
    Write-Host "Installed $Target"
    Write-Host "Verified: $($VersionOutput | Select-Object -First 1)"
    Write-Host "Next: agentra setup --deployment self-host"
} finally {
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $Temporary
}

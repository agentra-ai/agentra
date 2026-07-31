#Requires -Version 5.1

[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$RepositoryRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$Version = "9.9.9-test"
$Tag = "v$Version"
$Temporary = Join-Path ([IO.Path]::GetTempPath()) ("agentra-installer-test-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $Temporary | Out-Null

try {
    $ReleaseDir = Join-Path (Join-Path $Temporary "releases") $Tag
    New-Item -ItemType Directory -Path $ReleaseDir | Out-Null
    $FixtureBinary = Join-Path $Temporary "agentra.exe"
    Push-Location (Join-Path $RepositoryRoot "server")
    try {
        & go build -o $FixtureBinary ./cmd/agentra
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to build Windows installer fixture"
        }
    } finally {
        Pop-Location
    }

    $Asset = "agentra_windows_amd64.zip"
    $Archive = Join-Path $ReleaseDir $Asset
    Compress-Archive -Path $FixtureBinary -DestinationPath $Archive
    $Checksum = (Get-FileHash -Algorithm SHA256 -Path $Archive).Hash.ToLowerInvariant()
    $Utf8NoBom = New-Object Text.UTF8Encoding($false)
    [IO.File]::WriteAllText((Join-Path $ReleaseDir "checksums.txt"), "$Checksum  $Asset`n", $Utf8NoBom)

    $InstallDir = Join-Path $Temporary "installed"
    & (Join-Path $RepositoryRoot "scripts\install.ps1") `
        -Version $Version `
        -ReleaseBaseUrl (Join-Path $Temporary "releases") `
        -InstallDir $InstallDir `
        -Architecture amd64 `
        -NoPathUpdate
    if ($LASTEXITCODE -ne 0) {
        throw "Windows installer returned a non-zero exit code"
    }
    $InstalledBinary = Join-Path $InstallDir "agentra.exe"
    if (-not (Test-Path -LiteralPath $InstalledBinary -PathType Leaf)) {
        throw "Windows installer did not create agentra.exe"
    }
    $VersionOutput = & $InstalledBinary version 2>&1
    if ($LASTEXITCODE -ne 0 -or "$VersionOutput" -notmatch "agentra") {
        throw "Installed Windows binary failed its version check"
    }

    $TamperedInstall = Join-Path $Temporary "tampered-install"
    New-Item -ItemType Directory -Path $TamperedInstall | Out-Null
    $Sentinel = Join-Path $TamperedInstall "agentra.exe"
    [IO.File]::WriteAllText($Sentinel, "sentinel", $Utf8NoBom)
    $Stream = [IO.File]::Open($Archive, [IO.FileMode]::Append, [IO.FileAccess]::Write)
    try {
        $Stream.WriteByte(0x42)
    } finally {
        $Stream.Dispose()
    }
    $ChecksumFailed = $false
    try {
        & (Join-Path $RepositoryRoot "scripts\install.ps1") `
            -Version $Version `
            -ReleaseBaseUrl (Join-Path $Temporary "releases") `
            -InstallDir $TamperedInstall `
            -Architecture amd64 `
            -NoPathUpdate
    } catch {
        $ChecksumFailed = $_.Exception.Message -match "checksum mismatch"
    }
    if (-not $ChecksumFailed) {
        throw "Tampered Windows archive was not rejected by its checksum"
    }
    if ([IO.File]::ReadAllText($Sentinel) -ne "sentinel") {
        throw "Failed Windows install modified the existing binary"
    }

    $ArchitectureFailed = $false
    try {
        & (Join-Path $RepositoryRoot "scripts\install.ps1") `
            -Version $Version `
            -ReleaseBaseUrl (Join-Path $Temporary "releases") `
            -InstallDir (Join-Path $Temporary "unsupported") `
            -Architecture riscv64 `
            -NoPathUpdate
    } catch {
        $ArchitectureFailed = $_.Exception.Message -match "Unsupported Windows architecture"
    }
    if (-not $ArchitectureFailed) {
        throw "Unsupported Windows architecture was not rejected"
    }

    Write-Host "Windows installer contract tests passed."
} finally {
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $Temporary
}

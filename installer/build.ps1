# Build a Windows installer for JellyCouch.
#
# Steps:
#   1. Locate Go and Inno Setup (iscc).
#   2. Generate installer\jellycouch.ico from the in-app icon code.
#   3. Build jellycouch.exe if it doesn't already exist (or -Rebuild).
#   4. Run ISCC to produce dist\JellyCouch-Setup-<version>.exe.
#
# Usage (from repo root):
#   powershell -ExecutionPolicy Bypass -File installer\build.ps1
#   powershell -ExecutionPolicy Bypass -File installer\build.ps1 -Version 0.2.0 -Rebuild

[CmdletBinding()]
param(
    [string]$Version = "0.1.0",
    [switch]$Rebuild
)

$ErrorActionPreference = "Stop"

# Repo root = parent of this script's directory.
$repoRoot   = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$installer  = Join-Path $repoRoot "installer"
$distDir    = Join-Path $repoRoot "dist"
$exePath    = Join-Path $repoRoot "jellycouch.exe"
$dllPath    = Join-Path $repoRoot "libmpv.dll"
$icoPath    = Join-Path $installer "jellycouch.ico"
$issPath    = Join-Path $installer "jellycouch.iss"

function Resolve-Go {
    $cmd = Get-Command go.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $candidates = @(
        "C:\Program Files\Go\bin\go.exe",
        "C:\Go\bin\go.exe",
        "$env:LOCALAPPDATA\Programs\Go\bin\go.exe"
    )
    foreach ($p in $candidates) { if (Test-Path $p) { return $p } }
    throw "Go is not installed (or not on PATH). Install from https://go.dev/dl/"
}

function Resolve-ISCC {
    $cmd = Get-Command iscc.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $candidates = @(
        "C:\Program Files (x86)\Inno Setup 6\ISCC.exe",
        "C:\Program Files\Inno Setup 6\ISCC.exe",
        "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
        "C:\Program Files (x86)\Inno Setup 5\ISCC.exe"
    )
    foreach ($p in $candidates) { if (Test-Path $p) { return $p } }

    Write-Host "Inno Setup not found." -ForegroundColor Yellow
    $winget = Get-Command winget -ErrorAction SilentlyContinue
    if ($winget) {
        $reply = Read-Host "Install Inno Setup now via winget? [Y/n]"
        if ($reply -eq "" -or $reply -match "^[Yy]") {
            & winget install --id JRSoftware.InnoSetup --accept-source-agreements --accept-package-agreements
            foreach ($p in $candidates) { if (Test-Path $p) { return $p } }
        }
    }
    throw "Install Inno Setup 6 from https://jrsoftware.org/isdl.php and re-run this script."
}

$go   = Resolve-Go
$iscc = Resolve-ISCC

Write-Host "Go:   $go"
Write-Host "ISCC: $iscc"

# 1. Generate the .ico every run -- cheap and keeps it in sync with the icon code.
Write-Host "Generating $icoPath ..."
Push-Location $repoRoot
try {
    & $go run ./cmd/gen-ico $icoPath
    if ($LASTEXITCODE -ne 0) { throw "gen-ico failed" }
} finally {
    Pop-Location
}

# 2. Build the binary if missing or -Rebuild was supplied.
if ($Rebuild -or -not (Test-Path $exePath)) {
    Write-Host "Building jellycouch.exe ..."
    Push-Location $repoRoot
    try {
        $env:CGO_ENABLED = "1"
        & $go build -ldflags "-s -w -H=windowsgui" -o $exePath ./cmd/jellycouch
        if ($LASTEXITCODE -ne 0) { throw "go build failed" }
    } finally {
        Pop-Location
    }
} else {
    Write-Host "Reusing existing $exePath (pass -Rebuild to force a rebuild)"
}

if (-not (Test-Path $dllPath)) {
    throw "libmpv.dll not found at $dllPath -- the installer needs it bundled."
}

# 3. Run ISCC.
if (-not (Test-Path $distDir)) { New-Item -ItemType Directory -Path $distDir | Out-Null }

Write-Host "Compiling installer (version $Version) ..."
& $iscc "/DMyAppVersion=$Version" $issPath
if ($LASTEXITCODE -ne 0) { throw "ISCC failed" }

$output = Join-Path $distDir "JellyCouch-Setup-$Version.exe"
if (Test-Path $output) {
    Write-Host ""
    Write-Host "Installer built: $output" -ForegroundColor Green
} else {
    throw "Expected $output but it wasn't produced."
}

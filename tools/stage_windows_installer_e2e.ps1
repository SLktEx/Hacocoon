#Requires -Version 7.0
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$artifactRoot = Join-Path $env:RUNNER_TEMP 'windows-e2e-package'
$zip = Join-Path $artifactRoot 'hacocoon-windows-amd64.zip'
$packageRoot = Join-Path $env:RUNNER_TEMP 'hacocoon-windows-amd64'
if (-not (Test-Path -LiteralPath $zip -PathType Leaf)) {
    throw 'Shared Windows E2E package artifact is missing.'
}
Remove-Item -LiteralPath $packageRoot -Recurse -Force -ErrorAction SilentlyContinue
Expand-Archive -LiteralPath $zip -DestinationPath $packageRoot

@(
    'install-windows.bat',
    'install-windows.ps1',
    'install.sh',
    'incus-boot-guard.py',
    'haco_linux_amd64.tar.gz',
    'checksums.txt',
    'VERSION'
) | ForEach-Object {
    if (-not (Test-Path -LiteralPath (Join-Path $packageRoot $_) -PathType Leaf)) {
        throw "Downloaded candidate package is missing $_."
    }
}
if (Test-Path -LiteralPath (Join-Path $packageRoot 'haco_linux_arm64.tar.gz') -PathType Leaf) {
    throw 'amd64 package contains arm64 payload.'
}

$cachePath = Join-Path $env:RUNNER_TEMP 'hacocoon-wsl-cache\ubuntu.wsl'
if (Test-Path -LiteralPath $cachePath -PathType Leaf) {
    Copy-Item -LiteralPath $cachePath -Destination (Join-Path $packageRoot 'ubuntu.wsl') -Force
    Write-Host 'Using trusted main-branch Ubuntu WSL image cache.'
} else {
    Write-Host 'Trusted Ubuntu WSL image cache was not available; installer will perform the verified download.'
}

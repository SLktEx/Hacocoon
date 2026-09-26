#Requires -Version 7.0
param([switch]$FreshOnly)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$packageRoot = Join-Path $env:RUNNER_TEMP 'hacocoon-windows-amd64'
$driver = Join-Path $env:GITHUB_WORKSPACE 'test\e2e\windows\install.py'
$mode = if ($FreshOnly) { 'fresh' } else { 'lifecycle' }
$logPath = Join-Path $env:RUNNER_TEMP ("windows-installer-$mode.log")
$arguments = @($driver, '--use-cached-wsl-image')
if ($FreshOnly) { $arguments += '--fresh-only' }
$driverExit = 1

Push-Location $packageRoot
try {
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        Write-Host "=== Exact user-path $mode live log ==="
        & python @arguments 2>&1 | Tee-Object -FilePath $logPath
        $driverExit = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousPreference
    }
} finally {
    Pop-Location
}

if ($driverExit -ne 0) {
    Write-Host "=== Exact user-path $mode failure tail ==="
    Get-Content -LiteralPath $logPath -Tail 160
    throw "Exact Windows installer $mode path failed with exit $driverExit."
}
Write-Host "=== Exact user-path $mode success tail ==="
Get-Content -LiteralPath $logPath -Tail 60

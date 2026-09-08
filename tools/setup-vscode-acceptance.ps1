#Requires -Version 7.0
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ($env:GITHUB_ACTIONS -ne 'true' -or -not $env:RUNNER_TEMP -or -not $env:GITHUB_PATH) {
    throw 'Portable editor acceptance setup is restricted to the disposable GHA runner.'
}
$client = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'vscode-acceptance/client.json') | ConvertFrom-Json
if ($client.commit -notmatch '^[a-f0-9]{40}$' -or $client.sha256 -notmatch '^[a-f0-9]{64}$' -or
    -not $client.url.StartsWith("https://vscode.download.prss.microsoft.com/dbazure/download/stable/$($client.commit)/")) {
    throw 'Invalid pinned VS Code artifact identity.'
}
$root = Join-Path $env:RUNNER_TEMP 'hacocoon-vscode'
if (Test-Path -LiteralPath $root) { throw 'Editor fixture directory already exists.' }
[void][IO.Directory]::CreateDirectory($root)
$archive = Join-Path $root 'client.zip'
Invoke-WebRequest -Uri $client.url -OutFile $archive
if ((Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash.ToLowerInvariant() -ne $client.sha256) {
    throw 'VS Code archive checksum mismatch.'
}
$application = Join-Path $root 'client'
Expand-Archive -LiteralPath $archive -DestinationPath $application
# Portable mode owns all editor settings/extensions inside this fresh fixture.
$user = Join-Path $application 'data/user-data/User'
[void][IO.Directory]::CreateDirectory($user)
$settings = @{
    'update.mode' = 'none'
    'workbench.startupEditor' = 'none'
    'security.workspace.trust.enabled' = $false
    'remote.SSH.localServerDownload' = 'always'
    'telemetry.telemetryLevel' = 'off'
}
[IO.File]::WriteAllText((Join-Path $user 'settings.json'), ($settings | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
$bin = Join-Path $application 'bin'
$bin | Out-File -LiteralPath $env:GITHUB_PATH -Encoding utf8 -Append
Write-Host "Verified portable VS Code $($client.version), commit $($client.commit), SHA-256 $($client.sha256)"

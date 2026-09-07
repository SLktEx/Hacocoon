#Requires -Version 7.0
param([Parameter(Mandatory)][string]$EnvironmentName, [string]$Distro = 'Hacocoon')
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ($env:GITHUB_ACTIONS -ne 'true' -or -not $env:RUNNER_TEMP) {
    throw 'Real editor acceptance is restricted to the disposable GHA profile.'
}
if ($EnvironmentName -notmatch '^win-ssh-[a-f0-9]{16}$' -or $Distro -ne 'Hacocoon') {
    throw 'Unexpected editor acceptance target.'
}
$application = Join-Path $env:RUNNER_TEMP 'hacocoon-vscode/client'
$code = Join-Path $application 'bin/code.cmd'
$codeExecutable = [IO.Path]::GetFullPath((Join-Path $application 'Code.exe'))
if (-not (Test-Path -LiteralPath $code -PathType Leaf) -or
    -not (Test-Path -LiteralPath (Join-Path $application 'data/user-data/User/settings.json') -PathType Leaf)) {
    throw 'Pinned portable editor is not prepared.'
}
$work = Join-Path $env:RUNNER_TEMP ('haco-vscode-proof-' + [guid]::NewGuid().ToString('N'))
[void][IO.Directory]::CreateDirectory($work)
$vsix = Join-Path $work 'observer.vsix'
$resultFile = Join-Path $work 'result.json'
$manifestFile = Join-Path $work 'fixture.json'
& python (Join-Path $PSScriptRoot 'package_vscode_acceptance.py') --environment $EnvironmentName --output $vsix --result $resultFile --manifest $manifestFile
if ($LASTEXITCODE -ne 0) { throw 'Failed to package editor observer.' }
$fixture = Get-Content -Raw -LiteralPath $manifestFile | ConvertFrom-Json
try {
    & $code --install-extension $vsix
    if ($LASTEXITCODE -ne 0) { throw 'Failed to install disposable UI observer.' }
    # No replacement SSH transport, provider setup or product test override.
    & wsl.exe -d $Distro -u root --exec incus exec haco-host --project hacocoon -- /usr/local/bin/haco open $EnvironmentName
    if ($LASTEXITCODE -ne 0) { throw 'Ordinary haco open failed.' }
    $deadline = [DateTime]::UtcNow.AddMinutes(10)
    while (-not (Test-Path -LiteralPath $resultFile -PathType Leaf) -and [DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Seconds 2
    }
    if (-not (Test-Path -LiteralPath $resultFile -PathType Leaf)) {
        throw 'VS Code did not complete remote editor/terminal acceptance within 10 minutes.'
    }
    $result = Get-Content -Raw -LiteralPath $resultFile | ConvertFrom-Json
    $expected = @('workspace-marker','editor-file-read-write','remote-terminal-exec','owned-probes-removed')
    if ($result.status -ne 'passed' -or $result.stage -ne 'complete' -or
        $result.authority -ne $fixture.authority -or $result.nonce -ne $fixture.nonce -or
        ($result.checks -join ',') -ne ($expected -join ',')) {
        throw "Editor acceptance failed at stage '$($result.stage)'."
    }
    Write-Host "VS Code $($result.vscode): actual Remote-SSH editor file read/write, terminal execution and probe cleanup passed."
    Write-Host 'VS CODE REMOTE ENVIRONMENT: PASS'
} finally {
    # This exact executable belongs to the freshly created portable fixture;
    # never terminate another VS Code installation or an operator's editor.
    foreach ($process in @(Get-Process -Name Code -ErrorAction SilentlyContinue)) {
        if ($process.Path -eq $codeExecutable) {
            try { $process.Kill() } catch { if (-not $process.HasExited) { throw } }
        }
    }
}

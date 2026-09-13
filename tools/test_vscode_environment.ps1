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
# Supply the same saved platform choice a user selects on first Remote-SSH use.
# This belongs only to the disposable editor profile, not Hacocoon policy.
$settingsPath = Join-Path $application 'data/user-data/User/settings.json'
$settings = Get-Content -Raw -LiteralPath $settingsPath | ConvertFrom-Json -AsHashtable
$alias = 'haco-' + $EnvironmentName
$settings['remote.SSH.remotePlatform'] = @{ $alias = 'linux' }
[IO.File]::WriteAllText($settingsPath, ($settings | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
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
        Write-Host 'VS CODE ACCEPTANCE: FAIL phase=editor-timeout'
        throw 'VS Code did not complete remote editor/terminal acceptance within 10 minutes.'
    }
    $result = Get-Content -Raw -LiteralPath $resultFile | ConvertFrom-Json
    $expected = @('workspace-marker','editor-file-read-write','remote-terminal-exec','local-approval-stale-refusal','owned-probes-removed')
    if ($result.status -ne 'passed' -or $result.stage -ne 'complete' -or
        $result.authority -ne $fixture.authority -or $result.nonce -ne $fixture.nonce -or
        ($result.checks -join ',') -ne ($expected -join ',')) {
        $safeStage = 'invalid-receipt'
        if ($result.stage -cin @('remote-kind','remote-filesystem','remote-terminal','local-approval-review','cleanup','complete')) { $safeStage = $result.stage }
        Write-Host "VS CODE ACCEPTANCE: FAIL phase=$safeStage"
        if ($result.PSObject.Properties.Name -contains 'reviewDiagnostics') {
            $diagnostic = $result.reviewDiagnostics
            foreach ($key in @('localUI','desktop','trusted','panelCreated','readyObserved','refusalObserved','cleanup')) {
                if ($diagnostic.PSObject.Properties.Name -contains $key -and $diagnostic.$key -is [bool]) { Write-Host ('VS CODE REVIEW: ' + $key + '=' + $diagnostic.$key) }
            }
            if ($diagnostic.PSObject.Properties.Name -contains 'step' -and $diagnostic.step -cin @('api','create','open','wait')) { Write-Host ('VS CODE REVIEW STEP: ' + $diagnostic.step) }
        }
        throw "Editor acceptance failed at stage '$safeStage'."
    }
    Write-Host "VS Code $($result.vscode): actual Remote-SSH editor file read/write, terminal execution and probe cleanup passed."
    Write-Host 'VS CODE LOCAL APPROVAL WEBVIEW / REAL RENDERER HANDSHAKE / INSTALLED CONTROLLER STALE REFUSAL: PASS'
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

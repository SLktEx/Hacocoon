#Requires -Version 7.0
param([Parameter(Mandatory)][string]$EnvironmentName, [string]$Distro = 'Hacocoon')
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
# Called only for processes selected by the exact disposable editor path.
function Stop-OwnedEditorProcess([Diagnostics.Process]$Process) {
    if (-not $Process.HasExited) { $Process.Kill($true) }
    if (-not $Process.WaitForExit(10000)) { throw 'Owned editor did not exit' }
}

# Only fixed diagnostic values can cross the editor receipt boundary.
function Select-VSCodeDiagnosticValue($Value, [string[]]$Allowed) {
    if ($Value -is [string] -and $Value -cin $Allowed) { return $Value }
    return 'unobserved'
}

function Get-VSCodeDiagnosticDuration($Value) {
    if ($Value -isnot [int] -and $Value -isnot [long] -and $Value -isnot [double]) { return $null }
    if (-not [double]::IsFinite($Value) -or $Value -lt 0 -or $Value -gt 9007199254740991 -or [math]::Floor($Value) -ne $Value) { return $null }
    return $Value
}

function Write-VSCodeFailureDiagnostic($Result) {
    try {
        if ($Result.PSObject.Properties.Name -notcontains 'filesystemFailure' -or $null -eq $Result.filesystemFailure) { return }
        $detail = $Result.filesystemFailure
        $checks = @('workspace-marker','editor-file-read-write','remote-terminal-exec','local-approval-stale-refusal','owned-probes-removed')
        $record = [ordered]@{
            component = 'ci'; operation = 'vscode_acceptance_filesystem'
            suboperation = Select-VSCodeDiagnosticValue $detail.suboperation @('marker-read','marker-validate','editor-write','editor-open','editor-validate','editor-show')
            completed_checks = @($Result.checks | Where-Object { $_ -is [string] -and $_ -cin $checks } | Select-Object -Unique)
            error_category = Select-VSCodeDiagnosticValue $detail.error_category @('type','not-found','exists','not-directory','is-directory','no-permissions','unavailable','other','unobserved')
            duration_ms = Get-VSCodeDiagnosticDuration $detail.duration_ms
        }
        [Console]::Out.WriteLine(($record | ConvertTo-Json -Depth 3 -Compress))
        [Console]::Out.Flush()
    } catch {
        # Observation failure must not replace the original refusal or skip cleanup.
        return
    }
}

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
    # The previous successful editor setup includes Microsoft's standard client.
    # haco open used to install it; the cold test now starts with a saved-folder URI.
    & $code --install-extension ms-vscode-remote.remote-ssh
    if ($LASTEXITCODE -ne 0) { throw 'Failed to install standard Remote-SSH client.' }
    $extensions = & $code --list-extensions
    if ($LASTEXITCODE -ne 0 -or $extensions -notcontains 'ms-vscode-remote.remote-ssh') { throw 'Standard Remote-SSH client is missing.' }
    & $code --install-extension $vsix
    if ($LASTEXITCODE -ne 0) { throw 'Failed to install disposable UI observer.' }
    # SSH setup has already persisted this target. Simulate shutdown, then make
    # standard Remote-SSH's saved-folder route the first contact with Hacocoon.
    & wsl.exe -d $Distro -u root --exec incus exec haco-host --project hacocoon -- /usr/local/bin/haco env stop $EnvironmentName
    if ($LASTEXITCODE -ne 0) { throw 'Could not stop fixture Environment.' }
    & wsl.exe -d $Distro -u root --exec systemctl stop haco-controller.service
    if ($LASTEXITCODE -ne 0) { throw 'Could not stop fixture controller.' }
    & wsl.exe --terminate $Distro
    if ($LASTEXITCODE -ne 0) { throw 'Could not terminate fixture distribution.' }
    # No haco command, WSL shell, product extension, or repair precedes reconnect.
    & $code --folder-uri "vscode-remote://ssh-remote+$alias/workspace"
    if ($LASTEXITCODE -ne 0) { throw 'Standard saved remote folder launch failed.' }
    $deadline = [DateTime]::UtcNow.AddMinutes(10)
    while (-not (Test-Path -LiteralPath $resultFile -PathType Leaf) -and [DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Seconds 2
    }
    if (-not (Test-Path -LiteralPath $resultFile -PathType Leaf)) {
        Write-Host 'VS CODE ACCEPTANCE: FAIL phase=editor-timeout'
        # Read local fixed observer signals only after failure; never contact WSL
        # or print raw editor logs, paths, process arguments or remote errors.
        & python (Join-Path $PSScriptRoot 'vscode_acceptance_diagnostics.py') --manifest $manifestFile
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
        Write-VSCodeFailureDiagnostic $result
        & python (Join-Path $PSScriptRoot 'vscode_acceptance_diagnostics.py') --manifest $manifestFile
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
            Stop-OwnedEditorProcess $process
        }
    }
}

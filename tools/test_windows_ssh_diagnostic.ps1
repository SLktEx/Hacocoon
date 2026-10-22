$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'windows_ssh_diagnostic.ps1')
# The native SSH fixture has a path Workspace: its four prerequisites are
# required, while a Git route is absent by contract.
foreach ($case in @(
    @{Name='runtime'; Expected=@($true,$false,$false,$false,$false,$false)},
    @{Name='workspace'; Expected=@($true,$false,$false,$false,$false,$false)},
    @{Name='dns_service'; Expected=@($true,$false,$false,$false,$false,$false)},
    @{Name='ssh_service'; Expected=@($true,$false,$false,$false,$false,$false)},
    @{Name='git_broker'; Expected=@($true,$true,$false,$false,$false,$false)}
)) {
    $statuses = @('ok','not_applicable','failed','skipped','unknown','INVALID')
    for ($i=0; $i -lt $statuses.Count; $i++) {
        if ((Test-EnvironmentDoctorCheck ([pscustomobject]@{name=$case.Name; status=$statuses[$i]})) -ne $case.Expected[$i]) { throw 'Doctor prerequisite decision changed' }
    }
}
$sample = "debug1: Connection established.`nAuthenticated to SECRET-PEER using `"publickey`".`ndebug1: Entering interactive session.`ndebug1: Exit status 0`nidentity file SECRET-KEY"
# Proxy transport observations never establish host-key refusal or expose payloads.
foreach ($case in @(
    @{Text='debug1: Executing proxy command: exec SECRET-KEY SECRET-PEER'; Want='proxy_started'},
    @{Text='debug1: Local version string SSH-2.0-OpenSSH_for_Windows_SECRET'; Want='local_version_sent'},
    @{Text='debug1: Remote protocol version 2.0, remote software version SECRET'; Want='remote_version_received'},
    @{Text='Connection timed out during banner exchange'; Want='banner_timeout'},
    @{Text='kex_exchange_identification: banner line contains invalid characters'; Want='banner_invalid'},
    @{Text='kex_exchange_identification: Connection closed by remote host'; Want='banner_closed'},
    @{Text='haco: controller readiness failed'; Want='controller_unready'}
)) {
    if ((Get-SSHProgressEvidence '' $case.Text) -cne $case.Want) { throw 'Proxy transport classification missing or unsafe' }
    if ((Get-SSHHostKeyCheckOutcome 255 '' $case.Text) -cne 'refusal_unconfirmed') { throw 'Transport observation weakened host-key refusal' }
}
foreach ($hostile in @('prefix Connection timed out during banner exchange', 'Connection timed out during banner exchange SECRET', 'haco: controller readiness failed SECRET', "debug1: Executing proxy command: `nSECRET")) {
    if ((Get-SSHProgressEvidence '' $hostile) -cne 'no-recognized-progress') { throw 'Malformed transport observation accepted' }
}
$expected = 'connected,authenticated,session,exit_received,windows-workspace-ok'
if ((Get-SSHProgressEvidence "windows-workspace-ok`nSECRET-CONTENT" $sample) -cne $expected) { throw 'Progress selection failed' }
if ((Get-SSHProgressEvidence '' "[failed] operation=stream stage=target reason=incompatible_state`nSECRET") -cne 'stream_incompatible_state') { throw 'Stream reason selection failed' }
foreach ($hostile in @('[failed] operation=stream stage=target reason=SECRET', '[failed] operation=stream stage=target reason=denied SECRET', 'prefix [failed] operation=stream stage=target reason=denied')) {
    if ((Get-SSHProgressEvidence '' $hostile) -cne 'no-recognized-progress') { throw 'Untrusted stream reason escaped allowlist' }
}
foreach ($hostile in @('SECRET', "windows-workspace-ok SECRET", "prefix windows-workspace-ok", "WINDOWS-WORKSPACE-OK")) {
    if ((Get-SSHProgressEvidence $hostile $hostile) -cne 'no-recognized-progress') { throw 'Untrusted progress escaped allowlist' }
}
# The recorded Windows failure contains NUL-interleaved UTF-16 output. Keep
# only the known error classification; unrelated output must remain private.
$wslError = "Catastrophic failure`nError code: Wsl/Service/E_UNEXPECTED`nSECRET"
foreach ($encoded in @($wslError, [Text.Encoding]::UTF8.GetString([Text.Encoding]::Unicode.GetBytes($wslError)))) {
    if ((Get-SSHProgressEvidence $encoded '') -cne 'wsl_service_unexpected') { throw 'WSL service failure classification lost' }
}
foreach ($hostile in @('Error code: Wsl/Service/SECRET', 'prefix Error code: Wsl/Service/E_UNEXPECTED', 'Error code: Wsl/Service/E_UNEXPECTED SECRET')) {
    if ((Get-SSHProgressEvidence $hostile '') -cne 'no-recognized-progress') { throw 'WSL diagnostic escaped allowlist' }
}
foreach ($case in @(
    @{Exit=255; Out=''; Err='Host key verification failed.'; Want='refused'},
    @{Exit=255; Out=''; Err='WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!'; Want='refused'},
    @{Exit=0; Out=''; Err='Host key verification failed.'; Want='exit_zero'},
    @{Exit=255; Out='MUST-NOT-EXECUTE'; Err='Host key verification failed.'; Want='command_marker'},
    @{Exit=255; Out=''; Err=$wslError; Want='refusal_unconfirmed'},
    @{Exit=255; Out=''; Err='Connection timed out: SECRET-PEER'; Want='refusal_unconfirmed'},
    @{Exit=255; Out=''; Err='[failed] operation=stream stage=target reason=denied'; Want='refusal_unconfirmed'}
)) {
    if ((Get-SSHHostKeyCheckOutcome $case.Exit $case.Out $case.Err) -cne $case.Want) { throw 'Host-key refusal decision or private diagnostics changed' }
}
# Extract only the process helper; never run the real WSL/SSH fixture here.
$tokens = $null; $errors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'test_windows_environment_ssh.ps1'), [ref]$tokens, [ref]$errors)
if ($errors.Count -ne 0) { throw 'SSH fixture has syntax errors' }
$function = $ast.Find({param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Invoke-Captured'}, $true)
. ([scriptblock]::Create($function.Extent.Text))
$pwsh = (Get-Command pwsh -ErrorAction Stop).Source
$probe = '[Console]::Out.WriteLine("windows-ssh-command-complete"); [Console]::Error.WriteLine("SECRET-KEY"); Start-Sleep -Seconds 30'
$failure = $null
try { [void](Invoke-Captured $pwsh @('-NoProfile','-NonInteractive','-Command',$probe) -SSHProgress -TimeoutMilliseconds 3000) } catch { $failure = $_.Exception.Message }
if (-not $failure -or $failure -notmatch 'timed out' -or $failure -notmatch 'windows-ssh-command-complete' -or $failure -match 'SECRET') { throw 'Timeout lost safe evidence or exposed child output' }
$result = Invoke-Captured $pwsh @('-NoProfile','-NonInteractive','-Command','[Console]::Error.WriteLine("SECRET"); exit 17') -SSHProgress
if ($result.ExitCode -ne 17 -or $result.Stderr -cne 'no-recognized-progress') { throw 'Nonzero child status or redaction lost' }
Write-Host 'SSH progress allowlist / real child timeout / nonzero status: PASS'

# Completed captures retain raw sizes before optional progress redaction.
$result = Invoke-Captured $pwsh @('-NoProfile','-NonInteractive','-Command','[Console]::Out.Write("OK"); [Console]::Error.Write("PRIVATE"); exit 0')
if ($result.ExitCode -ne 0 -or $result.Stdout -cne 'OK' -or $result.Stderr -cne 'PRIVATE' -or
    -not $result.Capture.capture_complete -or $result.Capture.stdout_chars -ne 2 -or $result.Capture.stderr_chars -ne 7 -or $result.Capture.capture_duration_ms -lt 0) { throw 'Successful completed capture metadata differs' }
$result = Invoke-Captured $pwsh @('-NoProfile','-NonInteractive','-Command','[Console]::Error.Write("PRIVATE"); exit 17') -SSHProgress
if ($result.ExitCode -ne 17 -or $result.Capture.exit_code -ne 17 -or $result.Capture.stderr_chars -ne 7 -or
    $result.Capture.stdout_chars -ne 0 -or -not $result.Capture.capture_complete -or $result.Stderr -cne 'no-recognized-progress') { throw 'Failed completed capture metadata differs' }

# The extracted projection preserves every known state and rejects unknown data.
foreach ($state in @('not_attempted','unconfirmed','completed','failed','blocked')) {
    $inputOutcomes = @{policy=$state;disconnect=$state;environment=$state;refusal=$state;workspace=$state;base=$state;local_files=$state;SECRET='SECRET'}
    $projected = Get-SSHCleanupOutcomes $inputOutcomes
    if (($projected.Keys -join ',') -cne 'policy,disconnect,environment,refusal,workspace,base,local_files' -or
        @($projected.Values | Where-Object { $_ -cne $state }).Count) { throw 'Cleanup projection changed known fields or states' }
}
$projected = Get-SSHCleanupOutcomes @{policy='SECRET';disconnect=7;environment=$null;refusal='FAILED';workspace=@('completed');base='completed SECRET'}
if (@($projected.Values | Where-Object { $_ -cne 'unknown' }).Count) { throw 'Cleanup projection accepted malformed state' }

$clock = [Diagnostics.Stopwatch]::StartNew()
$originalOutput = [Console]::Out
$receiptOutput = [IO.StringWriter]::new()
try {
    [Console]::SetOut($receiptOutput)
    Write-SSHAcceptancePhase 'changed-host-key' 'completed' $clock $result.Capture 'proxy_started,SECRET,stream_denied'
    Write-SSHAcceptancePhase 'SECRET-PHASE' 'SECRET-STATE' $clock ([pscustomobject]@{stdout_chars='SECRET';stderr_chars=-1;capture_duration_ms='SECRET';exit_code='SECRET';capture_complete='SECRET'}) 'SECRET'
    Write-SSHAcceptanceSummary $true 'SECRET-PHASE' @('vscode','SECRET') $true $false @{policy='SECRET';disconnect='failed';SECRET='SECRET'} $clock
    Write-SSHAcceptanceSummary $false 'native-probes' @() $false $true @{environment='not_attempted'} $clock
} finally { [Console]::SetOut($originalOutput) }
$receiptText = $receiptOutput.ToString()
if ($receiptText.Contains('SECRET') -or $receiptText.Contains('PRIVATE')) { throw 'Receipt leaked arbitrary input' }
$receipts = @($receiptText.Trim() -split '\r?\n' | ForEach-Object { $_ | ConvertFrom-Json })
if ($receipts.Count -ne 4 -or ($receipts[0].ssh_progress -join ',') -cne 'proxy_started,stream_denied' -or
    $receipts[0].stderr_chars -ne 7 -or $receipts[1].phase -cne 'unknown' -or $receipts[1].capture_complete -or
    $receipts[2].primary_phase -cne 'unknown' -or ($receipts[2].desktop_failures -join ',') -cne 'vscode' -or
    -not $receipts[2].desktop_failure_unknown -or $receipts[2].cleanup.policy -cne 'unknown' -or $receipts[3].environment_absence -cne 'not_attempted') { throw 'Receipt schema or allowlist differs' }

Add-Type -TypeDefinition @'
using System;
using System.IO;
public sealed class BrokenSSHDiagnosticWriter : StringWriter {
    public override void WriteLine(string value) { throw new IOException("PRIVATE SINK ERROR"); }
    public override void Flush() { throw new IOException("PRIVATE FLUSH ERROR"); }
}
'@

# Execute the actual outer catch/finally and terminal assertions with all remote
# commands mocked. No WSL/SSH command, key creation or real policy action runs.
$mainTry = @($ast.EndBlock.Statements | Where-Object { $_ -is [Management.Automation.Language.TryStatementAst] })
if ($mainTry.Count -ne 1 -or $mainTry[0].CatchClauses.Count -ne 1) { throw 'Fixture failure boundary changed' }
$terminalAssertions = @($ast.EndBlock.Statements | Where-Object {
    $_ -is [Management.Automation.Language.IfStatementAst] -and $_.Extent.StartOffset -gt $mainTry[0].Extent.EndOffset
} | ForEach-Object { $_.Extent.Text }) -join "`n"
$cleanupDriver = [scriptblock]::Create('try { if ($FailPrimary) { throw "PRIVATE PRIMARY" } } ' +
    $mainTry[0].CatchClauses[0].Extent.Text + ' finally ' + $mainTry[0].Finally.Extent.Text + "`n" + $terminalAssertions)

function Invoke-SSHCleanupFixture([bool]$FailPrimary, [bool]$FailCleanup, [bool]$BrokenSink, [bool]$FailLocal, [bool]$DesktopFailed) {
    $calls = [Collections.Generic.List[string]]::new()
    $SSHProbeClock = [Diagnostics.Stopwatch]::StartNew()
    $PrimaryFailed = $false; $PrimaryPhase = 'changed-host-key'; $CleanupFailed = $false
    $CleanupOutcomes = [ordered]@{policy='not_attempted';disconnect='not_attempted';environment='not_attempted';refusal='not_attempted';workspace='not_attempted';base='not_attempted';local_files='not_attempted'}
    $DesktopFailures = [Collections.Generic.List[string]]::new()
    if ($DesktopFailed) { $DesktopFailures.Add('vscode'); $DesktopFailures.Add('preview') }
    $EnvironmentAttempted = $true; $WorkspaceCreated = $true; $ConnectionId = 'fixture-connection'
    $EnvironmentName = 'fixture'; $Workspace = 'unused'; $ReclamationManifest = $null; $BuiltBaseName = 'unused'; $BuiltBaseFingerprint = 'unused'; $Distro = 'unused'
    $Work = Join-Path ([IO.Path]::GetTempPath()) ('ssh-diagnostics-' + [guid]::NewGuid().ToString('N'))
    [void][IO.Directory]::CreateDirectory($Work)
    $KnownHosts = Join-Path $Work 'known'; $ConfigPath = Join-Path $Work 'config'; $PublicKey = Join-Path $Work 'public'; $PrivateKey = Join-Path $Work 'private'; $BaseDefinition = Join-Path $Work 'definition'
    foreach ($path in @($KnownHosts,$ConfigPath,$PublicKey,$PrivateKey,$BaseDefinition)) { [IO.File]::WriteAllText($path, 'non-credential test marker') }
    if ($FailLocal) { [IO.File]::WriteAllText((Join-Path $Work 'unexpected'), 'owned test marker') }
    function New-CleanupFailure {
        $failure = [Exception]::new('PRIVATE CLEANUP')
        $failure.Data['acceptance_capture'] = [pscustomobject]@{capture_complete=$true; stdout_chars=9; stderr_chars=0; capture_duration_ms=1; exit_code=-1}
        $failure.Data['acceptance_progress'] = 'wsl_service_unexpected'
        return $failure
    }
    function Update-SSHTestPolicy([string]$Action) { $calls.Add('policy'); if ($FailCleanup) { throw (New-CleanupFailure) } }
    function Invoke-HacoHost([string[]]$Arguments, [string]$Description) {
        $calls.Add($Description)
        if ($FailCleanup) { throw (New-CleanupFailure) }
        return [pscustomobject]@{ExitCode=0;Stdout='';Stderr=''}
    }
    function Invoke-Wsl([string[]]$Arguments, [string]$Description) { $calls.Add($Description); return [pscustomobject]@{ExitCode=0;Stdout='';Stderr=''} }
    function Invoke-Captured([string]$FileName, [string[]]$Arguments) {
        $calls.Add('post-deletion-refusal')
        return [pscustomobject]@{ExitCode=255;Stdout='';Stderr='';Capture=[pscustomobject]@{capture_complete=$true;stdout_chars=0;stderr_chars=0;capture_duration_ms=1;exit_code=255}}
    }
    $NativeSSH = 'never-executed'
    $beforeOutput = [Console]::Out
    $output = if ($BrokenSink) { [BrokenSSHDiagnosticWriter]::new() } else { [IO.StringWriter]::new() }
    $failureMessage = $null
    try {
        [Console]::SetOut($output)
        try { . $cleanupDriver } catch { $failureMessage = $_.Exception.Message }
    } finally {
        [Console]::SetOut($beforeOutput)
        if (Test-Path -LiteralPath $Work) { Remove-Item -LiteralPath $Work -Recurse -Force }
    }
    return [pscustomobject]@{Message=$failureMessage;Calls=$calls.ToArray();Receipts=$output.ToString();PrimaryFailed=$PrimaryFailed;CleanupFailed=$CleanupFailed;EnvironmentGone=$EnvironmentGone;Outcomes=$CleanupOutcomes}
}

foreach ($broken in @($false,$true)) {
    $case = Invoke-SSHCleanupFixture $true $true $broken $false $true
    if ($case.Message -cne 'PRIVATE PRIMARY' -or -not $case.PrimaryFailed -or -not $case.CleanupFailed -or $case.EnvironmentGone -or
        ($case.Calls -join '|') -cne 'policy|Disconnect acceptance SSH transport|Delete acceptance Environment' -or $case.Outcomes.local_files -cne 'completed') { throw 'Primary failure or exact cleanup ordering changed' }
    if (-not $broken) {
        if ($case.Receipts -match 'PRIVATE|fixture-connection|unused') { throw 'Cleanup diagnostics leaked input' }
        $summary = @($case.Receipts.Trim() -split '\r?\n' | ForEach-Object { $_ | ConvertFrom-Json })[-1]
        if ($summary.phase -cne 'summary' -or $summary.state -cne 'failed' -or -not $summary.primary_failed -or
            ($summary.desktop_failures -join ',') -cne 'vscode,preview' -or $summary.environment_absence -cne 'unconfirmed' -or
            $summary.cleanup.policy -cne 'failed' -or $summary.cleanup.disconnect -cne 'failed' -or $summary.cleanup.environment -cne 'failed' -or
            $summary.cleanup.refusal -cne 'blocked' -or $summary.cleanup.workspace -cne 'blocked' -or $summary.cleanup.base -cne 'blocked') { throw 'Combined failure summary lost a failure or ownership uncertainty' }
    }
    $case = Invoke-SSHCleanupFixture $false $false $broken $false $false
    if ($case.Message -or $case.PrimaryFailed -or $case.CleanupFailed -or -not $case.EnvironmentGone -or $case.Calls.Count -ne 7) { throw 'Successful cleanup changed with diagnostics' }
    if (-not $broken) {
        $summary = @($case.Receipts.Trim() -split '\r?\n' | ForEach-Object { $_ | ConvertFrom-Json })[-1]
        if ($summary.environment_absence -cne 'delete_succeeded') { throw 'Successful delete receipt differs' }
    }
    $case = Invoke-SSHCleanupFixture $true $false $broken $true $false
    if ($case.Message -cne 'PRIVATE PRIMARY' -or -not $case.CleanupFailed -or $case.Outcomes.local_files -cne 'unconfirmed') { throw 'Local cleanup or broken sink masked primary exception' }
}
$case = Invoke-SSHCleanupFixture $false $true $false $false $false
if ($case.Message -cne 'Windows SSH acceptance cleanup failed; inspect retained test resources') { throw 'Cleanup failure acceptance assertion changed' }
$case = Invoke-SSHCleanupFixture $false $false $false $false $true
if ($case.Message -cne 'Desktop acceptance failed: vscode, preview; independent probes and cleanup were attempted.') { throw 'Desktop failure acceptance assertion changed' }
$case = Invoke-SSHCleanupFixture $false $false $false $true $false
if (-not $case.Message -or -not $case.CleanupFailed -or $case.PrimaryFailed) { throw 'Local cleanup exception was swallowed' }
Write-Host 'SSH completed capture / safe receipts / primary and cleanup failures / broken sink: PASS'

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

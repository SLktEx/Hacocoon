$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'windows_ssh_diagnostic.ps1')
$sample = "debug1: Connection established.`nAuthenticated to SECRET-PEER using `"publickey`".`ndebug1: Entering interactive session.`ndebug1: Exit status 0`nidentity file SECRET-KEY"
$expected = 'connected,authenticated,session,exit_received,windows-workspace-ok'
if ((Get-SSHProgressEvidence "windows-workspace-ok`nSECRET-CONTENT" $sample) -cne $expected) { throw 'Progress selection failed' }
foreach ($hostile in @('SECRET', "windows-workspace-ok SECRET", "prefix windows-workspace-ok", "WINDOWS-WORKSPACE-OK")) {
    if ((Get-SSHProgressEvidence $hostile $hostile) -cne 'no-recognized-progress') { throw 'Untrusted progress escaped allowlist' }
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

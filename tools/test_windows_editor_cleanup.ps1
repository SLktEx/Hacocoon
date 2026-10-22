#Requires -Version 7.0
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
# Extract only cleanup; never run the editor/WSL fixture.
$tokens=$null; $errors=$null
$ast=[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'test_vscode_environment.ps1'),[ref]$tokens,[ref]$errors)
if ($errors.Count) { throw 'Editor fixture syntax error' }
$function=$ast.Find({param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Stop-OwnedEditorProcess'},$true)
. ([scriptblock]::Create($function.Extent.Text))
$pwsh=(Get-Command pwsh -ErrorAction Stop).Source
function Start-Probe([string]$Script) {
    $info=[Diagnostics.ProcessStartInfo]::new($pwsh)
    $info.UseShellExecute=$false; $info.CreateNoWindow=$true
    $info.RedirectStandardOutput=$true; $info.RedirectStandardError=$true
    foreach($arg in @('-NoProfile','-NonInteractive','-EncodedCommand',[Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($Script)))) { $info.ArgumentList.Add($arg) }
    return [Diagnostics.Process]::Start($info)
}
$parent=$null; $child=$null; $unrelated=$null
try {
    $unrelated=Start-Probe 'Start-Sleep -Seconds 60'
    $parent=Start-Probe '$info=[Diagnostics.ProcessStartInfo]::new((Get-Process -Id $PID).Path); $info.UseShellExecute=$false; $info.CreateNoWindow=$true; foreach($arg in @("-NoProfile","-NonInteractive","-Command","Start-Sleep -Seconds 60")) { $info.ArgumentList.Add($arg) }; $child=[Diagnostics.Process]::Start($info); [Console]::Out.WriteLine($child.Id); [Console]::Out.Flush(); Start-Sleep -Seconds 60'
    $ready=$parent.StandardOutput.ReadLineAsync()
    if (-not $ready.Wait(15000)) { throw 'Child readiness timeout' }
    $child=[Diagnostics.Process]::GetProcessById([int]$ready.Result)
    Stop-OwnedEditorProcess $parent
    if (-not $child.WaitForExit(10000)) { throw 'Editor child survived cleanup' }
    if ($unrelated.HasExited) { throw 'Unrelated process was stopped' }
    Stop-OwnedEditorProcess $parent
    Write-Host 'Owned editor descendants / unrelated process retained / repeated cleanup: PASS'
} finally {
    foreach($probe in @($child,$parent,$unrelated)) {
        if ($null -ne $probe) {
            if (-not $probe.HasExited) { $probe.Kill($true); [void]$probe.WaitForExit(10000) }
            $probe.Dispose()
        }
    }
}

# Load diagnostic helpers without launching VS Code or entering WSL.
foreach ($name in @('Select-VSCodeDiagnosticValue','Get-VSCodeDiagnosticDuration','Write-VSCodeFailureDiagnostic')) {
    $definition=$ast.Find({param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name},$true)
    . ([scriptblock]::Create($definition.Extent.Text))
}
function Get-DiagnosticText($Result) {
    $previous=[Console]::Out
    $output=[IO.StringWriter]::new()
    try {
        [Console]::SetOut($output)
        Write-VSCodeFailureDiagnostic $Result
    } finally { [Console]::SetOut($previous) }
    return $output.ToString()
}
$receipt=[pscustomobject]@{
    checks=@('workspace-marker','SECRET','workspace-marker',7,@('editor-file-read-write'))
    filesystemFailure=[pscustomobject]@{suboperation='editor-open';error_category='unavailable';duration_ms=25;error='SECRET'}
    authority='SECRET';nonce='SECRET';stack='SECRET'
}
$text=Get-DiagnosticText $receipt
$record=$text | ConvertFrom-Json
if ($text.Contains('SECRET') -or $record.suboperation -cne 'editor-open' -or
    $record.error_category -cne 'unavailable' -or $record.duration_ms -ne 25 -or
    ($record.completed_checks -join ',') -cne 'workspace-marker' -or
    ($record.PSObject.Properties.Name -join ',') -cne 'component,operation,suboperation,completed_checks,error_category,duration_ms') { throw 'Diagnostic projection changed or leaked arbitrary data' }
foreach ($step in @('marker-read','marker-validate','editor-write','editor-open','editor-validate','editor-show')) {
    $receipt.filesystemFailure.suboperation=$step
    if (((Get-DiagnosticText $receipt | ConvertFrom-Json).suboperation) -cne $step) { throw 'Known filesystem operation was lost' }
}
foreach ($category in @('type','not-found','exists','not-directory','is-directory','no-permissions','unavailable','other','unobserved')) {
    $receipt.filesystemFailure.error_category=$category
    if (((Get-DiagnosticText $receipt | ConvertFrom-Json).error_category) -cne $category) { throw 'Known filesystem error category was lost' }
}
$receipt.filesystemFailure.suboperation='SECRET'
$receipt.filesystemFailure.error_category='SECRET'
foreach ($duration in @('SECRET','25',-1,0.5,[double]::NaN,[double]::PositiveInfinity,9007199254740992,$true,@(25))) {
    $receipt.filesystemFailure.duration_ms=$duration
    $text=Get-DiagnosticText $receipt
    $record=$text | ConvertFrom-Json
    if ($text.Contains('SECRET') -or $record.suboperation -cne 'unobserved' -or
        $record.error_category -cne 'unobserved' -or $null -ne $record.duration_ms) { throw 'Malformed diagnostic value escaped projection' }
}
if ((Get-DiagnosticText ([pscustomobject]@{checks=@()})) -ne '') { throw 'Missing diagnostics fabricated a result' }
Add-Type @'
using System;
using System.IO;
public sealed class BrokenEditorDiagnosticWriter : StringWriter {
    public bool FailFlush;
    public override void WriteLine(string value) { if (!FailFlush) throw new IOException("SECRET sink"); }
    public override void Flush() { if (FailFlush) throw new IOException("SECRET flush"); }
}
'@
foreach ($flush in @($false,$true)) {
    $previous=[Console]::Out
    $output=[BrokenEditorDiagnosticWriter]::new()
    $output.FailFlush=$flush
    $cleaned=$false; $primary=$null
    try {
        [Console]::SetOut($output)
        try {
            Write-VSCodeFailureDiagnostic $receipt
            throw 'Original editor refusal'
        } finally { $cleaned=$true }
    } catch { $primary=$_.Exception.Message }
    finally { [Console]::SetOut($previous) }
    if ($primary -cne 'Original editor refusal' -or -not $cleaned) { throw 'Diagnostic sink replaced refusal or cleanup' }
}
function ConvertTo-Json { throw 'SECRET serialization' }
try {
    if ((Get-DiagnosticText $receipt) -ne '') { throw 'Failed serialization emitted output' }
} finally { Remove-Item Function:ConvertTo-Json }
Write-Host 'Editor filesystem diagnostic projection / hostile values / failed sinks: PASS'

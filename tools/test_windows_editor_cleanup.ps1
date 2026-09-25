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

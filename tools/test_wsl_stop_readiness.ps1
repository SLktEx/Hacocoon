# Load only pure/control functions; this test never invokes WSL or installer entry.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$tokens = $null
$errors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot '../scripts/install-windows.ps1'), [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw 'Installer syntax failed' }
foreach ($node in $ast.EndBlock.Statements) {
    if ($node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -in @('Wait-WslStopped', 'Assert-SafeName')) {
        . ([scriptblock]::Create($node.Extent.Text))
    }
}
function Get-InstalledDistros { return $script:registered }
function Start-Sleep { param([int]$Milliseconds); $script:delays++ }
function Invoke-WslCapture([string[]]$Arguments) {
    if (($Arguments -join ' ') -cne '--list --running --quiet') { throw 'Unexpected native mutation/probe' }
    $script:observations++
    $index = [Math]::Min($script:observations - 1, $script:responses.Count - 1)
    return $script:responses[$index]
}
foreach ($case in @(
    @{ States = @('Hacocoon', ''); Exit = 0; Registered = @('Hacocoon'); Pass = $true; Calls = 2 },
    @{ States = @(''); Exit = 0; Registered = @('Hacocoon'); Pass = $true; Calls = 1 },
    @{ States = @(''); Exit = 1; Registered = @('Hacocoon'); Pass = $false; Calls = 1 },
    @{ States = @('unexpected response'); Exit = 0; Registered = @('Hacocoon'); Pass = $false; Calls = 1 },
    @{ States = @('Hacocoon'); Exit = 0; Registered = @('Hacocoon'); Pass = $false; Calls = 120 },
    @{ States = @(''); Exit = 0; Registered = @(); Pass = $false; Calls = 0 }
)) {
    $script:registered = $case.Registered
    $script:responses = @($case.States | ForEach-Object { [pscustomobject]@{ ExitCode = $case.Exit; Stdout = $_ } })
    $script:observations = 0
    $script:delays = 0
    $passed = $false
    try { Wait-WslStopped 'Hacocoon'; $passed = $true } catch { }
    if ($passed -ne $case.Pass -or $script:observations -ne $case.Calls) { throw 'WSL stop readiness contract failed' }
    if ($script:delays -gt 119) { throw 'Unbounded WSL stop polling' }
}
Write-Host 'PASS: WSL stop readiness requires successful positive absence observations'

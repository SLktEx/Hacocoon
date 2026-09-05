param([string]$WslTransportDistro)
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

# Load function ASTs only. Never execute installer entry points in component tests.
$installer = Join-Path $PSScriptRoot "../scripts/install-windows.ps1"
$tokens = $null
$errors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($installer, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
foreach ($node in $ast.EndBlock.Statements) {
    if ($node -is [Management.Automation.Language.FunctionDefinitionAst]) {
        . ([scriptblock]::Create($node.Extent.Text))
    }
}
$realCapture = ${function:Invoke-WslCapture}
$ManagedLoginUser = 'hacocoon'
Assert-LoginUserName '_ubuntu-user'
function Assert-Equal($Actual, $Expected) {
    if ($Actual -cne $Expected) { throw "Expected '$Expected', got '$Actual'." }
}
# Hashing must work in the BAT's PS5.1 process without module autoloading.
function Get-FileHash { throw 'Get-FileHash must not be required by the installer' }
$hashFile = Join-Path ([IO.Path]::GetTempPath()) ('haco hash [' + [guid]::NewGuid() + '].bin')
try {
    [IO.File]::WriteAllBytes($hashFile, [Text.Encoding]::UTF8.GetBytes('abc'))
    Assert-Equal (Get-Sha256Hex $hashFile) 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad'
    # On Windows this also verifies the input file handle was disposed.
    [IO.File]::Delete($hashFile)
    $missingFailed = $false
    try { Get-Sha256Hex $hashFile | Out-Null } catch { $missingFailed = $true }
    Assert-Equal $missingFailed $true
} finally {
    [IO.File]::Delete($hashFile)
}
function Get-WslDefaultUser([string]$Name) { return $script:defaultUser }
function Get-WslLoginUser([string]$Name) { return $script:defaultUser }
function Ensure-ManagedWslLoginUser([string]$Name) { $script:managedCalls++; return 'hacocoon' }
function Complete-InteractiveWslUserSetup([string]$Name) { $script:interactiveCalls++; return 'alice' }
function Configure-ManagedWslOobe([string]$Name, [string]$User) {
    Assert-Equal $User 'hacocoon'
    $script:oobeCalls++
}
foreach ($scenario in @('fresh', 'interactive', 'rerun-managed', 'rerun-custom')) {
    $script:defaultUser = switch ($scenario) {
        'rerun-managed' { 'hacocoon' }
        'rerun-custom' { 'alice' }
        default { 'root' }
    }
    $InteractiveUserSetup = $scenario -eq 'interactive'
    $script:managedCalls = $script:interactiveCalls = $script:oobeCalls = 0
    $result = Initialize-WslLoginUser 'Hacocoon' ($scenario -in @('fresh', 'interactive'))
    Assert-Equal $result $(if ($scenario -in @('fresh', 'rerun-managed')) { 'hacocoon' } else { 'alice' })
    Assert-Equal $script:managedCalls $(if ($scenario -eq 'fresh') { 1 } else { 0 })
    Assert-Equal $script:interactiveCalls $(if ($scenario -eq 'interactive') { 1 } else { 0 })
    Assert-Equal $script:oobeCalls $(if ($scenario -in @('fresh', 'rerun-managed')) { 1 } else { 0 })
}
if ($WslTransportDistro) {
    # Optional real PS 5.1 -> WSL transport check. Only mktemp is written, then
    # removed by the product helper; argument contents must stay literal.
    ${function:Invoke-WslCapture} = $realCapture
    $literal = ([string][char]0x65e5) + [char]0x672c + [char]0x8a9e + ' path with spaces; $(touch /tmp/haco-must-not-execute) ` " '' $HOME'
    $result = Invoke-WslRootShellScript $WslTransportDistro 'printf ''%s'' "$1" | base64 -w0' @($literal)
    Assert-Equal $result.ExitCode 0
    Assert-Equal $result.Stdout ([Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($literal)))
}
Write-Host 'WINDOWS INSTALLER COMPONENTS OK'

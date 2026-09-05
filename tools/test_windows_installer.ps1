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
$LoginShell = "/usr/local/libexec/hacocoon-login"
Assert-LoginUserName '_ubuntu-user'

function Assert-Equal($Actual, $Expected) {
    if ($Actual -cne $Expected) { throw "Expected '$Expected', got '$Actual'." }
}
function Write-Step([string]$Message) {}
function Invoke-WslCapture([string[]]$Arguments) {
    $joined = $Arguments -join ' '
    if ($joined -like '*--exec id -un') {
        return New-WslCaptureResult 0 @($script:defaultUser) ""
    }
    if ($joined -like '*--exec getent passwd *') {
        return New-WslCaptureResult 0 @("$($script:defaultUser):x:1007:1013::/home/alice:$($script:defaultShell)") ""
    }
    if ($joined -like '*--exec id alice') { return New-WslCaptureResult 0 @("uid=1007(alice)") "" }
    throw "Unexpected WSL observation: $joined"
}
function wsl.exe {
    Assert-Equal ($args -join ' ') '--distribution Hacocoon'
    $script:interactiveCalls++
    $script:defaultUser = 'alice'
    Write-Output 'Ubuntu setup output must not become the login user'
    $global:LASTEXITCODE = $script:interactiveExit
}

foreach ($scenario in @('fresh', 'interrupted', 'rerun')) {
    $script:defaultUser = if ($scenario -eq 'fresh') { 'root' } else { 'alice' }
    $script:defaultShell = if ($scenario -eq 'rerun') { $LoginShell } else { '/bin/bash' }
    $script:interactiveCalls = 0
    $script:interactiveExit = 0
    Assert-Equal (Complete-WslUserSetup 'Hacocoon') 'alice'
    Assert-Equal $script:interactiveCalls $(if ($scenario -eq 'rerun') { 0 } else { 1 })
}
$script:defaultUser = 'root'
$script:defaultShell = '/bin/bash'
$script:interactiveExit = 1
$failed = $false
try { Complete-WslUserSetup 'Hacocoon' | Out-Null } catch {
    if ($_.Exception.Message -notlike '*interrupted*preserved*') { throw }
    $failed = $true
}
Assert-Equal $failed $true
Remove-Item Function:\wsl.exe

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

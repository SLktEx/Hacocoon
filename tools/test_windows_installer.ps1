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
# Mock only the native command boundary. Product stdout must remain visible,
# never become part of the exit-code decision, and no extra elevation may run.
$systemWsl = Join-Path ([Environment]::SystemDirectory) 'wsl.exe'
$wslFunctionPath = 'function:' + $systemWsl
function Start-Process { throw 'Installer must let WSL own prerequisite elevation' }
Set-Item -LiteralPath $wslFunctionPath -Value {
    Assert-Equal $ErrorActionPreference 'Continue'
    Assert-Equal ($args -join '|') ($script:installArguments -join '|')
    Write-Output 'WSL native progress'
    $global:LASTEXITCODE = $script:installExitCode
}
try {
    $script:installArguments = @('--install', '--from-file', "C:\cache space\a'b;`$(literal)\ubuntu.wsl", '--name', 'Hacocoon', '--no-launch')
    foreach ($code in @(0, 1, 1223)) {
        $script:installExitCode = $code
        $failure = $null
        $output = @()
        try { Invoke-WslInstall 'Hacocoon' $script:installArguments | ForEach-Object { $output += $_ } } catch { $failure = $_ }
        Assert-Equal ($output -join '|') 'WSL native progress'
        Assert-Equal $ErrorActionPreference 'Stop'
        if ($code -eq 0) {
            Assert-Equal ($null -eq $failure) $true
        } else {
            Assert-Equal ($failure.Exception.Message -like "*exit code $code*before common Ubuntu setup*") $true
        }
    }
} finally {
    Remove-Item -LiteralPath $wslFunctionPath
    Remove-Item -LiteralPath function:Start-Process
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
# A failed WSL invocation does not prove account absence. Keep the failure
# closed, preserve the native exit code, and do not reveal arbitrary stderr.
function Invoke-WslCapture([string[]]$Arguments) {
    if (($Arguments -join '|') -eq '--distribution|Hacocoon|--exec|id|-un') {
        return New-WslCaptureResult 0 @('hacocoon') ''
    }
    Assert-Equal ($Arguments -join '|') '--distribution|Hacocoon|--user|root|--exec|id|hacocoon'
    $script:userProbeCalls++
    $probeCode = $script:userProbeExit
    if ($script:userProbeFailures -gt 0) { $script:userProbeFailures--; $probeCode = 1 }
    return New-WslCaptureResult $probeCode @() 'untrusted stderr with secret'
}
foreach ($code in @(1, -1, 127)) {
    $script:userProbeExit = $code
    $script:userProbeCalls = $script:userProbeFailures = 0
    $lookupFailure = $null
    try { Get-WslLoginUser 'Hacocoon' | Out-Null } catch { $lookupFailure = $_ }
    Assert-Equal ($null -ne $lookupFailure) $true
    Assert-Equal $script:userProbeCalls 3
    Assert-Equal ($lookupFailure.Exception.Message -like "*WSL exit code $code*") $true
    Assert-Equal ($lookupFailure.Exception.Message -like '*does not exist*') $false
    Assert-Equal ($lookupFailure.Exception.Message -like '*secret*') $false
}
$script:userProbeExit = 0
$script:userProbeCalls = 0
$script:userProbeFailures = 1
Assert-Equal (Get-WslLoginUser 'Hacocoon') 'hacocoon'
Assert-Equal $script:userProbeCalls 2
$script:userProbeCalls = 0
Assert-Equal (Get-WslLoginUser 'Hacocoon') 'hacocoon'
Assert-Equal $script:userProbeCalls 1
${function:Invoke-WslCapture} = $realCapture

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
    # Actual native PS5.1 boundary, read-only: output on success, error on bad
    # usage, and no stale exit status after a preceding failed command.
    Invoke-WslInstall 'read-only version probe' @('--version') | Out-Null
    $invalidUsage = $false
    try { Invoke-WslInstall 'read-only invalid-option probe' @('--haco-invalid-option') | Out-Null } catch { $invalidUsage = $_.Exception.Message -like '*exit code*' }
    Assert-Equal $invalidUsage $true
    Invoke-WslInstall 'read-only version probe' @('--version') | Out-Null
}
# Load the unchanged cache function in a disposable package directory so its
# real PSScriptRoot is exercised. Mock only the network boundary, never hashing.
$cacheRoot = Join-Path ([IO.Path]::GetTempPath()) ('haco-cache-' + [guid]::NewGuid())
[IO.Directory]::CreateDirectory($cacheRoot) | Out-Null
$cacheFunctionFile = Join-Path $cacheRoot 'cache-functions.ps1'
$cachePath = Join-Path $cacheRoot 'ubuntu.wsl'
$cacheFunction = $ast.EndBlock.Statements | Where-Object { $_ -is [Management.Automation.Language.FunctionDefinitionAst] -and $_.Name -eq 'Get-CachedUbuntuWslImage' }
[IO.File]::WriteAllText($cacheFunctionFile, $cacheFunction.Extent.Text)
$ProgressPreference = 'Continue'
$script:downloads = 0
$script:expectedHash = 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad'
try {
    . $cacheFunctionFile
    function Invoke-RestMethod {
        $asset = @{ Url = 'https://example.invalid/ubuntu.wsl'; Sha256 = $script:expectedHash }
        return @{ ModernDistributions = @{ Ubuntu = @(@{ Name = 'Ubuntu-26.04'; Amd64Url = $asset; Arm64Url = $asset }) } }
    }
    function Invoke-WebRequest([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing) {
        Assert-Equal $ProgressPreference 'SilentlyContinue'
        Assert-Equal $UseBasicParsing.IsPresent $true
        $script:downloads++
        [IO.File]::WriteAllText($OutFile, 'abc')
    }
    Assert-Equal (Get-CachedUbuntuWslImage) $cachePath
    Assert-Equal (Get-CachedUbuntuWslImage) $cachePath
    Assert-Equal $script:downloads 1
    Assert-Equal $ProgressPreference 'Continue'
    [IO.File]::Delete($cachePath)
    $script:expectedHash = '0' * 64
    $hashMismatch = $false
    try { Get-CachedUbuntuWslImage | Out-Null } catch { $hashMismatch = $_.Exception.Message -like '*SHA256 mismatch*' }
    Assert-Equal $hashMismatch $true
    Assert-Equal ([IO.File]::Exists($cachePath)) $false
    Assert-Equal ([IO.File]::Exists($cachePath + '.download')) $false
    Assert-Equal $ProgressPreference 'Continue'
} finally {
    foreach ($file in @($cachePath, ($cachePath + '.download'), $cacheFunctionFile)) { [IO.File]::Delete($file) }
    [IO.Directory]::Delete($cacheRoot)
}
Write-Host 'WINDOWS INSTALLER COMPONENTS OK'

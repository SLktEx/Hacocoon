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
$realInstall = ${function:Invoke-WslInstall}
$realDistros = ${function:Get-InstalledDistros}
$realContinuation = ${function:Write-WslContinuation}
$ManagedLoginUser = 'hacocoon'
Assert-LoginUserName '_ubuntu-user'
function Assert-Equal($Actual, $Expected) {
    if ($Actual -cne $Expected) { throw "Expected '$Expected', got '$Actual'." }
}

# Fresh Windows installs inherit Japanese UI language into the WSL locale. The
# mapping is deliberately narrow so other Windows locales keep Ubuntu defaults.
foreach ($case in @(
    @{ Tag = 'ja-JP'; Want = 'ja_JP.UTF-8' },
    @{ Tag = 'ja'; Want = 'ja_JP.UTF-8' },
    @{ Tag = 'JA-jp'; Want = 'ja_JP.UTF-8' },
    @{ Tag = 'en-US'; Want = '' },
    @{ Tag = ''; Want = '' }
)) {
    Assert-Equal (Convert-WindowsLanguageTagToWslLocale $case.Tag) $case.Want
}
$realRootShell = ${function:Invoke-WslRootShellScript}
$realUiLanguage = ${function:Get-WindowsUiLanguageTag}
function Get-WindowsUiLanguageTag { return $script:windowsUiLanguage }
function Invoke-WslRootShellScript([string]$Name, [string]$Script, [string[]]$ScriptArguments = @()) {
    Assert-Equal $Name 'Hacocoon'
    Assert-Equal ($ScriptArguments -join '|') 'ja_JP.UTF-8'
    Assert-Equal ($Script -like '*locale-gen*') $true
    Assert-Equal ($Script -like '*update-locale LANG*') $true
    $script:localeCalls++
    return New-WslCaptureResult 0 @() ''
}
try {
    $script:windowsUiLanguage = 'ja-JP'
    $script:localeCalls = 0
    Initialize-WslLocaleFromWindows 'Hacocoon' $true
    Assert-Equal $script:localeCalls 1

    $script:localeCalls = 0
    Initialize-WslLocaleFromWindows 'Hacocoon' $false
    Assert-Equal $script:localeCalls 0

    $script:windowsUiLanguage = 'en-US'
    Initialize-WslLocaleFromWindows 'Hacocoon' $true
    Assert-Equal $script:localeCalls 0
} finally {
    ${function:Invoke-WslRootShellScript} = $realRootShell
    ${function:Get-WindowsUiLanguageTag} = $realUiLanguage
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
    foreach ($code in @(0, 1, 1223, 3010)) {
        $script:installExitCode = $code
        $failure = $null
        $output = @()
        try { Invoke-WslInstall 'Hacocoon' $script:installArguments | ForEach-Object { $output += $_ } } catch { $failure = $_ }
        Assert-Equal ($output -join '|') 'WSL native progress'
        Assert-Equal $ErrorActionPreference 'Stop'
        if ($code -eq 0) {
            Assert-Equal ($null -eq $failure) $true
        } elseif ($code -eq 3010) {
            Assert-Equal ($failure.Exception -is [ComponentModel.Win32Exception]) $true
            Assert-Equal $failure.Exception.NativeErrorCode 3010
        } else {
            Assert-Equal ($failure.Exception.Message -like "*exit code $code*before common Ubuntu setup*") $true
        }
    }
} finally {
    Remove-Item -LiteralPath $wslFunctionPath
    Remove-Item -LiteralPath function:Start-Process
}
# A failed listing is unknown, never an empty inventory. Do not echo stderr.
function Invoke-WslCapture([string[]]$Arguments) {
    Assert-Equal ($Arguments -join '|') '--list|--quiet'
    return New-WslCaptureResult $script:listCode @($script:listOutput) 'secret backend detail'
}
try {
    $script:listCode = 0
    $script:listOutput = "H`0a`0c`0o`0c`0o`0o`0n`0`r`nUbuntu-24.04`r`n"
    Assert-Equal (@(Get-InstalledDistros) -join '|') 'Hacocoon|Ubuntu-24.04'
    $script:listOutput = ''
    Assert-Equal (@(Get-InstalledDistros).Count) 0
    foreach ($code in @(1, -1, 1223)) {
        $script:listCode = $code
        $failure = $null
        try { Get-InstalledDistros | Out-Null } catch { $failure = $_ }
        Assert-Equal ($failure.Exception.Message -like "*exit code $code*") $true
        Assert-Equal ($failure.Exception.Message -like '*secret*') $false
    }
} finally { ${function:Invoke-WslCapture} = $realCapture }

# Exercise registration continuation with only native/inventory/file boundaries
# mocked. Native 0 without the exact distro must never advance to common setup.
function Invoke-WslInstall([string]$Name, [string[]]$Arguments) {
    Assert-Equal $Name 'Hacocoon'
    Assert-Equal ($Arguments -join '|') '--install|Ubuntu-26.04|--name|Hacocoon|--no-launch'
    $script:createCalls++
    if ($script:createCode -eq 3010) { throw [ComponentModel.Win32Exception]::new(3010, 'restart') }
    if ($script:createCode -ne 0) { throw 'native failure' }
}
function Get-InstalledDistros {
    $script:listCalls++
    if ($script:inventoryUnknown) { throw 'unknown inventory' }
    return $script:inventory
}
function Write-WslContinuation([string]$Directory, [string]$Name, [bool]$RestartRequired) {
    Assert-Equal $Name 'Hacocoon'
    $script:recordCalls++
    $script:recordRestart = $RestartRequired
}
try {
    foreach ($scenario in @('created', 'zero-without-distro', 'unknown', 'restart', 'cancelled')) {
        $script:createCalls = $script:listCalls = $script:recordCalls = 0
        $script:recordRestart = $false
        $script:createCode = switch ($scenario) { 'restart' { 3010 } 'cancelled' { 1223 } default { 0 } }
        $script:inventoryUnknown = $scenario -eq 'unknown'
        $script:inventory = if ($scenario -eq 'created') { @('Ubuntu', 'Hacocoon') } else { @('Ubuntu') }
        $failure = $null
        try { New-WslInstance 'Hacocoon' @('--install', 'Ubuntu-26.04', '--name', 'Hacocoon', '--no-launch') } catch { $failure = $_ }
        Assert-Equal ($null -eq $failure) ($scenario -eq 'created')
        Assert-Equal $script:createCalls 1
        Assert-Equal $script:listCalls $(if ($script:createCode -eq 0) { 1 } else { 0 })
        Assert-Equal $script:recordCalls $(if ($scenario -eq 'created') { 0 } else { 1 })
        Assert-Equal $script:recordRestart ($scenario -eq 'restart')
    }
} finally {
    ${function:Invoke-WslInstall} = $realInstall
    ${function:Get-InstalledDistros} = $realDistros
    ${function:Write-WslContinuation} = $realContinuation
}

# Continuation records retain validated options, are separate per attempt, and
# cannot overwrite old records. They are informational and are never read back.
$recordRoot = Join-Path ([IO.Path]::GetTempPath()) ('haco-continuation-' + [guid]::NewGuid())
[IO.Directory]::CreateDirectory($recordRoot) | Out-Null
$BaseDistro = 'Ubuntu-26.04'
$HacocoonVersion = 'latest'
$UseCachedWslImage = $InteractiveUserSetup = $true
$WebDownload = $SkipIncus = $GrantIncusAdmin = $false
try {
    Write-WslContinuation $recordRoot 'Hacocoon' $true
    $first = @(Get-ChildItem -LiteralPath $recordRoot -File)
    Assert-Equal $first.Count 1
    $saved = [IO.File]::ReadAllText($first[0].FullName)
    $record = $saved | ConvertFrom-Json
    Assert-Equal $record.schema_version 1
    Assert-Equal $record.stage 'wsl-registration'
    Assert-Equal $record.status 'restart-required'
    Assert-Equal $record.resume_command '.\install-windows.bat -InstanceName Hacocoon -BaseDistro Ubuntu-26.04 -HacocoonVersion latest -UseCachedWslImage -InteractiveUserSetup'
    Write-WslContinuation $recordRoot 'Hacocoon' $false
    Assert-Equal (@(Get-ChildItem -LiteralPath $recordRoot -File).Count) 2
    Assert-Equal ([IO.File]::ReadAllText($first[0].FullName)) $saved
    $failure = $null
    try { Write-WslContinuation $recordRoot 'bad;command' $true } catch { $failure = $_ }
    Assert-Equal ($null -ne $failure) $true
    Assert-Equal (@(Get-ChildItem -LiteralPath $recordRoot -File).Count) 2
} finally {
    foreach ($file in [IO.Directory]::GetFiles($recordRoot)) { [IO.File]::Delete($file) }
    [IO.Directory]::Delete($recordRoot)
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
    Assert-Equal (@(Get-InstalledDistros) -contains $WslTransportDistro) $true
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
# Failure-case probes intentionally change LASTEXITCODE. GitHub's PowerShell
# wrapper returns it after this script, so publish success only after every
# assertion and cleanup has completed. A thrown failure never reaches here.
$global:LASTEXITCODE = 0
Write-Host 'WINDOWS INSTALLER COMPONENTS OK'

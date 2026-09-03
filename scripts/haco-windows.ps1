[CmdletBinding(PositionalBinding = $false)]
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$HacoArgs
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Step([string]$Message) {
    Write-Host "==> $Message"
}

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-SystemWsl {
    $path = Join-Path ([Environment]::SystemDirectory) "wsl.exe"
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "haco: system wsl.exe is unavailable at '$path'."
    }
    return $path
}

function Assert-SafeInstanceName([string]$Value) {
    if ($Value -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$') {
        throw "haco: WSL instance name '$Value' contains unsupported characters."
    }
}

function Get-ConfiguredInstanceName {
    $instancePath = Join-Path $PSScriptRoot "INSTANCE"
    if (-not (Test-Path -LiteralPath $instancePath -PathType Leaf)) {
        return "Hacocoon"
    }
    $name = (Get-Content -LiteralPath $instancePath -Raw).Trim()
    Assert-SafeInstanceName $name
    return $name
}

function Invoke-WslExit([string]$Wsl, [string[]]$Arguments) {
    $previousPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = "Continue"
        $output = @(& $Wsl @Arguments 2>&1)
        $exitCode = [int]$LASTEXITCODE
        foreach ($line in $output) {
            Write-Host $line
        }
        return $exitCode
    } finally {
        $ErrorActionPreference = $previousPreference
    }
}

function Get-InstalledDistros([string]$Wsl) {
    $previousPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = "Continue"
        $lines = @(& $Wsl --list --quiet 2>$null)
        if ($LASTEXITCODE -ne 0) { return @() }
        return @($lines | ForEach-Object { ($_ -replace "`0", "").Trim() } | Where-Object { $_ })
    } finally {
        $ErrorActionPreference = $previousPreference
    }
}

function Get-RunningDistros([string]$Wsl) {
    $previousPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = "Continue"
        $lines = @(& $Wsl --list --running --quiet 2>$null)
        if ($LASTEXITCODE -ne 0) { return @() }
        return @($lines | ForEach-Object { ($_ -replace "`0", "").Trim() } | Where-Object { $_ })
    } finally {
        $ErrorActionPreference = $previousPreference
    }
}

function Resolve-WslVhdPath([string]$InstanceName) {
    $root = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Lxss"
    if (-not (Test-Path -LiteralPath $root)) {
        throw "haco maintenance compact: WSL registration data is unavailable for the current Windows user."
    }

    $matches = @()
    foreach ($key in Get-ChildItem -LiteralPath $root -ErrorAction Stop) {
        $properties = Get-ItemProperty -LiteralPath $key.PSPath -ErrorAction Stop
        if ([string]$properties.DistributionName -eq $InstanceName) {
            $matches += $properties
        }
    }
    if ($matches.Count -ne 1) {
        throw "haco maintenance compact: expected exactly one WSL registration for '$InstanceName', found $($matches.Count)."
    }

    $basePath = [Environment]::ExpandEnvironmentVariables([string]$matches[0].BasePath)
    if ([string]::IsNullOrWhiteSpace($basePath)) {
        throw "haco maintenance compact: WSL registration for '$InstanceName' has no BasePath."
    }
    $vhdPath = Join-Path $basePath "ext4.vhdx"
    if (-not (Test-Path -LiteralPath $vhdPath -PathType Leaf)) {
        throw "haco maintenance compact: expected WSL VHD was not found at '$vhdPath'."
    }
    return (Get-Item -LiteralPath $vhdPath -Force).FullName
}

function Wait-WslStopped([string]$Wsl, [string]$InstanceName) {
    for ($attempt = 0; $attempt -lt 80; $attempt++) {
        if ((Get-RunningDistros $Wsl) -notcontains $InstanceName) {
            return
        }
        Start-Sleep -Milliseconds 250
    }
    throw "haco maintenance compact: '$InstanceName' did not fully stop; refusing to compact a mounted filesystem."
}

function Test-VhdUnlocked([string]$VhdPath) {
    $stream = $null
    try {
        $stream = [IO.File]::Open($VhdPath, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
        return $true
    } catch [IO.IOException] {
        return $false
    } finally {
        if ($null -ne $stream) { $stream.Dispose() }
    }
}

function Wait-VhdUnlocked([string]$VhdPath) {
    for ($attempt = 0; $attempt -lt 80; $attempt++) {
        if (Test-VhdUnlocked $VhdPath) {
            return
        }
        Start-Sleep -Milliseconds 250
    }
    throw "haco maintenance compact: the WSL VHD is still open by another process; refusing host-side compaction. Close WSL-integrated applications and retry."
}

function Ensure-WslVhdOffline([string]$Wsl, [string]$InstanceName, [string]$VhdPath) {
    if (Test-VhdUnlocked $VhdPath) {
        return
    }

    $others = @(Get-RunningDistros $Wsl | Where-Object { $_ -ne $InstanceName })
    if ($others.Count -gt 0) {
        $display = $others -join ", "
        throw "haco maintenance compact: the WSL utility VM still owns '$InstanceName' VHD and other WSL distributions are running ($display). Hacocoon will not stop them. Stop those distributions or WSL-integrated applications yourself, then retry."
    }

    # WSL 2 can keep an ext4.vhdx open after the target distribution has been
    # terminated because the shared utility VM remains alive. With no unrelated
    # distributions running, shutting down that idle VM changes no other distro
    # runtime state and is the supported way to release the VHD before Windows
    # host tools access it.
    Write-Step "No unrelated WSL distributions are running; releasing the idle WSL utility VM"
    $code = Invoke-WslExit $Wsl @("--shutdown")
    if ($code -ne 0) {
        throw "haco maintenance compact: WSL utility VM shutdown failed with exit code $code; VHD compaction was not started."
    }
    Wait-VhdUnlocked $VhdPath
}

function Invoke-Trim([string]$Wsl, [string]$InstanceName) {
    $trim = $null
    foreach ($candidate in @("/usr/sbin/fstrim", "/sbin/fstrim")) {
        $code = Invoke-WslExit $wsl @("--distribution", $InstanceName, "--user", "root", "--exec", "test", "-x", $candidate)
        if ($code -eq 0) {
            $trim = $candidate
            break
        }
    }
    if ($null -eq $trim) {
        Write-Host "Guest fstrim is unavailable; continuing with host-side compaction only."
        return
    }

    Write-Step "Trimming free blocks inside '$InstanceName'"
    $code = Invoke-WslExit $wsl @("--distribution", $InstanceName, "--user", "root", "--exec", $trim, "-av")
    if ($code -ne 0) {
        throw "haco maintenance compact: guest fstrim failed with exit code $code; VHD compaction was not started."
    }
}

function Invoke-SystemProcess([string]$FilePath, [string[]]$Arguments) {
    $start = @{
        FilePath = $FilePath
        ArgumentList = $Arguments
        Wait = $true
        PassThru = $true
    }
    if (-not (Test-Administrator)) {
        $start["Verb"] = "RunAs"
    }
    try {
        $process = Start-Process @start
    } catch {
        throw "haco maintenance compact: administrator approval was cancelled or host compaction could not be started."
    }
    if ($process.ExitCode -ne 0) {
        throw "haco maintenance compact: host compaction process failed with exit code $($process.ExitCode)."
    }
}

function Invoke-OptimizeVhd([string]$VhdPath) {
    if (Test-Administrator) {
        Import-Module Hyper-V -ErrorAction Stop
        Optimize-VHD -Path $VhdPath -Mode Full -ErrorAction Stop
        return
    }

    $powershell = Join-Path ([Environment]::SystemDirectory) "WindowsPowerShell\v1.0\powershell.exe"
    if (-not (Test-Path -LiteralPath $powershell -PathType Leaf)) {
        throw "haco maintenance compact: system Windows PowerShell is unavailable for elevated Optimize-VHD."
    }
    $quoted = $VhdPath.Replace("'", "''")
    $command = "`$ErrorActionPreference='Stop'; Import-Module Hyper-V -ErrorAction Stop; Optimize-VHD -Path '$quoted' -Mode Full -ErrorAction Stop"
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($command))
    Invoke-SystemProcess $powershell @("-NoLogo", "-NoProfile", "-EncodedCommand", $encoded)
}

function Invoke-DiskPartCompact([string]$VhdPath) {
    $diskpart = Join-Path ([Environment]::SystemDirectory) "diskpart.exe"
    if (-not (Test-Path -LiteralPath $diskpart -PathType Leaf)) {
        throw "haco maintenance compact: neither Optimize-VHD nor system diskpart.exe is available. Install the Hyper-V PowerShell module or use a Windows edition with DiskPart VHD support."
    }

    $scriptPath = [IO.Path]::GetTempFileName()
    try {
        $script = "select vdisk file=`"$VhdPath`"`r`ncompact vdisk`r`nexit`r`n"
        [IO.File]::WriteAllText($scriptPath, $script, [Text.Encoding]::ASCII)
        Invoke-SystemProcess $diskpart @("/s", ('"' + $scriptPath + '"'))
    } finally {
        Remove-Item -LiteralPath $scriptPath -Force -ErrorAction SilentlyContinue
    }
}

function Format-ByteSize([long]$Bytes) {
    if ($Bytes -ge 1GB) { return ("{0:N2} GiB" -f ($Bytes / 1GB)) }
    if ($Bytes -ge 1MB) { return ("{0:N2} MiB" -f ($Bytes / 1MB)) }
    if ($Bytes -ge 1KB) { return ("{0:N2} KiB" -f ($Bytes / 1KB)) }
    return "$Bytes bytes"
}

function Invoke-Compact([string]$InstanceName) {
    $wsl = Get-SystemWsl
    if ((Get-InstalledDistros $wsl) -notcontains $InstanceName) {
        throw "haco maintenance compact: dedicated WSL distribution '$InstanceName' is not installed."
    }

    $vhdPath = Resolve-WslVhdPath $InstanceName
    Invoke-Trim $wsl $InstanceName

    Write-Step "Stopping only the dedicated WSL distribution '$InstanceName'"
    $code = Invoke-WslExit $wsl @("--terminate", $InstanceName)
    if ($code -ne 0) {
        throw "haco maintenance compact: failed to terminate '$InstanceName'; VHD compaction was not started."
    }
    Wait-WslStopped $wsl $InstanceName
    Ensure-WslVhdOffline $wsl $InstanceName $vhdPath

    $before = [long](Get-Item -LiteralPath $vhdPath -Force).Length
    Write-Host "VHD before: $(Format-ByteSize $before) ($before bytes)"

    if ($null -ne (Get-Command Optimize-VHD -ErrorAction SilentlyContinue)) {
        Write-Step "Compacting the offline VHD with Optimize-VHD"
        Invoke-OptimizeVhd $vhdPath
    } else {
        Write-Step "Compacting the offline VHD with DiskPart"
        Invoke-DiskPartCompact $vhdPath
    }

    $after = [long](Get-Item -LiteralPath $vhdPath -Force).Length
    $saved = [Math]::Max([long]0, $before - $after)
    Write-Host "VHD after:  $(Format-ByteSize $after) ($after bytes)"
    Write-Host "Reclaimed:  $(Format-ByteSize $saved) ($saved bytes)"

    Write-Step "Validating that '$InstanceName' still mounts after compaction"
    $code = Invoke-WslExit $wsl @("--distribution", $InstanceName, "--user", "root", "--exec", "true")
    if ($code -ne 0) {
        throw "haco maintenance compact: compaction completed but '$InstanceName' could not be started for validation. Do not unregister the distro; inspect WSL diagnostics before taking further action."
    }
    $code = Invoke-WslExit $wsl @("--terminate", $InstanceName)
    if ($code -ne 0) {
        throw "haco maintenance compact: validation succeeded but '$InstanceName' could not be returned to the stopped state."
    }
    Wait-WslStopped $wsl $InstanceName
    Write-Host "Hacocoon WSL compaction complete. The distro remains stopped and will start on next use."
}

function Install-Launcher([string]$InstanceName) {
    Assert-SafeInstanceName $InstanceName
    $wsl = Get-SystemWsl
    if ((Get-InstalledDistros $wsl) -notcontains $InstanceName) { return }
    $code = Invoke-WslExit $wsl @("--distribution", $InstanceName, "--user", "root", "--exec", "test", "-x", "/usr/local/bin/haco")
    if ($code -ne 0) { return }

    if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        throw "haco: LOCALAPPDATA is unavailable; cannot install the Windows launcher."
    }
    $sourceCmd = Join-Path $PSScriptRoot "haco-windows.cmd"
    if (-not (Test-Path -LiteralPath $sourceCmd -PathType Leaf)) {
        throw "haco: Windows installer package is missing haco-windows.cmd."
    }

    $targetBin = Join-Path $env:LOCALAPPDATA "Hacocoon\bin"
    New-Item -ItemType Directory -Path $targetBin -Force | Out-Null
    Copy-Item -LiteralPath $sourceCmd -Destination (Join-Path $targetBin "haco.cmd") -Force
    Copy-Item -LiteralPath $PSCommandPath -Destination (Join-Path $targetBin "haco-windows.ps1") -Force
    [IO.File]::WriteAllText((Join-Path $targetBin "INSTANCE"), $InstanceName + "`n", [Text.Encoding]::ASCII)

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = if ([string]::IsNullOrWhiteSpace($userPath)) { @() } else { @($userPath -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }) }
    $normalizedTarget = $targetBin.TrimEnd('\')
    $present = $false
    foreach ($entry in $entries) {
        if ($entry.Trim().TrimEnd('\') -ieq $normalizedTarget) {
            $present = $true
            break
        }
    }
    if (-not $present) {
        $newPath = (@($targetBin) + $entries) -join ';'
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Host "Installed Windows haco launcher in '$targetBin' and added it to the user PATH. Open a new terminal to use 'haco'."
    } else {
        Write-Host "Updated Windows haco launcher in '$targetBin'."
    }
}

if ($null -eq $HacoArgs) { $HacoArgs = @() }
if ($HacoArgs.Count -gt 0 -and $HacoArgs[0] -eq "__install-launcher") {
    if ($HacoArgs.Count -ne 2) {
        throw "haco: internal launcher installation requires exactly one WSL instance name."
    }
    Install-Launcher $HacoArgs[1]
    exit 0
}

$instanceName = Get-ConfiguredInstanceName
if ($HacoArgs.Count -eq 2 -and $HacoArgs[0] -eq "maintenance" -and $HacoArgs[1] -eq "compact") {
    Invoke-Compact $instanceName
    exit 0
}
if ($HacoArgs.Count -gt 0 -and $HacoArgs[0] -eq "maintenance") {
    throw "usage: haco maintenance compact"
}

$wsl = Get-SystemWsl
if ((Get-InstalledDistros $wsl) -notcontains $instanceName) {
    throw "haco: dedicated WSL distribution '$instanceName' is not installed."
}
$delegateArgs = @("--distribution", $instanceName, "--exec", "/usr/local/bin/haco") + @($HacoArgs)
$code = Invoke-WslExit $wsl $delegateArgs
exit $code

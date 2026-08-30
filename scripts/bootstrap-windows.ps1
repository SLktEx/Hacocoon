[CmdletBinding()]
param(
    [string]$InstanceName = "Hacocoon",
    [string]$BaseDistro = "Ubuntu-26.04",
    [string]$HacocoonVersion = "latest",
    [switch]$WebDownload,
    [switch]$SkipIncus,
    [switch]$GrantIncusAdmin
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$SystemdRestartRequired = 42

function Write-Step([string]$Message) {
    Write-Host "==> $Message"
}

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-InstalledDistros {
    $lines = & wsl.exe --list --quiet 2>$null
    if ($LASTEXITCODE -ne 0) {
        return @()
    }
    return @($lines | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" })
}

function Get-WslGeneration([string]$Name) {
    $lines = & wsl.exe --list --verbose 2>$null
    if ($LASTEXITCODE -ne 0) {
        throw "Unable to inspect WSL distributions."
    }
    $escaped = [regex]::Escape($Name)
    foreach ($line in $lines) {
        if ($line -match "^\s*\*?\s*$escaped\s+.*\s+([12])\s*$") {
            return [int]$Matches[1]
        }
    }
    throw "Unable to determine the WSL version for '$Name'."
}

function Ensure-Wsl2([string]$Name) {
    if ((Get-WslGeneration $Name) -eq 2) {
        return
    }

    Write-Step "Converting dedicated WSL instance '$Name' to WSL 2"
    & wsl.exe --set-version $Name 2
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to convert dedicated WSL instance '$Name' to WSL 2."
    }
    if ((Get-WslGeneration $Name) -ne 2) {
        throw "Dedicated WSL instance '$Name' is not running as WSL 2 after conversion."
    }
}

function Assert-SystemdSupported {
    $null = (& wsl.exe --version 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0) {
        throw "This WSL installation is too old for supported systemd integration. Update WSL explicitly with 'wsl --update', then run this bootstrap again."
    }
}

function Assert-SystemdActive([string]$Name) {
    $pid1 = (& wsl.exe --distribution $Name -- sh -c "ps -p 1 -o comm=").Trim()
    if ($LASTEXITCODE -ne 0 -or $pid1 -ne "systemd") {
        throw "systemd is not active as PID 1 inside dedicated WSL instance '$Name'."
    }
}

function Assert-SafeName([string]$Value, [string]$Label) {
    if ($Value -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$') {
        throw "$Label '$Value' contains unsupported characters. Use letters, digits, '.', '_' or '-'."
    }
}

if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
    throw "wsl.exe is not available. This bootstrap requires a supported Windows 10/11 installation."
}

Assert-SafeName $InstanceName "WSL instance name"
Assert-SafeName $BaseDistro "WSL base distribution"

$installed = @(Get-InstalledDistros)
if (-not ($installed -contains $InstanceName)) {
    Assert-SystemdSupported
    if (-not (Test-Administrator)) {
        throw "Creating the dedicated Hacocoon WSL instance requires an elevated PowerShell. Re-run this script as Administrator."
    }

    Write-Step "Creating dedicated WSL 2 instance '$InstanceName' from '$BaseDistro'"
    $installArgs = @("--install", $BaseDistro, "--name", $InstanceName, "--no-launch")
    if ($WebDownload) {
        $installArgs += "--web-download"
    }
    & wsl.exe @installArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to create dedicated WSL instance '$InstanceName' from '$BaseDistro'. Run 'wsl --list --online' to inspect available base distributions."
    }

    Ensure-Wsl2 $InstanceName

    Write-Host ""
    Write-Host "Dedicated Hacocoon WSL 2 instance '$InstanceName' was installed."
    Write-Host "Existing WSL distributions and global WSL defaults were not modified."
    Write-Host "Windows or the new instance may require a reboot or first-launch Linux user setup."
    Write-Host "Launch it once with: wsl -d $InstanceName"
    Write-Host "Complete the Linux user setup if prompted, then run this bootstrap again."
    exit 0
}

Assert-SystemdSupported
Ensure-Wsl2 $InstanceName

Write-Step "Checking dedicated WSL 2 instance '$InstanceName'"
& wsl.exe --distribution $InstanceName -- sh -c "printf hacocoon-wsl-ready"
if ($LASTEXITCODE -ne 0) {
    throw "Dedicated WSL instance '$InstanceName' exists but is not ready. Launch it once with 'wsl -d $InstanceName', complete first-run setup, and re-run this script."
}
Write-Host ""

$repoRoot = Split-Path -Parent $PSScriptRoot
$linuxRepoRoot = (& wsl.exe --distribution $InstanceName -- wslpath -u -a $repoRoot).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($linuxRepoRoot)) {
    throw "Failed to translate the Hacocoon repository path into the dedicated WSL instance."
}
$linuxBootstrap = "$linuxRepoRoot/scripts/bootstrap-wsl.sh"
$linuxInstaller = "$linuxRepoRoot/scripts/install.sh"
$skipIncusValue = if ($SkipIncus) { "1" } else { "0" }
$grantIncusAdminValue = if ($GrantIncusAdmin) { "1" } else { "0" }

if ($GrantIncusAdmin) {
    Write-Warning "Granting incus-admin gives the Linux user root-equivalent local Incus authority."
}

$wslArgs = @(
    "--distribution", $InstanceName,
    "--",
    "env",
    "HACO_BOOTSTRAP_SKIP_INCUS=$skipIncusValue",
    "HACO_BOOTSTRAP_GRANT_INCUS_ADMIN=$grantIncusAdminValue",
    "sh", $linuxBootstrap, $linuxInstaller, $HacocoonVersion
)

Write-Step "Installing systemd, Incus and Hacocoon inside dedicated WSL instance '$InstanceName'"
& wsl.exe @wslArgs
$bootstrapExit = $LASTEXITCODE

if ($bootstrapExit -eq $SystemdRestartRequired) {
    Write-Step "Restarting dedicated WSL instance '$InstanceName' to activate systemd"
    & wsl.exe --terminate $InstanceName
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to terminate dedicated WSL instance '$InstanceName' for systemd activation."
    }
    Start-Sleep -Milliseconds 750

    & wsl.exe @wslArgs
    $bootstrapExit = $LASTEXITCODE
}

if ($bootstrapExit -eq $SystemdRestartRequired) {
    throw "systemd still requires a restart after the dedicated WSL instance was restarted."
}
if ($bootstrapExit -ne 0) {
    throw "Hacocoon WSL bootstrap failed."
}

Assert-SystemdActive $InstanceName

Write-Host ""
Write-Step "Bootstrap complete"
Write-Host "Dedicated WSL instance: $InstanceName (WSL 2)"
Write-Host "Init system: systemd"
Write-Host "Base distribution: $BaseDistro"
Write-Host "Existing WSL distributions and global WSL defaults remain separate and untouched."
Write-Host "Next: wsl -d $InstanceName"
if ($SkipIncus) {
    Write-Host "Because -SkipIncus was used, this opens the Physical Host; haco-host automatic entry is not configured."
} else {
    Write-Host "Interactive default entry opens the trusted haco-host management environment."
    Write-Host "Physical Host recovery/debug: wsl -d $InstanceName -u root"
}
if (-not $SkipIncus -and -not $GrantIncusAdmin) {
    Write-Host "Broad incus-admin access remains ungranted; default haco-host entry uses only the narrow Hacocoon host commands configured by the bootstrap."
}

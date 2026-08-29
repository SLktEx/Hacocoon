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

function Assert-Wsl2([string]$Name) {
    $lines = & wsl.exe --list --verbose 2>$null
    if ($LASTEXITCODE -ne 0) {
        throw "Unable to inspect WSL distributions."
    }
    $escaped = [regex]::Escape($Name)
    foreach ($line in $lines) {
        if ($line -match "^\s*\*?\s*$escaped\s+.*\s+([12])\s*$") {
            if ($Matches[1] -ne "2") {
                throw "Dedicated WSL instance '$Name' is WSL 1. Convert it explicitly with: wsl --set-version $Name 2"
            }
            return
        }
    }
    throw "Unable to determine the WSL version for '$Name'."
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

    Write-Host ""
    Write-Host "Dedicated Hacocoon WSL instance '$InstanceName' was installed."
    Write-Host "Existing WSL distributions and global WSL defaults were not modified."
    Write-Host "Windows or the new instance may require a reboot or first-launch Linux user setup."
    Write-Host "Launch it once with: wsl -d $InstanceName"
    Write-Host "Complete the Linux user setup if prompted, then run this bootstrap again."
    exit 0
}

Assert-Wsl2 $InstanceName

Write-Step "Checking dedicated WSL instance '$InstanceName'"
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

Write-Step "Installing Hacocoon inside dedicated WSL instance '$InstanceName'"
$wslArgs = @(
    "--distribution", $InstanceName,
    "--",
    "env",
    "HACO_BOOTSTRAP_SKIP_INCUS=$skipIncusValue",
    "HACO_BOOTSTRAP_GRANT_INCUS_ADMIN=$grantIncusAdminValue",
    "sh", $linuxBootstrap, $linuxInstaller, $HacocoonVersion
)
& wsl.exe @wslArgs
if ($LASTEXITCODE -ne 0) {
    throw "Hacocoon WSL bootstrap failed."
}

Write-Host ""
Write-Step "Bootstrap complete"
Write-Host "Dedicated WSL instance: $InstanceName"
Write-Host "Base distribution: $BaseDistro"
Write-Host "Existing WSL distributions and global WSL defaults remain separate and untouched."
Write-Host "Next: run 'wsl -d $InstanceName', place workspaces in its Linux filesystem, then run 'haco-vscode open .'"
if (-not $SkipIncus -and -not $GrantIncusAdmin) {
    Write-Host "Incus was installed, but incus-admin was not granted. Re-run with -GrantIncusAdmin if this user should control the local Incus daemon."
}

[CmdletBinding()]
param(
    [string]$Distro = "",
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

function Get-DefaultDistro {
    $lines = & wsl.exe --list --verbose 2>$null
    if ($LASTEXITCODE -ne 0) {
        return $null
    }
    foreach ($line in $lines) {
        if ($line -match '^\s*\*\s+([^\s]+)\s+') {
            return $Matches[1]
        }
    }
    return $null
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
                throw "WSL distribution '$Name' is WSL 1. Convert it explicitly with: wsl --set-version $Name 2"
            }
            return
        }
    }
    throw "Unable to determine the WSL version for '$Name'."
}

if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
    throw "wsl.exe is not available. This bootstrap requires a supported Windows 10/11 installation."
}

$installed = @(Get-InstalledDistros)
$selected = $Distro
if ([string]::IsNullOrWhiteSpace($selected)) {
    $selected = Get-DefaultDistro
    if ([string]::IsNullOrWhiteSpace($selected) -and $installed.Count -gt 0) {
        $selected = $installed[0]
    }
}

if ([string]::IsNullOrWhiteSpace($selected) -or -not ($installed -contains $selected)) {
    if ([string]::IsNullOrWhiteSpace($selected)) {
        $selected = "Ubuntu"
    }
    if (-not (Test-Administrator)) {
        throw "Installing WSL or a new distribution requires an elevated PowerShell. Re-run this script as Administrator."
    }

    Write-Step "Installing WSL 2 distribution '$selected'"
    & wsl.exe --set-default-version 2
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to set WSL 2 as the default version."
    }

    $installArgs = @("--install", "--distribution", $selected, "--no-launch")
    if ($WebDownload) {
        $installArgs += "--web-download"
    }
    & wsl.exe @installArgs
    if ($LASTEXITCODE -ne 0) {
        throw "WSL distribution installation failed. Run 'wsl --list --online' to inspect valid distribution names."
    }

    Write-Host ""
    Write-Host "WSL distribution '$selected' was installed."
    Write-Host "Windows or the distribution may require first-launch initialization or a reboot."
    Write-Host "Launch it once with: wsl -d $selected"
    Write-Host "Complete the Linux user setup if prompted, then run this bootstrap again."
    exit 0
}

Assert-Wsl2 $selected

Write-Step "Checking that '$selected' can execute Linux commands"
& wsl.exe --distribution $selected -- sh -c "printf hacocoon-wsl-ready"
if ($LASTEXITCODE -ne 0) {
    throw "The WSL distribution '$selected' is installed but not ready. Launch it once with 'wsl -d $selected', complete first-run setup, and re-run this script."
}
Write-Host ""

$repoRoot = Split-Path -Parent $PSScriptRoot
$linuxRepoRoot = (& wsl.exe --distribution $selected -- wslpath -u -a $repoRoot).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($linuxRepoRoot)) {
    throw "Failed to translate the Hacocoon repository path into WSL."
}
$linuxBootstrap = "$linuxRepoRoot/scripts/bootstrap-wsl.sh"
$linuxInstaller = "$linuxRepoRoot/scripts/install.sh"
$skipIncusValue = if ($SkipIncus) { "1" } else { "0" }
$grantIncusAdminValue = if ($GrantIncusAdmin) { "1" } else { "0" }

if ($GrantIncusAdmin) {
    Write-Warning "Granting incus-admin gives the Linux user root-equivalent local Incus authority."
}

Write-Step "Installing Hacocoon inside '$selected'"
& wsl.exe --distribution $selected -- env \
    "HACO_BOOTSTRAP_SKIP_INCUS=$skipIncusValue" \
    "HACO_BOOTSTRAP_GRANT_INCUS_ADMIN=$grantIncusAdminValue" \
    sh $linuxBootstrap $linuxInstaller $HacocoonVersion
if ($LASTEXITCODE -ne 0) {
    throw "Hacocoon WSL bootstrap failed."
}

Write-Host ""
Write-Step "Bootstrap complete"
Write-Host "WSL distribution: $selected"
Write-Host "Next: open a workspace inside the WSL Linux filesystem and run 'haco-vscode open .'"
if (-not $SkipIncus -and -not $GrantIncusAdmin) {
    Write-Host "Incus was installed, but incus-admin was not granted. Re-run with -GrantIncusAdmin if this user should control the local Incus daemon."
}

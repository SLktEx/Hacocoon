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

$Repository = "SLktEx/Hacocoon"

function Write-Step([string]$Message) {
    Write-Host "==> $Message"
}

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Assert-SafeName([string]$Value, [string]$Label) {
    if ($Value -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$') {
        throw "$Label '$Value' contains unsupported characters. Use letters, digits, '.', '_' or '-'."
    }
}

function Assert-Version([string]$Version) {
    if ($Version -eq "latest") {
        return
    }
    if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$') {
        throw "Invalid Hacocoon version '$Version'."
    }
}

function Get-ReleaseBase([string]$Version) {
    if ($Version -eq "latest") {
        return "https://github.com/$Repository/releases/latest/download"
    }
    return "https://github.com/$Repository/releases/download/$Version"
}

function Download-ReleaseAsset([string]$Name, [string]$Destination, [string]$Version) {
    $downloaded = $false

    $gh = Get-Command gh.exe -ErrorAction SilentlyContinue
    if (-not $gh) {
        $gh = Get-Command gh -ErrorAction SilentlyContinue
    }
    if ($gh) {
        & $gh.Source auth status *> $null
        if ($LASTEXITCODE -eq 0) {
            $args = @("release", "download")
            if ($Version -ne "latest") {
                $args += $Version
            }
            $args += @(
                "--repo", $Repository,
                "--pattern", $Name,
                "--dir", (Split-Path -Parent $Destination),
                "--clobber"
            )
            & $gh.Source @args
            if ($LASTEXITCODE -eq 0 -and (Test-Path -LiteralPath $Destination)) {
                $downloaded = $true
            }
        }
    }

    if (-not $downloaded) {
        $headers = @{}
        $token = if ($env:GH_TOKEN) {
            $env:GH_TOKEN
        } elseif ($env:GITHUB_TOKEN) {
            $env:GITHUB_TOKEN
        } else {
            $null
        }
        if ($token) {
            $headers["Authorization"] = "Bearer $token"
        }

        $uri = "$(Get-ReleaseBase $Version)/$Name"
        try {
            Invoke-WebRequest -UseBasicParsing -Uri $uri -OutFile $Destination -Headers $headers
        } catch {
            throw "Failed to download '$Name'. Public releases need network access; private releases require an authenticated gh CLI or GH_TOKEN/GITHUB_TOKEN. $($_.Exception.Message)"
        }
    }

    if (-not (Test-Path -LiteralPath $Destination)) {
        throw "Failed to download release asset '$Name'."
    }
}

function Get-ExpectedHash([string]$ChecksumsPath, [string]$Name) {
    foreach ($line in Get-Content -LiteralPath $ChecksumsPath) {
        if ($line -match '^([0-9a-fA-F]{64})\s+\*?(.+)$' -and $Matches[2] -eq $Name) {
            return $Matches[1].ToLowerInvariant()
        }
    }
    throw "Checksum for '$Name' was not found in checksums.txt."
}

function Assert-Sha256([string]$Path, [string]$Expected) {
    $actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $Expected.ToLowerInvariant()) {
        throw "Checksum verification failed for '$(Split-Path -Leaf $Path)'."
    }
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

Assert-SafeName $InstanceName "WSL instance name"
Assert-SafeName $BaseDistro "WSL base distribution"
Assert-Version $HacocoonVersion

if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
    throw "wsl.exe is not available. This installer requires a supported Windows 10/11 installation."
}

$installed = @(Get-InstalledDistros)
if (-not ($installed -contains $InstanceName)) {
    if (-not (Test-Administrator)) {
        throw "Creating the dedicated Hacocoon WSL instance requires an elevated PowerShell. Re-run this installer as Administrator."
    }

    Write-Step "Creating dedicated WSL 2 instance '$InstanceName' from '$BaseDistro'"
    & wsl.exe --set-default-version 2
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to set WSL 2 as the default version for new distributions."
    }

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
    Write-Host "Existing WSL distributions were not modified."
    Write-Host "Windows or the new instance may require a reboot or first-launch Linux user setup."
    Write-Host "Launch it once with: wsl -d $InstanceName"
    Write-Host "Complete the Linux user setup if prompted, then run this installer again."
    exit 0
}

Assert-Wsl2 $InstanceName

Write-Step "Checking dedicated WSL instance '$InstanceName'"
& wsl.exe --distribution $InstanceName -- sh -c "printf hacocoon-wsl-ready"
if ($LASTEXITCODE -ne 0) {
    throw "Dedicated WSL instance '$InstanceName' exists but is not ready. Launch it once with 'wsl -d $InstanceName', complete first-run setup, and run this installer again."
}
Write-Host ""

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("hacocoon-install-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempRoot | Out-Null
try {
    $checksumsPath = Join-Path $tempRoot "checksums.txt"
    $bootstrapPath = Join-Path $tempRoot "bootstrap-wsl.sh"
    $linuxInstallerPath = Join-Path $tempRoot "install.sh"

    Write-Step "Downloading Hacocoon bootstrap assets"
    Download-ReleaseAsset "checksums.txt" $checksumsPath $HacocoonVersion
    Download-ReleaseAsset "bootstrap-wsl.sh" $bootstrapPath $HacocoonVersion
    Download-ReleaseAsset "install.sh" $linuxInstallerPath $HacocoonVersion

    Assert-Sha256 $bootstrapPath (Get-ExpectedHash $checksumsPath "bootstrap-wsl.sh")
    Assert-Sha256 $linuxInstallerPath (Get-ExpectedHash $checksumsPath "install.sh")

    $linuxTemp = (& wsl.exe --distribution $InstanceName -- wslpath -u -a $tempRoot).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($linuxTemp)) {
        throw "Failed to translate the temporary installer path into the dedicated WSL instance."
    }

    $skipIncusValue = if ($SkipIncus) { "1" } else { "0" }
    $grantIncusAdminValue = if ($GrantIncusAdmin) { "1" } else { "0" }
    if ($GrantIncusAdmin) {
        Write-Warning "Granting incus-admin gives the Linux user root-equivalent local Incus authority."
    }

    Write-Step "Installing Incus and Hacocoon inside '$InstanceName'"
    $wslArgs = @(
        "--distribution", $InstanceName,
        "--",
        "env",
        "HACO_BOOTSTRAP_SKIP_INCUS=$skipIncusValue",
        "HACO_BOOTSTRAP_GRANT_INCUS_ADMIN=$grantIncusAdminValue",
        "sh", "$linuxTemp/bootstrap-wsl.sh", "$linuxTemp/install.sh", $HacocoonVersion
    )
    & wsl.exe @wslArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Hacocoon WSL installation failed."
    }
}
finally {
    Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Step "Hacocoon installation complete"
Write-Host "Dedicated WSL instance: $InstanceName"
Write-Host "Base distribution: $BaseDistro"
Write-Host "Existing WSL distributions remain separate and untouched."
Write-Host "Next: wsl -d $InstanceName"
Write-Host "Then place a workspace in the Linux filesystem and run: haco-vscode open ."
if (-not $SkipIncus -and -not $GrantIncusAdmin) {
    Write-Host "Incus was installed, but incus-admin was not granted. Re-run with -GrantIncusAdmin if this user should control the local Incus daemon."
}

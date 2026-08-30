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
$SignerWorkflow = "$Repository/.github/workflows/release.yml"
$SignerSourceRef = "refs/heads/main"
$ReleasePredicateType = "https://hacocoon.dev/attestations/release/v1"
$SystemdRestartRequired = 42

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
    Assert-ReleaseTag $Version
}

function Assert-ReleaseTag([string]$Version) {
    if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$') {
        throw "Invalid Hacocoon version '$Version'."
    }
}

function Get-GhCommand {
    $gh = Get-Command gh.exe -ErrorAction SilentlyContinue
    if (-not $gh) {
        $gh = Get-Command gh -ErrorAction SilentlyContinue
    }
    if (-not $gh) {
        throw "Trusted public installation requires GitHub CLI with 'gh attestation verify' support."
    }

    & $gh.Source attestation verify --help *> $null
    if ($LASTEXITCODE -ne 0) {
        throw "Trusted public installation requires a GitHub CLI version with 'gh attestation verify' support."
    }
    return $gh
}

function Resolve-ReleaseVersion([string]$Version) {
    if ($Version -ne "latest") {
        Assert-ReleaseTag $Version
        return $Version
    }

    $gh = Get-GhCommand
    $tagOutput = & $gh.Source release view --repo $Repository --json tagName --jq .tagName 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to resolve the latest Hacocoon release to an explicit tag: $($tagOutput | Out-String)"
    }
    $tag = ($tagOutput | Out-String).Trim()
    Assert-ReleaseTag $tag
    Write-Host "Resolved latest Hacocoon release to $tag."
    return $tag
}

function Assert-NamedInstallSupported {
    $helpText = (& wsl.exe --help 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0 -or $helpText -notmatch '(?m)--name\b') {
        throw "This WSL installation does not support named distribution installation. Update WSL explicitly with 'wsl --update', then run the Hacocoon installer again."
    }
}

function Assert-SystemdSupported {
    $null = (& wsl.exe --version 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0) {
        throw "This WSL installation is too old for supported systemd integration. Update WSL explicitly with 'wsl --update', then run the Hacocoon installer again."
    }
}

function Get-ReleaseBase([string]$Version) {
    Assert-ReleaseTag $Version
    return "https://github.com/$Repository/releases/download/$Version"
}

function Download-ReleaseAsset([string]$Name, [string]$Destination, [string]$Version) {
    Assert-ReleaseTag $Version
    $downloaded = $false

    $gh = Get-Command gh.exe -ErrorAction SilentlyContinue
    if (-not $gh) {
        $gh = Get-Command gh -ErrorAction SilentlyContinue
    }
    if ($gh) {
        & $gh.Source auth status *> $null
        if ($LASTEXITCODE -eq 0) {
            $args = @(
                "release", "download", $Version,
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

function Assert-TrustedReleaseAsset([string]$Path, [string]$Version) {
    Assert-ReleaseTag $Version
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Release asset does not exist for provenance verification: $Path"
    }

    $gh = Get-GhCommand
    & $gh.Source attestation verify $Path `
        --repo $Repository `
        --signer-workflow $SignerWorkflow `
        --source-ref $SignerSourceRef `
        --deny-self-hosted-runners *> $null
    if ($LASTEXITCODE -ne 0) {
        throw "Trusted build provenance verification failed for '$(Split-Path -Leaf $Path)'."
    }

    $bindingOutput = & $gh.Source attestation verify $Path `
        --repo $Repository `
        --signer-workflow $SignerWorkflow `
        --source-ref $SignerSourceRef `
        --predicate-type $ReleasePredicateType `
        --deny-self-hosted-runners `
        --format json 2>$null
    if ($LASTEXITCODE -ne 0) {
        throw "Signed release-binding verification failed for '$(Split-Path -Leaf $Path)'."
    }

    try {
        $records = @((($bindingOutput | Out-String) | ConvertFrom-Json))
        $tags = @($records | ForEach-Object { $_.verificationResult.statement.predicate.tag })
    } catch {
        throw "Signed release-binding attestation could not be decoded for '$(Split-Path -Leaf $Path)': $($_.Exception.Message)"
    }
    if ($tags -notcontains $Version) {
        throw "Signed release-binding verification failed for '$(Split-Path -Leaf $Path)': expected tag $Version."
    }

    Write-Host "Verified trusted provenance and signed release binding for $(Split-Path -Leaf $Path)."
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

function Assert-SystemdActive([string]$Name) {
    $pid1 = (& wsl.exe --distribution $Name -- sh -c "ps -p 1 -o comm=").Trim()
    if ($LASTEXITCODE -ne 0 -or $pid1 -ne "systemd") {
        throw "systemd is not active as PID 1 inside dedicated WSL instance '$Name'."
    }
}

# Tests dot-source this script to exercise trust helpers without invoking WSL.
if ($MyInvocation.InvocationName -eq '.') {
    return
}

Assert-SafeName $InstanceName "WSL instance name"
Assert-SafeName $BaseDistro "WSL base distribution"
Assert-Version $HacocoonVersion

if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
    throw "wsl.exe is not available. This installer requires a supported Windows 10/11 installation."
}

$installed = @(Get-InstalledDistros)
if (-not ($installed -contains $InstanceName)) {
    Assert-NamedInstallSupported
    Assert-SystemdSupported
    if (-not (Test-Administrator)) {
        throw "Creating the dedicated Hacocoon WSL instance requires an elevated PowerShell. Re-run this installer as Administrator."
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
    Write-Host "Complete the Linux user setup if prompted, then run this installer again."
    exit 0
}

Assert-SystemdSupported
Ensure-Wsl2 $InstanceName

Write-Step "Checking dedicated WSL 2 instance '$InstanceName'"
& wsl.exe --distribution $InstanceName -- sh -c "printf hacocoon-wsl-ready"
if ($LASTEXITCODE -ne 0) {
    throw "Dedicated WSL instance '$InstanceName' exists but is not ready. Launch it once with 'wsl -d $InstanceName', complete first-run setup, and run this installer again."
}
Write-Host ""

$ResolvedHacocoonVersion = Resolve-ReleaseVersion $HacocoonVersion
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("hacocoon-install-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempRoot | Out-Null
try {
    $checksumsPath = Join-Path $tempRoot "checksums.txt"
    $bootstrapPath = Join-Path $tempRoot "bootstrap-wsl.sh"
    $linuxInstallerPath = Join-Path $tempRoot "install.sh"

    Write-Step "Downloading Hacocoon bootstrap assets"
    Download-ReleaseAsset "checksums.txt" $checksumsPath $ResolvedHacocoonVersion
    Download-ReleaseAsset "bootstrap-wsl.sh" $bootstrapPath $ResolvedHacocoonVersion
    Download-ReleaseAsset "install.sh" $linuxInstallerPath $ResolvedHacocoonVersion

    Assert-Sha256 $bootstrapPath (Get-ExpectedHash $checksumsPath "bootstrap-wsl.sh")
    Assert-Sha256 $linuxInstallerPath (Get-ExpectedHash $checksumsPath "install.sh")

    Write-Step "Verifying trusted release provenance before executing bootstrap assets"
    Assert-TrustedReleaseAsset $checksumsPath $ResolvedHacocoonVersion
    Assert-TrustedReleaseAsset $bootstrapPath $ResolvedHacocoonVersion
    Assert-TrustedReleaseAsset $linuxInstallerPath $ResolvedHacocoonVersion

    $linuxTemp = (& wsl.exe --distribution $InstanceName -- wslpath -u -a $tempRoot).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($linuxTemp)) {
        throw "Failed to translate the temporary installer path into the dedicated WSL instance."
    }

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
        "HACO_REQUIRE_PROVENANCE=1",
        "sh", "$linuxTemp/bootstrap-wsl.sh", "$linuxTemp/install.sh", $ResolvedHacocoonVersion
    )

    Write-Step "Installing systemd, Incus and Hacocoon inside '$InstanceName'"
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
        throw "Hacocoon WSL installation failed."
    }

    Assert-SystemdActive $InstanceName
}
finally {
    Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Step "Hacocoon installation complete"
Write-Host "Dedicated WSL instance: $InstanceName (WSL 2)"
Write-Host "Init system: systemd"
Write-Host "Base distribution: $BaseDistro"
Write-Host "Release: $ResolvedHacocoonVersion"
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

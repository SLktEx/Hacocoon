[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$Distro = "HacocoonE2E"
$Repo = (Get-Location).Path
$Created = $false

function Normalize-WslText([string]$Text) {
    if ($null -eq $Text) {
        return ""
    }
    return $Text.Replace([char]0, '').Trim()
}

function Get-WslText {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)
    $text = (& wsl.exe @Arguments 2>&1 | Out-String)
    $exit = $LASTEXITCODE
    $text = Normalize-WslText $text
    if ($exit -ne 0) {
        throw "wsl.exe $($Arguments -join ' ') failed with exit code $exit`: $text"
    }
    return $text
}

function Invoke-WslChecked {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)
    & wsl.exe @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "wsl.exe $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

function Write-Probe([string]$Title, [scriptblock]$Action) {
    Write-Host "::group::$Title"
    try {
        & $Action
    }
    finally {
        Write-Host "::endgroup::"
    }
}

try {
    Write-Probe "Windows / WSL probe" {
        Get-ComputerInfo | Select-Object WindowsProductName, WindowsVersion, OsBuildNumber | Format-List
        Write-Host (Get-WslText @("--version"))
        Write-Host (Get-WslText @("--status"))
        try {
            Write-Host (Get-WslText @("--list", "--verbose"))
        }
        catch {
            Write-Host "No distro is installed yet: $($_.Exception.Message)"
        }
        Write-Host (Get-WslText @("--list", "--online"))
    }

    $help = Get-WslText @("--help")
    if ($help -notmatch '(?m)--name\b') {
        throw "GitHub-hosted Windows runner does not expose the named WSL install contract required by Hacocoon"
    }

    $existingText = ""
    try {
        $existingText = Get-WslText @("--list", "--quiet")
    }
    catch {
        $existingText = ""
    }
    $existing = @($existingText -split "`r?`n" | ForEach-Object { $_.Trim() } | Where-Object { $_ })
    if ($existing -contains $Distro) {
        Invoke-WslChecked @("--unregister", $Distro)
    }

    Write-Host "==> Installing dedicated Ubuntu 26.04 WSL2 distro '$Distro'"
    Invoke-WslChecked @("--install", "Ubuntu-26.04", "--name", $Distro, "--no-launch", "--web-download")
    $Created = $true

    Invoke-WslChecked @("--set-version", $Distro, "2")

    Write-Host "==> Initializing distro non-interactively for CI"
    Invoke-WslChecked @(
        "--distribution", $Distro,
        "--user", "root",
        "--", "sh", "-lc",
        "printf '[boot]\\nsystemd=true\\n[user]\\ndefault=root\\n' > /etc/wsl.conf"
    )
    Invoke-WslChecked @("--terminate", $Distro)
    Start-Sleep -Seconds 2

    $pid1 = Get-WslText @("--distribution", $Distro, "--user", "root", "--", "sh", "-lc", "ps -p 1 -o comm=")
    if ($pid1 -ne "systemd") {
        throw "systemd is not PID 1 inside the GitHub-hosted WSL2 distro (got '$pid1')"
    }

    $versionCommand = '. /etc/os-release; printf "%s" "$VERSION_ID"'
    $version = Get-WslText @("--distribution", $Distro, "--user", "root", "--", "sh", "-lc", $versionCommand)
    if ($version -ne "26.04") {
        throw "expected Ubuntu 26.04 inside WSL, got '$version'"
    }

    $LinuxRepo = Get-WslText @("--distribution", $Distro, "--user", "root", "--", "wslpath", "-u", "-a", $Repo)
    if ([string]::IsNullOrWhiteSpace($LinuxRepo)) {
        throw "failed to translate repository path into WSL"
    }

    Write-Host "==> Running the same real-Incus substrate verification inside WSL2"
    $Run = "mkdir -p /tmp/hacocoon-runner; cd '$LinuxRepo'; " +
        "export GITHUB_ACTIONS=true HACO_CI_RUNNER_ENVIRONMENT=github-hosted RUNNER_TEMP=/tmp/hacocoon-runner; " +
        "bash -n tools/ci-incus.sh test/e2e/incus_standalone.sh; " +
        "bash tools/ci-incus.sh setup; " +
        "bash tools/ci-incus.sh standalone"
    Invoke-WslChecked @("--distribution", $Distro, "--user", "root", "--", "bash", "-lc", $Run)

    Write-Host "Windows/WSL acceptance passed: Windows runner -> WSL2 Ubuntu 26.04 -> systemd -> real Incus system container"
}
catch {
    Write-Host "::error::Windows/WSL acceptance did not complete: $($_.Exception.Message)"
    if ($Created) {
        Write-Probe "WSL failure diagnostics" {
            try { Write-Host (Get-WslText @("--list", "--verbose")) } catch { Write-Host $_ }
            & wsl.exe --distribution $Distro --user root -- sh -lc "cat /etc/os-release; uname -a; ps -p 1 -o pid,comm,args=; systemctl --no-pager --failed || true; journalctl -b --no-pager -n 200 || true" 2>&1
        }
    }
    throw
}
finally {
    if ($Created) {
        & wsl.exe --terminate $Distro 2>$null | Out-Null
        & wsl.exe --unregister $Distro 2>$null | Out-Null
    }
}

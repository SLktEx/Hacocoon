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
    return (($Text -replace "`0", "").Trim())
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

function Get-WslValue {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)
    # Store WSL can emit a host-side user-session warning on stderr even when
    # the requested root command succeeds. Get-WslText intentionally keeps
    # stderr for diagnostics, so scalar probes consume only the final non-empty
    # command-output line rather than mistaking that warning for the value.
    $text = Get-WslText $Arguments
    $lines = @($text -split "`r?`n" | ForEach-Object { $_.Trim() } | Where-Object { $_ })
    if ($lines.Count -eq 0) {
        return ""
    }
    return $lines[-1]
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

    # Current Store WSL prints valid help but may return -1 on Windows Server
    # hosted runners. Treat the help text itself as the capability contract.
    $help = Normalize-WslText ((& wsl.exe --help 2>&1 | Out-String))
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

    # Fresh installs inherit the runner's default WSL version. Assert that the
    # created distro really is WSL2 instead of running --set-version again:
    # Store WSL on Windows Server 2025 may return WSL_E_VM_MODE_INVALID_STATE
    # when asked to convert a distro that is already stopped on version 2.
    $installed = Get-WslText @("--list", "--verbose")
    Write-Host $installed
    $distroLine = @($installed -split "`r?`n" | Where-Object { $_ -match "(?:^|\s)$([regex]::Escape($Distro))\s+" }) | Select-Object -First 1
    if (-not $distroLine -or $distroLine -notmatch '\s2\s*$') {
        throw "fresh named distro is not WSL2: $distroLine"
    }

    Write-Host "==> Initializing distro non-interactively for CI"
    Invoke-WslChecked @(
        "--distribution", $Distro,
        "--user", "root",
        "--", "sh", "-lc",
        "printf '[boot]\\nsystemd=true\\n[user]\\ndefault=root\\n' > /etc/wsl.conf"
    )
    Invoke-WslChecked @("--terminate", $Distro)
    Start-Sleep -Seconds 2

    # Avoid a shell wrapper for scalar probes. Besides being simpler, this
    # prevents Windows/WSL argument quoting from eating Linux '$VAR' expansion.
    $pid1 = Get-WslValue @("--distribution", $Distro, "--user", "root", "--", "ps", "-p", "1", "-o", "comm=")
    if ($pid1 -ne "systemd") {
        throw "systemd is not PID 1 inside the GitHub-hosted WSL2 distro (got '$pid1')"
    }

    $osRelease = Get-WslText @("--distribution", $Distro, "--user", "root", "--", "cat", "/etc/os-release")
    $versionMatch = [regex]::Match($osRelease, '(?m)^VERSION_ID="?([^"\r\n]+)"?\s*$')
    if (-not $versionMatch.Success) {
        throw "could not determine Ubuntu version inside WSL: $osRelease"
    }
    $version = $versionMatch.Groups[1].Value
    if ($version -ne "26.04") {
        throw "expected Ubuntu 26.04 inside WSL, got '$version'"
    }

    # GitHub-hosted Windows runners use WSL's default DrvFS automount. Avoid
    # passing a backslash-heavy Windows path through a Linux command argument:
    # wslpath can receive D:\a\repo as D:arepo at this boundary. Build the
    # canonical /mnt/<drive>/... path on the Windows side, then assert it exists.
    $repoRoot = [System.IO.Path]::GetPathRoot($Repo)
    if ($repoRoot -notmatch '^([A-Za-z]):\\$') {
        throw "expected the GitHub checkout on a drive-letter path, got '$Repo'"
    }
    $drive = $Matches[1].ToLowerInvariant()
    $relativeRepo = $Repo.Substring($repoRoot.Length).Replace('\', '/')
    $LinuxRepo = "/mnt/$drive/$relativeRepo"
    Invoke-WslChecked @("--distribution", $Distro, "--user", "root", "--", "test", "-d", $LinuxRepo)
    Write-Host "WSL checkout path: $LinuxRepo"

    # wsl.exe does not inherit arbitrary Windows environment variables into the
    # Linux process. Pass the numeric GitHub run identity explicitly so the
    # existing Incus helper derives the same safe per-run resource prefix it
    # uses on native Linux runners.
    $RunId = $env:GITHUB_RUN_ID
    $RunAttempt = $env:GITHUB_RUN_ATTEMPT
    if ($RunId -notmatch '^\d+$' -or $RunAttempt -notmatch '^\d+$') {
        throw "GitHub Actions did not provide a numeric run identity (run_id='$RunId', attempt='$RunAttempt')"
    }

    Write-Host "==> Running the same real-Incus substrate verification inside WSL2"
    $Run = "mkdir -p /tmp/hacocoon-runner; cd '$LinuxRepo'; " +
        "export GITHUB_ACTIONS=true HACO_CI_RUNNER_ENVIRONMENT=github-hosted RUNNER_TEMP=/tmp/hacocoon-runner " +
        "GITHUB_RUN_ID=$RunId GITHUB_RUN_ATTEMPT=$RunAttempt; " +
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

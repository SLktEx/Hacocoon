[CmdletBinding()]
param(
    [string]$InstanceName = "Hacocoon",
    [string]$BaseDistro = "Ubuntu-26.04",
    [string]$HacocoonVersion = "latest",
    [switch]$WebDownload,
    [switch]$UseCachedWslImage,
    [switch]$SkipIncus,
    [switch]$GrantIncusAdmin,
    [switch]$InteractiveUserSetup
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$LoginShell = "/usr/local/libexec/hacocoon-login"
$ManagedLoginUser = "hacocoon"

function Write-Step([string]$Message) {
    Write-Host "==> $Message"
}

function Invoke-WslInstall([string]$Name, [string[]]$Arguments) {
    $systemWsl = Join-Path ([Environment]::SystemDirectory) "wsl.exe"
    if (-not (Test-Path -LiteralPath $systemWsl -PathType Leaf)) {
        throw "The system wsl.exe is unavailable at '$systemWsl'."
    }
    # WSL owns any prerequisite elevation and parent-console attachment. Do not
    # elevate distro registration ourselves or lose diagnostics in a new window.
    $previousPreference = $ErrorActionPreference
    $createExitCode = 1
    try {
        # PS5.1 can turn native stderr into an ErrorRecord. Preserve the display
        # and wait for the exit status instead of aborting at the first message.
        $ErrorActionPreference = "Continue"
        $global:LASTEXITCODE = 1
        & $systemWsl @Arguments
        $createExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousPreference
    }
    if ($createExitCode -ne 0) {
        throw "WSL failed to create '$Name' (exit code $createExitCode). The installer stopped before common Ubuntu setup."
    }
}

function Assert-SafeName([string]$Value, [string]$Label) {
    if ($Value -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$') {
        throw "$Label '$Value' contains unsupported characters."
    }
}

function Assert-ReleaseTag([string]$Version) {
    if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$') {
        throw "Invalid Hacocoon version '$Version'."
    }
}

function New-WslCaptureResult([int]$ExitCode, [object[]]$Stdout, [object]$Stderr) {
    $stdoutText = if ($null -eq $Stdout -or $Stdout.Count -eq 0) { "" } else { ($Stdout -join [Environment]::NewLine).Trim() }
    $stderrText = if ($null -eq $Stderr) { "" } else { ([string]$Stderr).Trim() }
    return [pscustomobject]@{
        ExitCode = $ExitCode
        Stdout = $stdoutText
        Stderr = $stderrText
    }
}

function Invoke-WslCapture([string[]]$Arguments) {
    $nativePassing = Get-Variable PSNativeCommandArgumentPassing -ErrorAction SilentlyContinue
    if ($null -eq $nativePassing -or $nativePassing.Value -eq 'Legacy') {
        # PS 5.1 strips embedded quotes unless escaped for the native binder.
        $Arguments = @($Arguments | ForEach-Object { $_ -replace '(\\*)"', '$1$1\"' })
    }
    $stderrPath = [IO.Path]::GetTempFileName()
    $previousPreference = $ErrorActionPreference
    $stdout = @()
    $stderr = ""
    $exitCode = 1
    try {
        # Windows PowerShell can promote native stderr to an ErrorRecord while
        # $ErrorActionPreference is Stop. WSL may emit advisory systemd/session
        # warnings even when the requested command succeeds, so let the native
        # process finish and make the decision from its exit code.
        $ErrorActionPreference = "Continue"
        $stdout = @(& wsl.exe @Arguments 2> $stderrPath)
        $exitCode = $LASTEXITCODE
        if (Test-Path -LiteralPath $stderrPath) {
            $stderr = Get-Content -LiteralPath $stderrPath -Raw -ErrorAction SilentlyContinue
        }
    } finally {
        $ErrorActionPreference = $previousPreference
        Remove-Item -LiteralPath $stderrPath -Force -ErrorAction SilentlyContinue
    }
    return New-WslCaptureResult $exitCode $stdout $stderr
}

function Invoke-WslRootShellScript([string]$Name, [string]$Script, [string[]]$ScriptArguments = @()) {
    # Shell source is control data, not stdin payload. Encode it into an argv-safe
    # base64 string, materialize it inside WSL, and execute the temporary file.
    # This avoids Windows PowerShell native-pipeline encoding/BOM behavior and
    # also avoids sh -s early-exit interactions with the stdin producer.
    $normalized = ($Script -replace "`r`n", "`n") -replace "`r", "`n"
    $encoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($normalized))
    $runner = 'tmp="$(mktemp)"; trap ''rm -f "$tmp"'' EXIT; printf ''%s'' "$1" | base64 -d > "$tmp"; shift; sh -eu "$tmp" "$@"'
    $arguments = @(
        "--distribution", $Name,
        "--user", "root",
        "--exec", "sh", "-eu", "-c", $runner, "sh", $encoded
    ) + @($ScriptArguments)
    return Invoke-WslCapture $arguments
}

function Write-WslUtf8File([string]$Name, [string]$Path, [string]$Content, [switch]$Append) {
    # Never send installer-controlled bytes through the Windows native stdin
    # pipeline. Windows PowerShell 5.1 can change encoding/preambles there.
    # Base64 is argv-safe and decoded entirely inside WSL.
    $normalized = ($Content -replace "`r`n", "`n") -replace "`r", "`n"
    $encoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($normalized))
    $script = if ($Append) {
        'printf ''%s'' "$1" | base64 -d >> "$2"'
    } else {
        'printf ''%s'' "$1" | base64 -d > "$2"'
    }
    return Invoke-WslCapture @(
        "--distribution", $Name,
        "--user", "root",
        "--exec", "sh", "-eu", "-c", $script, "sh", $encoded, $Path
    )
}

function Get-InstalledDistros {
    $lines = & wsl.exe --list --quiet 2>$null
    if ($LASTEXITCODE -ne 0) { return @() }
    return @($lines | ForEach-Object { ($_ -replace "`0", "").Trim() } | Where-Object { $_ })
}

function Get-WslGeneration([string]$Name) {
    $escaped = [regex]::Escape($Name)
    for ($attempt = 0; $attempt -lt 20; $attempt++) {
        $lines = & wsl.exe --list --verbose 2>$null
        if ($LASTEXITCODE -eq 0) {
            foreach ($line in $lines) {
                $normalized = ($line -replace "`0", "").Trim()
                if ($normalized -match "^\*?\s*$escaped\s+.*\s+([12])\s*$") {
                    return [int]$Matches[1]
                }
            }
        }
        Start-Sleep -Milliseconds 250
    }
    throw "Unable to determine WSL generation for '$Name'."
}

function Ensure-Wsl2([string]$Name) {
    if ((Get-WslGeneration $Name) -eq 2) { return }
    Write-Step "Converting '$Name' to WSL 2"
    & wsl.exe --set-version $Name 2
    if ($LASTEXITCODE -ne 0 -or (Get-WslGeneration $Name) -ne 2) {
        throw "Failed to convert '$Name' to WSL 2."
    }
}

function Assert-WslSupported {
    $null = (& wsl.exe --version 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0) {
        throw "This WSL installation is too old. Update WSL with 'wsl --update'."
    }
}

function Get-Sha256Hex([string]$Path) {
    $stream = [IO.File]::OpenRead($Path)
    try {
        $sha256 = [Security.Cryptography.SHA256]::Create()
        try {
            $hash = $sha256.ComputeHash($stream)
        } finally {
            $sha256.Dispose()
        }
    } finally {
        $stream.Dispose()
    }
    return ([BitConverter]::ToString($hash) -replace '-', '').ToLowerInvariant()
}

function Get-CachedUbuntuWslImage {
    # PS5.1 renders progress for every small download chunk, which can dominate
    # large-image transfers. This preference is local to the product function;
    # callers retain their progress settings, including when a download fails.
    $ProgressPreference = 'SilentlyContinue'
    $cachePath = Join-Path $PSScriptRoot "ubuntu.wsl"
    if (Test-Path -LiteralPath $cachePath -PathType Leaf) {
        Write-Step "Using cached Ubuntu WSL image '$cachePath'"
        return (Resolve-Path -LiteralPath $cachePath).Path
    }

    Write-Step "Cached Ubuntu WSL image not found; downloading Ubuntu-26.04"
    $distributionInfoUrl = "https://raw.githubusercontent.com/microsoft/WSL/master/distributions/DistributionInfo.json"
    try {
        $distributionInfo = Invoke-RestMethod -Uri $distributionInfoUrl -UseBasicParsing
    } catch {
        throw "Failed to load WSL distribution metadata from '$distributionInfoUrl': $($_.Exception.Message)"
    }

    $ubuntu = @($distributionInfo.ModernDistributions.Ubuntu | Where-Object { $_.Name -eq "Ubuntu-26.04" }) | Select-Object -First 1
    if ($null -eq $ubuntu) {
        throw "Ubuntu-26.04 is not present in Microsoft's WSL distribution metadata."
    }

    $osArch = [Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    $asset = switch ($osArch) {
        "X64" { $ubuntu.Amd64Url }
        "Arm64" { $ubuntu.Arm64Url }
        default { throw "Unsupported Windows architecture '$osArch' for cached Ubuntu WSL image download." }
    }
    if ($null -eq $asset -or [string]::IsNullOrWhiteSpace([string]$asset.Url) -or [string]::IsNullOrWhiteSpace([string]$asset.Sha256)) {
        throw "Ubuntu-26.04 WSL metadata does not contain a download URL and SHA256 for '$osArch'."
    }

    $expectedSha256 = ([string]$asset.Sha256).Trim().ToLowerInvariant() -replace '^0x', ''
    $temporaryPath = "$cachePath.download"
    Remove-Item -LiteralPath $temporaryPath -Force -ErrorAction SilentlyContinue
    try {
        Write-Step "Downloading Ubuntu-26.04 WSL image to '$cachePath'"
        Invoke-WebRequest -Uri ([string]$asset.Url) -OutFile $temporaryPath -UseBasicParsing
        $actualSha256 = Get-Sha256Hex $temporaryPath
        if ($actualSha256 -ne $expectedSha256) {
            throw "Downloaded Ubuntu-26.04 WSL image SHA256 mismatch (expected $expectedSha256, got $actualSha256)."
        }
        Move-Item -LiteralPath $temporaryPath -Destination $cachePath -Force
    } catch {
        Remove-Item -LiteralPath $temporaryPath -Force -ErrorAction SilentlyContinue
        throw
    }

    return (Resolve-Path -LiteralPath $cachePath).Path
}

function Get-WslDefaultUser([string]$Name) {
    $probe = Invoke-WslCapture @("--distribution", $Name, "--exec", "id", "-un")
    if ($probe.ExitCode -ne 0 -or [string]::IsNullOrWhiteSpace($probe.Stdout)) {
        throw "Unable to determine the default Linux user in '$Name'."
    }
    $candidate = $probe.Stdout
    Assert-LoginUserName $candidate
    return $candidate
}

function Get-WslLoginUser([string]$Name) {
    $candidate = Get-WslDefaultUser $Name
    Assert-LoginUserName $candidate
    if ($candidate -eq "root") {
        throw "The dedicated WSL instance still defaults to root. Re-run with -InteractiveUserSetup or let the installer create the managed '$ManagedLoginUser' user."
    }
    # A native WSL failure is not proof that the already-resolved user is
    # absent. Retry only this read-only lookup, never account/setup mutations.
    for ($attempt = 0; $attempt -lt 3; $attempt++) {
        $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "id", $candidate)
        if ($probe.ExitCode -eq 0) { return $candidate }
        if ($attempt -lt 2) { Start-Sleep -Milliseconds 250 }
    }
    throw "Unable to verify WSL login user '$candidate' in '$Name' after 3 read-only probes (WSL exit code $($probe.ExitCode)). A failed WSL invocation does not establish that the account is absent."
}

function Assert-LoginUserName([string]$Value) {
    if ($Value -notmatch '^[A-Za-z_][A-Za-z0-9_.-]{0,63}$') {
        throw "WSL login user contains unsupported characters."
    }
}

function Set-WslDefaultUser([string]$Name, [string]$LoginUser) {
    $script = @'
set -eu
login_user="$1"
tmp="$(mktemp)"
if [ -f /etc/wsl.conf ]; then
  awk -v login_user="$login_user" '
    BEGIN { in_user=0; user_seen=0; default_seen=0 }
    function flush_user() {
      if (in_user && !default_seen) {
        print "default=" login_user
        default_seen=1
      }
    }
    /^[[:space:]]*\[[^]]+\][[:space:]]*$/ {
      flush_user()
      in_user = ($0 ~ /^[[:space:]]*\[user\][[:space:]]*$/)
      if (in_user) {
        user_seen=1
        default_seen=0
      }
      print
      next
    }
    {
      if (in_user && $0 ~ /^[[:space:]]*default[[:space:]]*=/) {
        if (!default_seen) {
          print "default=" login_user
          default_seen=1
        }
        next
      }
      print
    }
    END {
      flush_user()
      if (!user_seen) {
        if (NR > 0) print ""
        print "[user]"
        print "default=" login_user
      }
    }
  ' /etc/wsl.conf > "$tmp"
else
  printf '[user]\ndefault=%s\n' "$login_user" > "$tmp"
fi
install -o root -g root -m 0644 "$tmp" /etc/wsl.conf
rm -f "$tmp"
'@
    $probe = Invoke-WslRootShellScript $Name $script @($LoginUser)
    if ($probe.ExitCode -ne 0) {
        throw "Failed to configure '$LoginUser' as the default WSL user: $($probe.Stderr)"
    }
}

function Ensure-ManagedWslLoginUser([string]$Name) {
    Assert-SafeName $ManagedLoginUser "managed WSL login user"
    Write-Step "Preparing managed WSL login user '$ManagedLoginUser'"

    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "id", "-u", $ManagedLoginUser)
    if ($probe.ExitCode -ne 0) {
        $probe = Invoke-WslCapture @(
            "--distribution", $Name,
            "--user", "root",
            "--exec", "/usr/sbin/useradd", "--create-home", "--shell", "/bin/bash", $ManagedLoginUser
        )
        if ($probe.ExitCode -ne 0) {
            throw "Failed to create managed WSL login user '$ManagedLoginUser': $($probe.Stderr)"
        }
    }

    # useradd creates a locked-password account by default. Never reset an
    # existing account's password on retry; normal WSL entry needs no password.
    $probe = Invoke-WslRootShellScript $Name '[ "$(id -u -- "$1")" -gt 0 ] && [ "$(id -g -- "$1")" -gt 0 ]' @($ManagedLoginUser)
    if ($probe.ExitCode -ne 0) { throw "Managed login user must have non-root UID and GID." }

    Set-WslDefaultUser $Name $ManagedLoginUser
    & wsl.exe --terminate $Name | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Failed to restart '$Name' after configuring the managed login user." }
    Start-Sleep -Milliseconds 750

    $actual = Get-WslDefaultUser $Name
    if ($actual -ne $ManagedLoginUser) {
        throw "Managed WSL default user setup failed: expected '$ManagedLoginUser', got '$actual'."
    }
    return $ManagedLoginUser
}

function Configure-ManagedWslOobe([string]$Name, [string]$LoginUser) {
    # The default managed-account path owns account preparation. Do not leave
    # Ubuntu account creation/metrics prompts pending at normal WSL entry.
    # InteractiveUserSetup retains Ubuntu's original OOBE unchanged.
    $script = @'
set -eu
uid="$(id -u -- "$1")"
[ "$uid" -gt 0 ]
config=/etc/wsl-distribution.conf
[ -f "$config" ] && [ ! -L "$config" ]
[ "$(stat -Lc '%u' "$config")" = 0 ]
[ -z "$(find "$config" -perm /022 -print -quit)" ]
tmp="$(mktemp /etc/.hacocoon-oobe.XXXXXX)"
trap 'rm -f "$tmp"' EXIT
# BEGIN MANAGED OOBE TRANSFORM
awk -v uid="$uid" '
  /^[[:space:]]*\[[^]]+\][[:space:]]*$/ {
    in_oobe = ($0 ~ /^[[:space:]]*\[oobe\][[:space:]]*$/)
    if (in_oobe) sections++
    print; next
  }
  in_oobe && /^[[:space:]]*command[[:space:]]*=/ {
    value=$0; sub(/^[^=]*=[[:space:]]*/, "", value); sub(/[[:space:]]*$/, "", value)
    if (value != "" && value != "/usr/lib/wsl/wsl-setup") bad=1
    commands++; print "command="; next
  }
  in_oobe && /^[[:space:]]*defaultUid[[:space:]]*=/ {
    ids++; print "defaultUid=" uid; next
  }
  { print }
  END { if (bad || sections != 1 || commands != 1 || ids != 1) exit 1 }
' "$config" > "$tmp"
# END MANAGED OOBE TRANSFORM
chown root:root "$tmp"
chmod 0644 "$tmp"
mv -fT "$tmp" "$config"
'@
    $probe = Invoke-WslRootShellScript $Name $script @($LoginUser)
    if ($probe.ExitCode -ne 0) {
        throw "Managed WSL first-launch configuration failed; existing distribution configuration was preserved. Use -InteractiveUserSetup for a custom Ubuntu OOBE."
    }
}

function Complete-InteractiveWslUserSetup([string]$Name) {
    Write-Step "Launching interactive Ubuntu user setup in '$Name'"
    Write-Host "Create the Ubuntu user you want to use, then exit the WSL shell to continue installation."
    & wsl.exe --distribution $Name | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "Interactive Ubuntu user setup in '$Name' failed."
    }
    $candidate = Get-WslDefaultUser $Name
    if ($candidate -eq "root") {
        throw "Interactive Ubuntu user setup did not configure a normal default user."
    }
    return Get-WslLoginUser $Name
}

function Initialize-WslLoginUser([string]$Name, [bool]$Created) {
    $defaultUser = Get-WslDefaultUser $Name
    if ($InteractiveUserSetup) {
        if ($defaultUser -eq "root") {
            $loginUser = Complete-InteractiveWslUserSetup $Name
        } else {
            $loginUser = Get-WslLoginUser $Name
        }
    } elseif ($Created -or $defaultUser -eq "root") {
        $loginUser = Ensure-ManagedWslLoginUser $Name
    } else {
        # Preserve already-configured non-root users when upgrading older installs.
        $loginUser = Get-WslLoginUser $Name
    }
    if (-not $InteractiveUserSetup -and $loginUser -eq $ManagedLoginUser) {
        Configure-ManagedWslOobe $Name $loginUser
    }
    return $loginUser
}

function Get-OsReleaseValue([string]$Content, [string]$Key) {
    foreach ($line in ($Content -split "`r?`n")) {
        if ($line -match ('^' + [regex]::Escape($Key) + '=(.*)$')) {
            $value = $Matches[1].Trim()
            if ($value.Length -ge 2 -and (($value[0] -eq '"' -and $value[$value.Length - 1] -eq '"') -or ($value[0] -eq "'" -and $value[$value.Length - 1] -eq "'"))) {
                $value = $value.Substring(1, $value.Length - 2)
            }
            return $value
        }
    }
    return $null
}

function Assert-UbuntuBaseline([string]$Name) {
    $probe = Invoke-WslCapture @("--distribution", $Name, "--exec", "cat", "/etc/os-release")
    if ($probe.ExitCode -ne 0) {
        throw "Unable to read /etc/os-release from the dedicated WSL instance."
    }
    $id = Get-OsReleaseValue $probe.Stdout "ID"
    $versionId = Get-OsReleaseValue $probe.Stdout "VERSION_ID"
    if ($id -ne "ubuntu" -or [string]::IsNullOrWhiteSpace($versionId)) {
        throw "The dedicated WSL instance must be Ubuntu 26.04 or newer (got ID='$id' VERSION_ID='$versionId')."
    }
    try { $version = [version]$versionId } catch { throw "Unable to parse Ubuntu version '$versionId'." }
    if ($version -lt [version]'26.4') {
        throw "Hacocoon requires Ubuntu 26.04 or newer (got $version)."
    }
}

function Assert-SystemdActive([string]$Name) {
    $probe = Invoke-WslCapture @("--distribution", $Name, "--exec", "ps", "-p", "1", "-o", "comm=")
    $pid1 = $probe.Stdout
    if ($probe.ExitCode -ne 0 -or $pid1 -ne "systemd") {
        throw "systemd is not active as PID 1 inside '$Name'."
    }
}

function Enable-WslSystemd([string]$Name) {
    $probe = Invoke-WslCapture @("--distribution", $Name, "--exec", "ps", "-p", "1", "-o", "comm=")
    if ($probe.ExitCode -eq 0 -and $probe.Stdout -eq "systemd") { return }

    Write-Step "Enabling systemd in '$Name'"
    $script = @'
set -eu
tmp="$(mktemp)"
if [ -f /etc/wsl.conf ]; then
  awk '
    BEGIN { in_boot=0; boot_seen=0; systemd_seen=0 }
    function flush_boot() { if (in_boot && !systemd_seen) { print "systemd=true"; systemd_seen=1 } }
    /^[[:space:]]*\[[^]]+\][[:space:]]*$/ {
      flush_boot()
      in_boot = ($0 ~ /^[[:space:]]*\[boot\][[:space:]]*$/)
      if (in_boot) { boot_seen=1; systemd_seen=0 }
      print
      next
    }
    {
      if (in_boot && $0 ~ /^[[:space:]]*systemd[[:space:]]*=/) {
        if (!systemd_seen) { print "systemd=true"; systemd_seen=1 }
        next
      }
      print
    }
    END {
      flush_boot()
      if (!boot_seen) {
        if (NR > 0) print ""
        print "[boot]"
        print "systemd=true"
      }
    }
  ' /etc/wsl.conf > "$tmp"
else
  printf '[boot]\nsystemd=true\n' > "$tmp"
fi
install -o root -g root -m 0644 "$tmp" /etc/wsl.conf
rm -f "$tmp"
'@
    $probe = Invoke-WslRootShellScript $Name $script
    if ($probe.ExitCode -ne 0) { throw "Failed to configure systemd in '$Name'." }

    & wsl.exe --terminate $Name
    if ($LASTEXITCODE -ne 0) { throw "Failed to restart '$Name' after enabling systemd." }
    Start-Sleep -Milliseconds 750
    $probe = Invoke-WslCapture @("--distribution", $Name, "--exec", "true")
    if ($probe.ExitCode -ne 0) { throw "Failed to start '$Name' after enabling systemd." }
    Assert-SystemdActive $Name
}

function Get-BundledVersion {
    $versionPath = Join-Path $PSScriptRoot "VERSION"
    if (-not (Test-Path -LiteralPath $versionPath -PathType Leaf)) {
        if ($HacocoonVersion -eq "latest") { return "latest" }
        Assert-ReleaseTag $HacocoonVersion
        return $HacocoonVersion
    }

    $bundled = (Get-Content -LiteralPath $versionPath -Raw).Trim()
    Assert-ReleaseTag $bundled
    if ($HacocoonVersion -ne "latest" -and $HacocoonVersion -ne $bundled) {
        throw "This installer contains $bundled but '$HacocoonVersion' was requested. Download the matching installer package instead."
    }
    return $bundled
}

function Get-WslArch([string]$Name) {
    $probe = Invoke-WslCapture @("--distribution", $Name, "--exec", "uname", "-m")
    if ($probe.ExitCode -ne 0) {
        throw "Unable to determine WSL architecture for '$Name'."
    }
    $machine = $probe.Stdout
    switch ($machine) {
        { $_ -in @("x86_64", "amd64") } { return "amd64" }
        { $_ -in @("aarch64", "arm64") } { return "arm64" }
        default { throw "Unsupported WSL architecture '$machine'." }
    }
}

function Configure-WslPost([string]$Name, [string]$LoginUser) {
    Write-Step "Configuring WSL login to enter trusted haco-host"
    $haco = $null
    foreach ($candidate in @('/usr/local/bin/haco', '/usr/bin/haco')) {
        $probe = Invoke-WslCapture @("--distribution", $Name, "--user", $LoginUser, "--exec", "test", "-x", $candidate)
        if ($probe.ExitCode -eq 0) {
            $haco = $candidate
            break
        }
    }
    if ([string]::IsNullOrWhiteSpace($haco)) {
        throw "Installed system haco binary is unavailable for WSL post-install setup."
    }

    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "stat", "-Lc", "%u", $haco)
    if ($probe.ExitCode -ne 0 -or $probe.Stdout -ne '0') {
        throw "Refusing WSL login integration through a non-root-owned haco binary."
    }
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "find", $haco, "-perm", "/022", "-print", "-quit")
    if ($probe.ExitCode -ne 0 -or -not [string]::IsNullOrWhiteSpace($probe.Stdout)) {
        throw "Refusing WSL login integration through a group/world-writable haco binary."
    }

    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "mkdir", "-p", "/usr/local/libexec")
    if ($probe.ExitCode -ne 0) { throw "Failed to prepare WSL login integration directory." }
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "ln", "-sfn", $haco, $LoginShell)
    if ($probe.ExitCode -ne 0) { throw "Failed to install Hacocoon WSL login shell." }

    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "grep", "-Fx", $LoginShell, "/etc/shells")
    if ($probe.ExitCode -eq 1) {
        $probe = Write-WslUtf8File $Name "/etc/shells" ($LoginShell + "`n") -Append
    }
    if ($probe.ExitCode -ne 0) { throw "Failed to register Hacocoon WSL login shell: $($probe.Stderr)" }

    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "/usr/sbin/usermod", "-s", $LoginShell, $LoginUser)
    if ($probe.ExitCode -ne 0) { throw "Failed to configure Hacocoon WSL login shell for '$LoginUser'." }

    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "getent", "passwd", $LoginUser)
    if ($probe.ExitCode -ne 0) {
        throw "Failed to inspect the WSL login user after shell configuration."
    }
    $passwdFields = $probe.Stdout -split ':'
    $actualShell = if ($passwdFields.Count -ge 7) { $passwdFields[6].Trim() } else { "" }
    if ($actualShell -ne $LoginShell) {
        throw "WSL post-install validation failed: '$LoginUser' shell is '$actualShell'."
    }
}

Assert-SafeName $InstanceName "WSL instance name"
Assert-SafeName $BaseDistro "WSL base distribution"
if ($HacocoonVersion -ne "latest") { Assert-ReleaseTag $HacocoonVersion }
if ($UseCachedWslImage -and $BaseDistro -ne "Ubuntu-26.04") {
    throw "-UseCachedWslImage currently supports only -BaseDistro Ubuntu-26.04."
}
if ($UseCachedWslImage -and $WebDownload) {
    throw "-UseCachedWslImage and -WebDownload cannot be used together."
}

if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
    throw "wsl.exe is unavailable. This installer requires Windows with current WSL."
}
Assert-WslSupported

$installed = @(Get-InstalledDistros)
$createdInstance = $false
if (-not ($installed -contains $InstanceName)) {
    if ($UseCachedWslImage) {
        $wslImagePath = Get-CachedUbuntuWslImage
        Write-Step "Creating dedicated WSL instance '$InstanceName' from cached Ubuntu-26.04 image"
        $args = @("--install", "--from-file", $wslImagePath, "--name", $InstanceName, "--no-launch")
    } else {
        Write-Step "Creating dedicated WSL instance '$InstanceName' from '$BaseDistro'"
        $args = @("--install", $BaseDistro, "--name", $InstanceName, "--no-launch")
        if ($WebDownload) { $args += "--web-download" }
    }

    Invoke-WslInstall $InstanceName $args
    $createdInstance = $true
}

Ensure-Wsl2 $InstanceName
$probe = Invoke-WslCapture @("--distribution", $InstanceName, "--exec", "true")
if ($probe.ExitCode -ne 0) {
    throw "'$InstanceName' exists but is not ready."
}

# pre
Assert-UbuntuBaseline $InstanceName
$loginUser = Initialize-WslLoginUser $InstanceName $createdInstance
Enable-WslSystemd $InstanceName
$arch = Get-WslArch $InstanceName
$archiveName = "haco_linux_$arch.tar.gz"
$archivePath = Join-Path $PSScriptRoot $archiveName
$installerPath = Join-Path $PSScriptRoot "install.sh"
$checksumsPath = Join-Path $PSScriptRoot "checksums.txt"
foreach ($required in @($installerPath, $archivePath, $checksumsPath)) {
    if (-not (Test-Path -LiteralPath $required -PathType Leaf)) {
        throw "Installer package is incomplete: missing $(Split-Path -Leaf $required). Download the $arch package."
    }
}
$resolvedVersion = Get-BundledVersion
$probe = Invoke-WslCapture @("--distribution", $InstanceName, "--user", "root", "--exec", "wslpath", "-u", "-a", $PSScriptRoot)
$linuxAssetRoot = $probe.Stdout
if ($probe.ExitCode -ne 0 -or [string]::IsNullOrWhiteSpace($linuxAssetRoot)) {
    throw "Failed to translate the installer package path into WSL."
}

# main
$skipIncusValue = if ($SkipIncus) { "1" } else { "0" }
$grantIncusAdminValue = if ($GrantIncusAdmin) { "1" } else { "0" }
$requireProvenance = if ($env:HACO_REQUIRE_PROVENANCE) { $env:HACO_REQUIRE_PROVENANCE } else { "1" }
Write-Step "Running common Ubuntu install.sh inside '$InstanceName'"
& wsl.exe --distribution $InstanceName --user root --exec env `
    "HACO_INSTALL_USER=$loginUser" `
    "HACO_BUNDLE_ROOT=$linuxAssetRoot" `
    "HACO_BOOTSTRAP_SKIP_INCUS=$skipIncusValue" `
    "HACO_BOOTSTRAP_GRANT_INCUS_ADMIN=$grantIncusAdminValue" `
    "HACO_REQUIRE_PROVENANCE=$requireProvenance" `
    sh "$linuxAssetRoot/install.sh" $resolvedVersion
if ($LASTEXITCODE -ne 0) {
    throw "Common Hacocoon Ubuntu installation failed inside WSL."
}
Assert-SystemdActive $InstanceName

# post
if (-not $SkipIncus) {
    Configure-WslPost $InstanceName $loginUser
    $probe = Invoke-WslCapture @("--distribution", $InstanceName, "--user", "root", "--exec", "incus", "exec", "haco-host", "--project", "hacocoon", "--", "/usr/local/bin/haco-host", "doctor")
    if ($probe.ExitCode -ne 0) { throw "WSL post-install haco-host acceptance failed." }
}

Write-Host ""
Write-Step "Hacocoon WSL installation complete"
Write-Host "Instance: $InstanceName"
Write-Host "Ubuntu user: $loginUser"
Write-Host "Architecture: $arch"
Write-Host "Release: $resolvedVersion"
if ($SkipIncus) {
    Write-Host "-SkipIncus was used; automatic haco-host entry was not configured."
} else {
    Write-Host "Next: wsl -d $InstanceName"
    Write-Host "Physical Host recovery: wsl -d $InstanceName -u root"
}

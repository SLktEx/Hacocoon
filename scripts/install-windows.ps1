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

$LoginShell = "/usr/local/libexec/hacocoon-login"

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
        throw "$Label '$Value' contains unsupported characters."
    }
}

function Assert-ReleaseTag([string]$Version) {
    if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$') {
        throw "Invalid Hacocoon version '$Version'."
    }
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

function Get-WslLoginUser([string]$Name) {
    $candidate = $env:HACO_BOOTSTRAP_LOGIN_USER
    if ([string]::IsNullOrWhiteSpace($candidate)) {
        $candidate = (& wsl.exe --distribution $Name --exec id -un | Out-String).Trim()
        if ($LASTEXITCODE -ne 0) {
            throw "Unable to determine the default Linux user in '$Name'."
        }
    }
    Assert-SafeName $candidate "WSL login user"
    if ($candidate -eq "root") {
        throw "The dedicated WSL instance still defaults to root. Launch it once with 'wsl -d $Name' and complete normal Ubuntu user setup."
    }
    & wsl.exe --distribution $Name --user root --exec id $candidate *> $null
    if ($LASTEXITCODE -ne 0) {
        throw "WSL login user '$candidate' does not exist in '$Name'."
    }
    return $candidate
}

function Assert-UbuntuBaseline([string]$Name) {
    $os = (& wsl.exe --distribution $Name --exec sh -c '. /etc/os-release; printf "%s|%s" "$ID" "$VERSION_ID"' | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $os -notmatch '^ubuntu\|(.+)$') {
        throw "The dedicated WSL instance must be Ubuntu 26.04 or newer (got '$os')."
    }
    try { $version = [version]$Matches[1] } catch { throw "Unable to parse Ubuntu version '$($Matches[1])'." }
    if ($version -lt [version]'26.4') {
        throw "Hacocoon requires Ubuntu 26.04 or newer (got $version)."
    }
}

function Assert-SystemdActive([string]$Name) {
    $pid1 = (& wsl.exe --distribution $Name --exec sh -c 'ps -p 1 -o comm=' | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $pid1 -ne "systemd") {
        throw "systemd is not active as PID 1 inside '$Name'."
    }
}

function Enable-WslSystemd([string]$Name) {
    $pid1 = (& wsl.exe --distribution $Name --exec sh -c 'ps -p 1 -o comm=' | Out-String).Trim()
    if ($LASTEXITCODE -eq 0 -and $pid1 -eq "systemd") { return }

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
    $script | & wsl.exe --distribution $Name --user root --exec sh -s
    if ($LASTEXITCODE -ne 0) { throw "Failed to configure systemd in '$Name'." }

    & wsl.exe --terminate $Name
    if ($LASTEXITCODE -ne 0) { throw "Failed to restart '$Name' after enabling systemd." }
    Start-Sleep -Milliseconds 750
    & wsl.exe --distribution $Name --exec true
    if ($LASTEXITCODE -ne 0) { throw "Failed to start '$Name' after enabling systemd." }
    Assert-SystemdActive $Name
}

function Configure-WslPost([string]$Name, [string]$LoginUser) {
    Write-Step "Configuring WSL login to enter trusted haco-host"
    $haco = (& wsl.exe --distribution $Name --user $LoginUser --exec sh -c 'command -v haco' | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $haco -notin @('/usr/local/bin/haco', '/usr/bin/haco')) {
        throw "Installed system haco binary is unavailable for WSL post-install setup."
    }

    $owner = (& wsl.exe --distribution $Name --user root --exec stat -Lc '%u' $haco | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $owner -ne '0') {
        throw "Refusing WSL login integration through a non-root-owned haco binary."
    }
    $writable = (& wsl.exe --distribution $Name --user root --exec find $haco -perm /022 -print -quit | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or -not [string]::IsNullOrWhiteSpace($writable)) {
        throw "Refusing WSL login integration through a group/world-writable haco binary."
    }

    & wsl.exe --distribution $Name --user root --exec mkdir -p /usr/local/libexec
    if ($LASTEXITCODE -ne 0) { throw "Failed to prepare WSL login integration directory." }
    & wsl.exe --distribution $Name --user root --exec ln -sfn $haco $LoginShell
    if ($LASTEXITCODE -ne 0) { throw "Failed to install Hacocoon WSL login shell." }
    & wsl.exe --distribution $Name --user root --exec sh -c "grep -Fx '$LoginShell' /etc/shells >/dev/null 2>&1 || printf '%s\n' '$LoginShell' >> /etc/shells"
    if ($LASTEXITCODE -ne 0) { throw "Failed to register Hacocoon WSL login shell." }

    $sudoers = "$LoginUser ALL=(root) NOPASSWD: $haco host ensure, $haco host shell`n"
    $sudoers | & wsl.exe --distribution $Name --user root --exec sh -c 'cat > /etc/sudoers.d/hacocoon-login && chmod 0440 /etc/sudoers.d/hacocoon-login && visudo -cf /etc/sudoers.d/hacocoon-login >/dev/null'
    if ($LASTEXITCODE -ne 0) { throw "Failed to install the narrow Hacocoon WSL sudo rule." }

    & wsl.exe --distribution $Name --user root --exec usermod -s $LoginShell $LoginUser
    if ($LASTEXITCODE -ne 0) { throw "Failed to configure Hacocoon WSL login shell for '$LoginUser'." }

    $actualShell = (& wsl.exe --distribution $Name --user root --exec sh -c "getent passwd '$LoginUser' | cut -d: -f7" | Out-String).Trim()
    if ($actualShell -ne $LoginShell) {
        throw "WSL post-install validation failed: '$LoginUser' shell is '$actualShell'."
    }
}

Assert-SafeName $InstanceName "WSL instance name"
Assert-SafeName $BaseDistro "WSL base distribution"
if ($BaseDistro -ne "Ubuntu-26.04") { throw "Hacocoon currently supports Ubuntu-26.04 as the WSL base distribution." }
if ($HacocoonVersion -ne "latest") { Assert-ReleaseTag $HacocoonVersion }

if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
    throw "wsl.exe is unavailable. This installer requires Windows with current WSL."
}
Assert-WslSupported

$installed = @(Get-InstalledDistros)
if (-not ($installed -contains $InstanceName)) {
    if (-not (Test-Administrator)) {
        throw "Creating the dedicated Hacocoon WSL instance requires an elevated PowerShell."
    }
    Write-Step "Creating dedicated WSL instance '$InstanceName' from '$BaseDistro'"
    $args = @("--install", $BaseDistro, "--name", $InstanceName, "--no-launch")
    if ($WebDownload) { $args += "--web-download" }
    & wsl.exe @args
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to create '$InstanceName'. Update WSL with 'wsl --update' if named installation is unsupported."
    }
    Ensure-Wsl2 $InstanceName
    Write-Host ""
    Write-Host "Dedicated Hacocoon WSL instance '$InstanceName' was created."
    Write-Host "Launch it once with: wsl -d $InstanceName"
    Write-Host "Complete normal Ubuntu user setup, then run this installer again."
    exit 0
}

Ensure-Wsl2 $InstanceName
& wsl.exe --distribution $InstanceName --exec true
if ($LASTEXITCODE -ne 0) {
    throw "'$InstanceName' exists but is not ready. Launch it once and complete Ubuntu user setup."
}

# pre
$loginUser = Get-WslLoginUser $InstanceName
Assert-UbuntuBaseline $InstanceName
Enable-WslSystemd $InstanceName
$installerPath = Join-Path $PSScriptRoot "install.sh"
if (-not (Test-Path -LiteralPath $installerPath -PathType Leaf)) {
    throw "Installer package is incomplete: install.sh must be bundled next to install-windows.ps1."
}
$linuxAssetRoot = (& wsl.exe --distribution $InstanceName --user root --exec wslpath -u -a $PSScriptRoot | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($linuxAssetRoot)) {
    throw "Failed to translate the installer package path into WSL."
}

# main: execute the shared Ubuntu installer as Physical Host root. The normal
# WSL user is used only by the environment-specific post phase below.
$skipIncusValue = if ($SkipIncus) { "1" } else { "0" }
$requireProvenance = if ($env:HACO_REQUIRE_PROVENANCE) { $env:HACO_REQUIRE_PROVENANCE } else { "1" }
Write-Step "Running common Ubuntu install.sh inside '$InstanceName'"
& wsl.exe --distribution $InstanceName --user root --exec env `
    "HACO_BOOTSTRAP_SKIP_INCUS=$skipIncusValue" `
    "HACO_BOOTSTRAP_GRANT_INCUS_ADMIN=0" `
    "HACO_REQUIRE_PROVENANCE=$requireProvenance" `
    sh "$linuxAssetRoot/install.sh" $HacocoonVersion
if ($LASTEXITCODE -ne 0) {
    throw "Common Hacocoon Ubuntu installation failed inside WSL."
}
Assert-SystemdActive $InstanceName

# post
if (-not $SkipIncus) {
    Configure-WslPost $InstanceName $loginUser
    if ($GrantIncusAdmin) {
        Write-Warning "Granting incus-admin gives '$loginUser' root-equivalent local Incus authority."
        & wsl.exe --distribution $InstanceName --user root --exec usermod -aG incus-admin $loginUser
        if ($LASTEXITCODE -ne 0) { throw "Failed to grant incus-admin to '$loginUser'." }
    }
    & wsl.exe --distribution $InstanceName --user root --exec incus exec haco-host --project hacocoon -- /usr/local/bin/haco-host doctor *> $null
    if ($LASTEXITCODE -ne 0) { throw "WSL post-install haco-host acceptance failed." }
}

Write-Host ""
Write-Step "Hacocoon WSL installation complete"
Write-Host "Instance: $InstanceName"
Write-Host "Ubuntu user: $loginUser"
Write-Host "Release: $HacocoonVersion"
if ($SkipIncus) {
    Write-Host "-SkipIncus was used; automatic haco-host entry was not configured."
} else {
    Write-Host "Next: wsl -d $InstanceName"
    Write-Host "Physical Host recovery: wsl -d $InstanceName -u root"
}

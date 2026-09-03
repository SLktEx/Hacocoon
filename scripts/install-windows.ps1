[CmdletBinding()]
param(
    [string]$InstanceName = "Hacocoon",
    [string]$BaseDistro = "Ubuntu-26.04",
    [string]$HacocoonVersion = "latest",
    [string]$LoginUser = "hacocoon",
    [switch]$InteractiveUserSetup,
    [switch]$WebDownload,
    [switch]$SkipIncus,
    [switch]$GrantIncusAdmin
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$LoginShell = "/usr/local/libexec/hacocoon-login"
$BootstrapSudoers = "/etc/sudoers.d/hacocoon-bootstrap"
$LoginUserExplicit = $PSBoundParameters.ContainsKey("LoginUser")

function Write-Step([string]$Message) {
    Write-Host "==> $Message"
}

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Invoke-ElevatedWsl([string[]]$Arguments) {
    $systemWsl = Join-Path ([Environment]::SystemDirectory) "wsl.exe"
    if (-not (Test-Path -LiteralPath $systemWsl -PathType Leaf)) {
        throw "The system wsl.exe is unavailable at '$systemWsl'."
    }
    Write-Step "Administrator approval is required only to create the dedicated Hacocoon WSL instance. Requesting UAC."
    try {
        $process = Start-Process -FilePath $systemWsl -ArgumentList $Arguments -Verb RunAs -Wait -PassThru
    } catch {
        throw "Administrator approval was cancelled or elevation could not be started. The dedicated Hacocoon WSL instance was not created."
    }
    return [int]$process.ExitCode
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
    $stderrPath = [IO.Path]::GetTempFileName()
    $previousPreference = $ErrorActionPreference
    $stdout = @()
    $stderr = ""
    $exitCode = 1
    try {
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

function Invoke-WslCaptureWithInput([string[]]$Arguments, [string]$InputText) {
    $stderrPath = [IO.Path]::GetTempFileName()
    $previousPreference = $ErrorActionPreference
    $stdout = @()
    $stderr = ""
    $exitCode = 1
    try {
        $ErrorActionPreference = "Continue"
        $stdout = @($InputText | & wsl.exe @Arguments 2> $stderrPath)
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

function Write-WslUtf8File([string]$Name, [string]$Path, [string]$Content, [switch]$Append) {
    $normalized = ($Content -replace "`r`n", "`n") -replace "`r", "`n"
    $encoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($normalized))
    $script = if ($Append) {
        'base64 -d | tee -a "$1" >/dev/null'
    } else {
        'base64 -d | tee "$1" >/dev/null'
    }
    return Invoke-WslCaptureWithInput @(
        "--distribution", $Name,
        "--user", "root",
        "--exec", "sh", "-eu", "-c", $script, "sh", $Path
    ) ($encoded + "`n")
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

function Get-CurrentWslLoginUser([string]$Name) {
    $probe = Invoke-WslCapture @("--distribution", $Name, "--exec", "id", "-un")
    if ($probe.ExitCode -ne 0 -or [string]::IsNullOrWhiteSpace($probe.Stdout)) {
        return ""
    }
    $candidate = $probe.Stdout.Trim()
    Assert-SafeName $candidate "WSL login user"
    return $candidate
}

function Ensure-AutomaticWslLoginUser([string]$Name, [string]$User) {
    Assert-SafeName $User "WSL login user"
    if ($User -eq "root") {
        throw "The automatic Hacocoon WSL login user cannot be root."
    }

    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "id", $User)
    if ($probe.ExitCode -eq 0) { return }

    Write-Step "Creating dedicated Ubuntu user '$User'"
    $script = @'
set -eu
user="$1"
if command -v adduser >/dev/null 2>&1; then
  adduser --disabled-password --gecos '' "$user"
elif command -v useradd >/dev/null 2>&1; then
  useradd --create-home --user-group --shell /bin/bash "$user"
else
  echo "neither adduser nor useradd is available" >&2
  exit 1
fi
'@
    $probe = Invoke-WslCaptureWithInput @(
        "--distribution", $Name,
        "--user", "root",
        "--exec", "sh", "-s", "--", $User
    ) $script
    if ($probe.ExitCode -ne 0) {
        throw "Failed to create dedicated Ubuntu user '$User': $($probe.Stderr)"
    }
}

function Invoke-InteractiveWslUserSetup([string]$Name) {
    $current = Get-CurrentWslLoginUser $Name
    if (-not [string]::IsNullOrWhiteSpace($current) -and $current -ne "root") {
        return $current
    }

    Write-Host ""
    Write-Step "Starting normal Ubuntu first-launch user setup"
    Write-Host "Create the Ubuntu user you want to use for Hacocoon."
    Write-Host "After setup reaches the shell, run 'exit' once so this installer can continue automatically."
    & wsl.exe --distribution $Name
    if ($LASTEXITCODE -ne 0) {
        throw "Interactive Ubuntu user setup failed for '$Name'."
    }

    $current = Get-CurrentWslLoginUser $Name
    if ([string]::IsNullOrWhiteSpace($current) -or $current -eq "root") {
        throw "Interactive Ubuntu user setup did not produce a normal non-root default user."
    }
    return $current
}

function Resolve-WslLoginUser([string]$Name, [bool]$CreatedInstance) {
    if ($InteractiveUserSetup) {
        return Invoke-InteractiveWslUserSetup $Name
    }

    if (-not $CreatedInstance -and -not $LoginUserExplicit) {
        $current = Get-CurrentWslLoginUser $Name
        if (-not [string]::IsNullOrWhiteSpace($current) -and $current -ne "root") {
            Write-Step "Reusing existing Ubuntu user '$current'"
            return $current
        }
    }

    $candidate = $LoginUser
    if (-not $LoginUserExplicit -and -not [string]::IsNullOrWhiteSpace($env:HACO_BOOTSTRAP_LOGIN_USER)) {
        $candidate = $env:HACO_BOOTSTRAP_LOGIN_USER
    }
    Assert-SafeName $candidate "WSL login user"
    Ensure-AutomaticWslLoginUser $Name $candidate
    return $candidate
}

function Set-WslDefaultUser([string]$Name, [string]$User) {
    Write-Step "Setting '$User' as the default Ubuntu user"
    $script = @'
set -eu
user="$1"
tmp="$(mktemp)"
if [ -f /etc/wsl.conf ]; then
  awk -v user="$user" '
    BEGIN { in_user=0; user_seen=0; default_seen=0 }
    function flush_user() { if (in_user && !default_seen) { print "default=" user; default_seen=1 } }
    /^[[:space:]]*\[[^]]+\][[:space:]]*$/ {
      flush_user()
      in_user = ($0 ~ /^[[:space:]]*\[user\][[:space:]]*$/)
      if (in_user) { user_seen=1; default_seen=0 }
      print
      next
    }
    {
      if (in_user && $0 ~ /^[[:space:]]*default[[:space:]]*=/) {
        if (!default_seen) { print "default=" user; default_seen=1 }
        next
      }
      print
    }
    END {
      flush_user()
      if (!user_seen) {
        if (NR > 0) print ""
        print "[user]"
        print "default=" user
      }
    }
  ' /etc/wsl.conf > "$tmp"
else
  printf '[user]\ndefault=%s\n' "$user" > "$tmp"
fi
install -o root -g root -m 0644 "$tmp" /etc/wsl.conf
rm -f "$tmp"
'@
    $probe = Invoke-WslCaptureWithInput @(
        "--distribution", $Name,
        "--user", "root",
        "--exec", "sh", "-s", "--", $User
    ) $script
    if ($probe.ExitCode -ne 0) {
        throw "Failed to set '$User' as the WSL default user: $($probe.Stderr)"
    }
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
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "cat", "/etc/os-release")
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
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "ps", "-p", "1", "-o", "comm=")
    $pid1 = $probe.Stdout
    if ($probe.ExitCode -ne 0 -or $pid1 -ne "systemd") {
        throw "systemd is not active as PID 1 inside '$Name'."
    }
}

function Enable-WslSystemd([string]$Name) {
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "ps", "-p", "1", "-o", "comm=")
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
    $probe = Invoke-WslCaptureWithInput @("--distribution", $Name, "--user", "root", "--exec", "sh", "-s") $script
    if ($probe.ExitCode -ne 0) { throw "Failed to configure systemd in '$Name'." }

    & wsl.exe --terminate $Name
    if ($LASTEXITCODE -ne 0) { throw "Failed to restart '$Name' after enabling systemd." }
    Start-Sleep -Milliseconds 750
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "true")
    if ($probe.ExitCode -ne 0) { throw "Failed to start '$Name' after enabling systemd." }
    Assert-SystemdActive $Name
}

function Remove-WslBootstrapSudo([string]$Name) {
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "rm", "-f", $BootstrapSudoers)
    if ($probe.ExitCode -ne 0) {
        Write-Warning "Failed to remove temporary Hacocoon bootstrap sudo rule."
    }
}

function Grant-WslBootstrapSudo([string]$Name, [string]$User) {
    Write-Step "Granting temporary bootstrap privileges to '$User'"
    try {
        $sudoers = "$User ALL=(root) NOPASSWD: ALL`n"
        $probe = Write-WslUtf8File $Name $BootstrapSudoers $sudoers
        if ($probe.ExitCode -ne 0) { throw "Failed to write temporary bootstrap sudo rule: $($probe.Stderr)" }
        $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "chmod", "0440", $BootstrapSudoers)
        if ($probe.ExitCode -ne 0) { throw "Failed to protect temporary bootstrap sudo rule." }
        $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "/usr/sbin/visudo", "-cf", $BootstrapSudoers)
        if ($probe.ExitCode -ne 0) { throw "Failed to validate temporary bootstrap sudo rule: $($probe.Stderr)" }
    } catch {
        Remove-WslBootstrapSudo $Name
        throw
    }
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
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "uname", "-m")
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

function Configure-WslPost([string]$Name, [string]$User) {
    Write-Step "Configuring WSL login to enter trusted haco-host"
    $haco = $null
    foreach ($candidate in @('/usr/local/bin/haco', '/usr/bin/haco')) {
        $probe = Invoke-WslCapture @("--distribution", $Name, "--user", $User, "--exec", "test", "-x", $candidate)
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

    $sudoers = "$User ALL=(root) NOPASSWD: $haco host ensure, $haco host shell`n"
    $probe = Write-WslUtf8File $Name "/etc/sudoers.d/hacocoon-login" $sudoers
    if ($probe.ExitCode -ne 0) { throw "Failed to write the narrow Hacocoon WSL sudo rule: $($probe.Stderr)" }
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "chmod", "0440", "/etc/sudoers.d/hacocoon-login")
    if ($probe.ExitCode -ne 0) { throw "Failed to protect the narrow Hacocoon WSL sudo rule: $($probe.Stderr)" }
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "/usr/sbin/visudo", "-cf", "/etc/sudoers.d/hacocoon-login")
    if ($probe.ExitCode -ne 0) { throw "Failed to validate the narrow Hacocoon WSL sudo rule: $($probe.Stderr)" }

    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "/usr/sbin/usermod", "-s", $LoginShell, $User)
    if ($probe.ExitCode -ne 0) { throw "Failed to configure Hacocoon WSL login shell for '$User'." }

    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "getent", "passwd", $User)
    if ($probe.ExitCode -ne 0) {
        throw "Failed to inspect the WSL login user after shell configuration."
    }
    $passwdFields = $probe.Stdout -split ':'
    $actualShell = if ($passwdFields.Count -ge 7) { $passwdFields[6].Trim() } else { "" }
    if ($actualShell -ne $LoginShell) {
        throw "WSL post-install validation failed: '$User' shell is '$actualShell'."
    }
}

Assert-SafeName $InstanceName "WSL instance name"
Assert-SafeName $BaseDistro "WSL base distribution"
Assert-SafeName $LoginUser "WSL login user"
if ($HacocoonVersion -ne "latest") { Assert-ReleaseTag $HacocoonVersion }

if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
    throw "wsl.exe is unavailable. This installer requires Windows with current WSL."
}
Assert-WslSupported

$installed = @(Get-InstalledDistros)
$createdInstance = $false
if (-not ($installed -contains $InstanceName)) {
    Write-Step "Creating dedicated WSL instance '$InstanceName' from '$BaseDistro'"
    $args = @("--install", $BaseDistro, "--name", $InstanceName, "--no-launch")
    if ($WebDownload) { $args += "--web-download" }

    if (Test-Administrator) {
        & wsl.exe @args
        $createExitCode = $LASTEXITCODE
    } else {
        $createExitCode = Invoke-ElevatedWsl $args
    }
    if ($createExitCode -ne 0) {
        throw "Failed to create '$InstanceName'. Update WSL with 'wsl --update' if named installation is unsupported."
    }
    $createdInstance = $true
}

Ensure-Wsl2 $InstanceName
$probe = Invoke-WslCapture @("--distribution", $InstanceName, "--user", "root", "--exec", "true")
if ($probe.ExitCode -ne 0) {
    throw "'$InstanceName' exists but is not ready for bootstrap."
}

# pre
$loginUser = Resolve-WslLoginUser $InstanceName $createdInstance
Set-WslDefaultUser $InstanceName $loginUser
Assert-UbuntuBaseline $InstanceName
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
$bootstrapSudoGranted = $false
try {
    Grant-WslBootstrapSudo $InstanceName $loginUser
    $bootstrapSudoGranted = $true
    Write-Step "Running common Ubuntu install.sh inside '$InstanceName'"
    & wsl.exe --distribution $InstanceName --user $loginUser --exec env `
        "HACO_BUNDLE_ROOT=$linuxAssetRoot" `
        "HACO_BOOTSTRAP_SKIP_INCUS=$skipIncusValue" `
        "HACO_BOOTSTRAP_GRANT_INCUS_ADMIN=$grantIncusAdminValue" `
        "HACO_REQUIRE_PROVENANCE=$requireProvenance" `
        sh "$linuxAssetRoot/install.sh" $resolvedVersion
    if ($LASTEXITCODE -ne 0) {
        throw "Common Hacocoon Ubuntu installation failed inside WSL."
    }
} finally {
    if ($bootstrapSudoGranted) {
        Remove-WslBootstrapSudo $InstanceName
    }
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

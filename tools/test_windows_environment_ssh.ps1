# Follow-on B1+B5 acceptance after the exact Windows installer journey has passed.
# The test proves a real Windows OpenSSH client can use Hacocoon's loopback-only
# SSH transport. It deliberately does not install or edit the user's SSH config.
#Requires -Version 7.0
param([string]$Distro = 'Hacocoon', [int]$Port = 22229)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$EnvironmentName = 'win-ssh-' + [guid]::NewGuid().ToString('N').Substring(0,16)
$Workspace = "/tmp/$EnvironmentName-workspace"
$Work = Join-Path ([IO.Path]::GetTempPath()) $EnvironmentName
$PrivateKey = Join-Path $Work 'id_ed25519'
$PublicKey = "$PrivateKey.pub"
$ConfigPath = Join-Path $Work 'ssh-config'
$KnownHosts = Join-Path $Work 'known_hosts'
$ConnectionId = $null
$EnvironmentCreated = $false
$WorkspaceCreated = $false

function Invoke-Captured([string]$FileName, [string[]]$Arguments) {
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $FileName
    $start.UseShellExecute = $false
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    foreach ($argument in $Arguments) {
        [void]$start.ArgumentList.Add($argument)
    }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $start
    if (-not $process.Start()) {
        throw "Cannot start $FileName."
    }
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    $process.WaitForExit()
    $stdout = $stdoutTask.GetAwaiter().GetResult()
    $stderr = $stderrTask.GetAwaiter().GetResult()
    return [pscustomobject]@{
        ExitCode = $process.ExitCode
        Stdout = $stdout
        Stderr = $stderr
    }
}

function Invoke-Checked([string]$FileName, [string[]]$Arguments, [string]$Description) {
    $result = Invoke-Captured $FileName $Arguments
    if ($result.ExitCode -ne 0) {
        $details = @($result.Stderr.Trim(), $result.Stdout.Trim()) | Where-Object { $_ }
        $detail = $details -join "`n"
        throw "$Description failed with exit $($result.ExitCode). $detail"
    }
    return $result
}

function Invoke-Wsl([string[]]$Arguments, [string]$Description) {
    return Invoke-Checked 'wsl.exe' (@('-d', $Distro) + $Arguments) $Description
}

function Invoke-HacoHost([string[]]$Arguments, [string]$Description) {
    return Invoke-Wsl (@('-u', 'root', '--exec', 'incus', 'exec', 'haco-host', '--project', 'hacocoon', '--') + $Arguments) $Description
}

[IO.Directory]::CreateDirectory($Work) | Out-Null
try {
    foreach ($tool in @('ssh.exe', 'ssh-keygen.exe')) {
        if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
            throw "$tool is unavailable on the Windows SSH client."
        }
    }

    # B1 must come from the installed product; do not repair it from source here.
    $NativeSSH = Join-Path $env:WINDIR 'System32\OpenSSH\ssh.exe'
    $NativeKeygen = Join-Path $env:WINDIR 'System32\OpenSSH\ssh-keygen.exe'
    if (-not (Test-Path -LiteralPath $NativeSSH)) { throw 'Windows standard OpenSSH is unavailable' }

    # The private key is born and remains on Windows. Only the .pub path is
    # handed to haco-host through the B1 drive projection.
    [void](Invoke-Checked $NativeKeygen @('-q', '-t', 'ed25519', '-N', '', '-f', $PrivateKey) 'Generate Windows-owned acceptance SSH key')
    $translatedPublicKey = Invoke-Wsl @('--exec', 'wslpath', '-u', '-a', $PublicKey) 'Translate Windows public-key path into WSL'
    $PublicKeyWsl = $translatedPublicKey.Stdout.Trim()
    if (-not $PublicKeyWsl.StartsWith('/mnt/')) {
        throw "Windows public key did not translate below /mnt: $PublicKeyWsl"
    }
    [void](Invoke-HacoHost @('test', '-r', $PublicKeyWsl) 'Read Windows public key from trusted haco-host through B1')

    # Create one disposable external-path Workspace/Environment through the
    # installed product CLI. The SSH preparation itself is performed from the
    # trusted haco-host below.
    [void](Invoke-Wsl @('--exec', 'mkdir', '-m', '700', $Workspace) 'Create acceptance Workspace on the WSL Physical Host')
    $WorkspaceCreated = $true
    [void](Invoke-Wsl @('--exec', 'sh', '-c', "printf 'windows-workspace-ok\n' > '$Workspace/windows-marker'") 'Seed acceptance Workspace marker')
    [void](Invoke-Wsl @('--exec', '/usr/local/bin/haco', 'env', 'create', '--workspace', $Workspace, $EnvironmentName) 'Create acceptance Environment')
    $EnvironmentCreated = $true

    $prepared = Invoke-HacoHost @('/usr/local/bin/haco', 'env', 'ssh', '--key', $PublicKeyWsl, '--port', $Port.ToString(), $EnvironmentName) 'Prepare loopback-only SSH from trusted haco-host'
    $connection = $prepared.Stdout | ConvertFrom-Json
    if ($connection.kind -ne 'ssh' -or $connection.host -ne '127.0.0.1' -or [int]$connection.port -ne $Port -or $connection.user -ne 'root') {
        throw "Unexpected prepared SSH connection metadata: $($prepared.Stdout.Trim())"
    }
    $ConnectionId = [string]$connection.id
    if ([string]::IsNullOrWhiteSpace($ConnectionId)) {
        throw 'Prepared SSH connection did not return an ID.'
    }

    $instance = Invoke-Wsl @('-u', 'root', '--exec', 'incus', 'query', "/1.0/instances/haco-$EnvironmentName`?project=hacocoon") 'Inspect actual SSH proxy binding'
    $observed = $instance.Stdout | ConvertFrom-Json
    $proxy = $observed.expanded_devices.PSObject.Properties["haco-$ConnectionId"].Value
    if ($proxy.type -ne 'proxy' -or $proxy.listen -ne "tcp:127.0.0.1:$Port" -or $proxy.connect -ne 'tcp:127.0.0.1:22') { throw 'SSH proxy is not loopback-only into Environment sshd' }
    $generated = Invoke-HacoHost @('/usr/local/bin/haco', 'env', 'ssh-config', $EnvironmentName) 'Generate OpenSSH config from trusted haco-host'
    $config = $generated.Stdout
    if ($config -notmatch "(?m)^Host haco-$([regex]::Escape($EnvironmentName))$" -or
        $config -notmatch '(?m)^  HostName 127\.0\.0\.1$' -or
        $config -notmatch "(?m)^  Port $Port$" -or
        $config -notmatch '(?m)^  User root$' -or
        $config -notmatch '(?m)^  StrictHostKeyChecking yes$') {
        throw "Generated SSH config does not describe the expected Windows loopback target.`n$config"
    }
    [IO.File]::WriteAllText($ConfigPath, $config, [Text.UTF8Encoding]::new($false))

    # Obtain the public server identity through the trusted provider, never
    # treat an unauthenticated network scan as proof of server identity.
    $trustedKey = Invoke-Wsl @('-u', 'root', '--exec', 'incus', 'exec', "haco-$EnvironmentName", '--project', 'hacocoon', '--', 'cat', '/etc/ssh/ssh_host_ed25519_key.pub') 'Read trusted provider host public key'
    $keyParts = $trustedKey.Stdout.Trim() -split '\s+'
    if ($keyParts.Length -lt 2 -or $keyParts[0] -ne 'ssh-ed25519' -or $keyParts[1] -notmatch '^[A-Za-z0-9+/=]+$') { throw 'Malformed host public key' }
    $hostKey = "[127.0.0.1]:$Port $($keyParts[0]) $($keyParts[1])"
    [IO.File]::WriteAllText($KnownHosts, $hostKey + "`n", [Text.UTF8Encoding]::new($false))

    $alias = "haco-$EnvironmentName"
    $remote = Invoke-Checked $NativeSSH @(
        '-F', $ConfigPath,
        '-i', $PrivateKey,
        '-o', "UserKnownHostsFile=$KnownHosts",
        '-o', 'BatchMode=yes',
        '-o', 'ConnectTimeout=10',
        $alias,
        'pwd && test -d /workspace && echo windows-ssh-ok && cat /workspace/windows-marker && test ! -e /init && test ! -e /var/lib/hacocoon-wsl && test ! -e /run/WSL && test ! -e /var/lib/hacocoon-control.sock && test -z "$WSL_INTEROP" && test -z "$(find /mnt -mindepth 1 -maxdepth 1 -print -quit)" && ! command -v cmd.exe'
    ) 'Connect from Windows OpenSSH to the Hacocoon Environment'
    $remoteLines = $remote.Stdout -split "`r?`n"
    if ($remoteLines -notcontains 'windows-ssh-ok' -or $remoteLines -notcontains 'windows-workspace-ok') {
        throw "Windows SSH did not execute in the expected Environment Workspace. Output: $($remote.Stdout.Trim())"
    }

    # A changed key must fail closed before any remote command is executed.
    $wrongKey = (Get-Content -Raw -LiteralPath $PublicKey).Trim() -split '\s+'
    [IO.File]::WriteAllText($KnownHosts, "[127.0.0.1]:$Port $($wrongKey[0]) $($wrongKey[1])`n", [Text.UTF8Encoding]::new($false))
    $mismatch = Invoke-Captured $NativeSSH @('-F', $ConfigPath, '-i', $PrivateKey, '-o', "UserKnownHostsFile=$KnownHosts", '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=10', $alias, 'echo MUST-NOT-EXECUTE')
    if ($mismatch.ExitCode -eq 0 -or $mismatch.Stdout.Contains('MUST-NOT-EXECUTE') -or $mismatch.Stderr -notmatch 'HOST IDENTIFICATION HAS CHANGED|Host key verification failed') { throw 'Changed host key did not fail safely' }
    Write-Host "Native client: $NativeSSH"
    Write-Host "Route: Windows 127.0.0.1:$Port -> WSL Physical Host -> Incus proxy -> haco-$EnvironmentName sshd"
    Write-Host 'Private key remained in its Windows directory; only the .pub was passed to haco-host.'
    Write-Host 'WINDOWS DIRECT ENVIRONMENT SSH: PASS'
} finally {
    if ($ConnectionId) {
        try {
            [void](Invoke-HacoHost @('/usr/local/bin/haco', 'env', 'disconnect', $EnvironmentName, $ConnectionId) 'Disconnect acceptance SSH transport')
        } catch {
            Write-Warning $_
        }
    }
    $EnvironmentGone = -not $EnvironmentCreated
    if ($EnvironmentCreated) {
        try {
            [void](Invoke-HacoHost @('/usr/local/bin/haco', 'env', 'delete', $EnvironmentName) 'Delete acceptance Environment')
            $EnvironmentGone = $true
        } catch {
            Write-Warning $_
        }
    }
    if ($WorkspaceCreated -and $EnvironmentGone) {
        try {
            [void](Invoke-Wsl @('--exec', 'rm', '-f', "$Workspace/windows-marker") 'Remove acceptance Workspace marker')
            [void](Invoke-Wsl @('--exec', 'rmdir', $Workspace) 'Remove acceptance Workspace')
        } catch {
            Write-Warning $_
        }
    }
    foreach ($path in @($KnownHosts, $ConfigPath, $PublicKey, $PrivateKey)) {
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            Remove-Item -LiteralPath $path -Force
        }
    }
    if (Test-Path -LiteralPath $Work -PathType Container) {
        # Only known test files were removed above; leave unexpected contents.
        Remove-Item -LiteralPath $Work -Force
    }
}

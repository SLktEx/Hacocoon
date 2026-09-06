# Follow-on B1+B5 acceptance after the exact Windows installer journey has passed.
# The test proves a real Windows OpenSSH client can use Hacocoon's loopback-only
# SSH transport. It deliberately does not install or edit the user's SSH config.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$Distro = 'Hacocoon'
$EnvironmentName = 'win-ssh-' + [guid]::NewGuid().ToString('N').Substring(0,16)
$Workspace = "/tmp/$EnvironmentName-workspace"
$Port = 22229
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
    foreach ($tool in @('ssh.exe', 'ssh-keygen.exe', 'ssh-keyscan.exe')) {
        if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
            throw "$tool is unavailable on the Windows SSH client."
        }
    }

    # B1: enable the documented Windows drive/WSL interop projection for the
    # exactly owned trusted haco-host. This script is an administrator action,
    # not automatic SSH client configuration.
    $setupWindowsPath = Join-Path $env:GITHUB_WORKSPACE 'scripts\setup-wsl-host-interop.py'
    $translatedSetup = Invoke-Wsl @('--exec', 'wslpath', '-u', '-a', $setupWindowsPath) 'Translate B1 setup path into WSL'
    $setupWslPath = $translatedSetup.Stdout.Trim()
    if (-not $setupWslPath.StartsWith('/')) {
        throw 'B1 setup path did not translate to an absolute WSL path.'
    }
    [void](Invoke-Wsl @('-u', 'root', '--exec', 'python3', $setupWslPath) 'Enable trusted haco-host Windows interop')

    # The private key is born and remains on Windows. Only the .pub path is
    # handed to haco-host through the B1 drive projection.
    [void](Invoke-Checked 'ssh-keygen.exe' @('-q', '-t', 'ed25519', '-N', '', '-f', $PrivateKey) 'Generate Windows-owned acceptance SSH key')
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

    # Pin the ephemeral test host key in a dedicated client-owned file. The
    # product-generated config remains StrictHostKeyChecking=yes and is not
    # merged into the Windows user's normal ~/.ssh/config.
    $hostKey = $null
    for ($attempt = 0; $attempt -lt 20 -and [string]::IsNullOrWhiteSpace($hostKey); $attempt++) {
        $scan = Invoke-Captured 'ssh-keyscan.exe' @('-T', '2', '-p', $Port.ToString(), '127.0.0.1')
        if ($scan.ExitCode -eq 0) {
            $hostKey = (($scan.Stdout -split "`r?`n") | Where-Object { $_ -and -not $_.StartsWith('#') }) -join "`n"
        }
        if ([string]::IsNullOrWhiteSpace($hostKey)) {
            Start-Sleep -Milliseconds 500
        }
    }
    if ([string]::IsNullOrWhiteSpace($hostKey)) {
        throw 'Windows ssh-keyscan could not reach the prepared WSL loopback SSH listener.'
    }
    [IO.File]::WriteAllText($KnownHosts, $hostKey + "`n", [Text.UTF8Encoding]::new($false))

    $alias = "haco-$EnvironmentName"
    $remote = Invoke-Checked 'ssh.exe' @(
        '-F', $ConfigPath,
        '-i', $PrivateKey,
        '-o', "UserKnownHostsFile=$KnownHosts",
        '-o', 'BatchMode=yes',
        '-o', 'ConnectTimeout=10',
        $alias,
        "printf 'windows-ssh-ok\n'; cat /workspace/windows-marker; test ! -e /mnt/c"
    ) 'Connect from Windows OpenSSH to the Hacocoon Environment'
    $remoteLines = $remote.Stdout -split "`r?`n"
    if ($remoteLines -notcontains 'windows-ssh-ok' -or $remoteLines -notcontains 'windows-workspace-ok') {
        throw "Windows SSH did not execute in the expected Environment Workspace. Output: $($remote.Stdout.Trim())"
    }

    Write-Host 'WINDOWS DIRECT ENVIRONMENT SSH: PASS'
} finally {
    if ($ConnectionId) {
        try {
            [void](Invoke-HacoHost @('/usr/local/bin/haco', 'env', 'disconnect', $EnvironmentName, $ConnectionId) 'Disconnect acceptance SSH transport')
        } catch {
            Write-Warning $_
        }
    }
    if ($EnvironmentCreated) {
        try {
            [void](Invoke-HacoHost @('/usr/local/bin/haco-host', 'env', 'delete', $EnvironmentName) 'Delete acceptance Environment')
        } catch {
            Write-Warning $_
        }
    }
    if ($WorkspaceCreated) {
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
        Remove-Item -LiteralPath $Work -Force -Recurse
    }
}

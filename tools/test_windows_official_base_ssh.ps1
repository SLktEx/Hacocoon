# Prove the installed official Base is SSH-ready before any Environment package
# network permission exists. This intentionally exercises the ordinary desktop
# `haco ssh setup` path before the broader Windows SSH acceptance adds test policy.
#Requires -Version 7.0
param([string]$Distro = 'Hacocoon')
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ($env:GITHUB_ACTIONS -ne 'true') {
    throw 'Official Base default-deny SSH acceptance is restricted to the disposable GHA Windows profile.'
}

$EnvironmentName = 'win-official-ssh-' + [guid]::NewGuid().ToString('N').Substring(0,12)
$Workspace = "/tmp/$EnvironmentName-workspace"
$EnvironmentAttempted = $false
$WorkspaceCreated = $false

function Invoke-Captured([string]$FileName, [string[]]$Arguments, [int]$TimeoutMilliseconds = 300000) {
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $FileName
    $start.UseShellExecute = $false
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    foreach ($argument in $Arguments) { [void]$start.ArgumentList.Add($argument) }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $start
    if (-not $process.Start()) { throw "Cannot start $FileName." }
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    if (-not $process.WaitForExit($TimeoutMilliseconds)) {
        $process.Kill($true)
        [void]$process.WaitForExit(10000)
        throw "Acceptance child timed out after ${TimeoutMilliseconds}ms"
    }
    if (-not [Threading.Tasks.Task]::WaitAll([Threading.Tasks.Task[]]@($stdoutTask, $stderrTask), 10000)) {
        throw "Acceptance child output did not close: $FileName"
    }
    $result = [pscustomobject]@{
        ExitCode = $process.ExitCode
        Stdout = $stdoutTask.GetAwaiter().GetResult()
        Stderr = $stderrTask.GetAwaiter().GetResult()
    }
    $process.Dispose()
    return $result
}

function Invoke-Checked([string]$FileName, [string[]]$Arguments, [string]$Description) {
    Write-Host "ACCEPTANCE: $Description"
    $result = Invoke-Captured $FileName $Arguments
    if ($result.ExitCode -ne 0) {
        $details = @($result.Stderr.Trim(), $result.Stdout.Trim()) | Where-Object { $_ }
        throw "$Description failed with exit $($result.ExitCode). $($details -join "`n")"
    }
    return $result
}

function Invoke-Wsl([string[]]$Arguments, [string]$Description) {
    return Invoke-Checked 'wsl.exe' (@('-d', $Distro) + $Arguments) $Description
}

function Invoke-HacoHost([string[]]$Arguments, [string]$Description) {
    return Invoke-Wsl (@('-u', 'root', '--exec', 'incus', 'exec', 'haco-host', '--project', 'hacocoon', '--') + $Arguments) $Description
}

try {
    $NativeSSH = Join-Path $env:WINDIR 'System32\OpenSSH\ssh.exe'
    if (-not (Test-Path -LiteralPath $NativeSSH -PathType Leaf)) {
        throw 'Windows standard OpenSSH is unavailable.'
    }

    $configuration = (Invoke-HacoHost @('/usr/local/bin/haco', 'config') 'Inspect installed default network Policy').Stdout | ConvertFrom-Json
    if ($configuration.policy.default -ne 'deny') {
        throw 'Fresh installed Policy is not default-deny.'
    }

    [void](Invoke-Wsl @('--exec', 'mkdir', '-m', '700', $Workspace) 'Create fresh official-Base acceptance Workspace')
    $WorkspaceCreated = $true
    [void](Invoke-Wsl @('--exec', 'sh', '-ceu', "printf 'official-base-default-deny-ok\n' > '$Workspace/official-ssh-marker'") 'Seed official-Base Workspace marker')

    $EnvironmentAttempted = $true
    [void](Invoke-Wsl @('--exec', '/usr/local/bin/haco', 'env', 'create', '--no-oci', '--workspace', $Workspace, $EnvironmentName) 'Create fresh Environment from the installed default official Base')

    # The server must already be in the immutable Base. Do this before ssh setup,
    # so a runtime package install cannot satisfy the assertion.
    [void](Invoke-Wsl @('-u', 'root', '--exec', 'incus', 'exec', "haco-$EnvironmentName", '--project', 'hacocoon', '--', 'sh', '-ceu', 'command -v sshd >/dev/null') 'Verify sshd is already present in the fresh official Base')

    [void](Invoke-HacoHost @('/usr/local/bin/haco', 'ssh', 'setup', $EnvironmentName) 'Prepare ordinary desktop SSH without package-network Policy')
    $alias = "haco-$EnvironmentName"
    $connected = Invoke-Checked $NativeSSH @('-o', 'BatchMode=yes', '-o', 'ConnectTimeout=10', $alias, 'cat /workspace/official-ssh-marker') 'Connect from Windows native OpenSSH using the generated alias'
    if ($connected.Stdout.Trim() -ne 'official-base-default-deny-ok') {
        throw 'Windows native SSH reached the wrong Workspace.'
    }

    Write-Host 'WINDOWS OFFICIAL BASE DEFAULT-DENY SSH: PASS'
} finally {
    if ($EnvironmentAttempted) {
        $deleted = Invoke-Captured 'wsl.exe' @('-d', $Distro, '--exec', '/usr/local/bin/haco', 'env', 'delete', $EnvironmentName)
        if ($deleted.ExitCode -ne 0) {
            Write-Host 'OFFICIAL BASE SSH CLEANUP: Environment delete failed'
        }
    }
    if ($WorkspaceCreated) {
        $removed = Invoke-Captured 'wsl.exe' @('-d', $Distro, '--exec', 'rm', '-rf', '--', $Workspace)
        if ($removed.ExitCode -ne 0) {
            Write-Host 'OFFICIAL BASE SSH CLEANUP: Workspace removal failed'
        }
    }
}

# Follow-on B1+B5 acceptance after the exact Windows installer journey has passed.
# The test proves a real Windows OpenSSH client can use Hacocoon's loopback-only
# SSH transport. Desktop SSH setup is also exercised on the disposable GHA user;
# local manual execution preserves the operator's SSH configuration.
#Requires -Version 7.0
param([string]$Distro = 'Hacocoon', [int]$Port = 0)
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
$EnvironmentAttempted = $false
$WorkspaceCreated = $false
$CleanupFailed = $false

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

# This is the documented administrator Policy operation, scoped to this test
# Environment. Existing rules are preserved; no network/provider repair occurs.
function Update-SSHTestPolicy([string]$Action) {
    $policyScript=@"
import json, os, pathlib, re, stat, sys, tempfile
operation, environment = sys.argv[1:]
if operation not in ('add','remove') or not re.fullmatch(r'win-ssh-[a-f0-9]{16}', environment):
    raise SystemExit('invalid test Policy scope')
p = pathlib.Path('/var/lib/hacocoon/policy.json')
if p.exists():
    metadata = p.lstat()
    if not stat.S_ISREG(metadata.st_mode) or metadata.st_uid != 0 or metadata.st_mode & 0o022:
        raise SystemExit('unsafe existing Policy file')
    data = json.loads(p.read_text())
else:
    data = {'default':'deny','rules':[]}
if not isinstance(data.get('rules'),list): raise SystemExit('invalid existing Policy')
rules = [{'capability':'network.egress','action':'connect','resource':host,
          'environment':environment,'attributes':{'protocol':protocol,'port':port},
          'decision':'allow','reason':'Windows SSH acceptance '+environment}
         for host in ('archive.ubuntu.com','security.ubuntu.com')
         for protocol,port in (('http','80'),('https','443'))]
rules += [{'capability':'network.resolve','action':'lookup','resource':'one.one.one.one',
           'environment':environment,'decision':'allow','reason':'Windows DNS acceptance '+environment}]
data['rules'] = [rule for rule in data['rules'] if rule not in rules]
if operation == 'add': data['rules'] += rules
with tempfile.NamedTemporaryFile(mode='w',dir=p.parent,delete=False) as f:
    json.dump(data,f); f.flush(); os.fsync(f.fileno()); temporary=f.name
os.replace(temporary,p)
"@
    [void](Invoke-Wsl @('-u','root','--exec','python3','-c',$policyScript,$Action,$EnvironmentName) 'Configure only this SSH test Environment package Policy')
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
    $EnvironmentAttempted = $true
    [void](Invoke-Wsl @('--exec', '/usr/local/bin/haco', 'env', 'create', '--workspace', $Workspace, $EnvironmentName) 'Create acceptance Environment')

    Update-SSHTestPolicy 'add'
    if ($env:GITHUB_ACTIONS -eq 'true') {
        $expectedDNS = @(Resolve-DnsName -Name 'one.one.one.one' -Type A -DnsOnly |
            Where-Object Type -eq 'A' | Select-Object -ExpandProperty IPAddress | Sort-Object -Unique)
        if ($expectedDNS.Count -eq 0) { throw 'Windows resolver returned no public IPv4 address' }
        $physicalDNS = Invoke-Wsl @('-u','root','--exec','getent','ahostsv4','one.one.one.one') 'Resolve through WSL platform DNS'
        $hostDNS = Invoke-HacoHost @('getent','ahostsv4','one.one.one.one') 'Resolve inside trusted Host'
        $guestDNS = Invoke-Wsl @('-u','root','--exec','incus','exec',"haco-$EnvironmentName",'--project','hacocoon','--','getent','ahostsv4','one.one.one.one') 'Resolve through automatic Environment DNS'
        foreach ($observedDNS in @($physicalDNS,$hostDNS,$guestDNS)) {
            $addresses = @($observedDNS.Stdout -split "\r?\n" | ForEach-Object { ($_ -split '\s+')[0] } |
                Where-Object { $_ -match '^\d+\.\d+\.\d+\.\d+$' } | Sort-Object -Unique)
            if (($addresses -join ',') -ne ($expectedDNS -join ',')) { throw 'Windows/WSL/Host/Environment DNS address sets differ' }
        }
        $deniedDNS = Invoke-Captured 'wsl.exe' @('-d',$Distro,'-u','root','--exec','incus','exec',"haco-$EnvironmentName",'--project','hacocoon','--','getent','ahostsv4','example.com')
        if ($deniedDNS.ExitCode -ne 2 -or -not [string]::IsNullOrWhiteSpace($deniedDNS.Stdout)) {
            throw 'Environment DNS default denial failed'
        }
        Write-Host 'WINDOWS / WSL / HOST / ENVIRONMENT GETADDRINFO AND DNS DENIAL: PASS'
        Write-Host 'SKIP: VPN/NRPT acceptance requires an available VPN and private test name.'
    }

    $sshArgs = @('/usr/local/bin/haco', 'env', 'ssh', '--key', $PublicKeyWsl)
    if ($Port -ne 0) { $sshArgs += @('--port', $Port.ToString()) }
    $sshArgs += $EnvironmentName
    $prepared = Invoke-HacoHost $sshArgs 'Prepare loopback-only SSH from trusted haco-host'
    $connection = $prepared.Stdout | ConvertFrom-Json
    if ([int]$connection.port -lt 1 -or [int]$connection.port -gt 65535) { throw 'Invalid allocated SSH port' }
    if ($Port -eq 0) { $Port = [int]$connection.port }
    if ($connection.kind -ne 'ssh' -or $connection.host -ne '127.0.0.1' -or [int]$connection.port -ne $Port -or $connection.user -ne 'root') {
        throw "Unexpected prepared SSH connection metadata: $($prepared.Stdout.Trim())"
    }
    $ConnectionId = [string]$connection.id
    if ([string]::IsNullOrWhiteSpace($ConnectionId)) {
        throw 'Prepared SSH connection did not return an ID.'
    }

    $instance = Invoke-Wsl @('-u', 'root', '--exec', 'incus', 'query', "/1.0/instances/haco-$EnvironmentName`?project=hacocoon") 'Inspect actual SSH proxy binding'
    $observed = $instance.Stdout | ConvertFrom-Json -AsHashtable
    $proxy = $observed.expanded_devices["haco-$ConnectionId"]
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
    $keyParts = ([string]$connection.host_public_key).Trim() -split '\s+'
    if ($keyParts.Length -ne 2 -or $keyParts[0] -ne 'ssh-ed25519' -or $keyParts[1] -notmatch '^[A-Za-z0-9+/=]+$') { throw 'Malformed controller-provided host public key' }
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


    # The disposable GHA Windows user exercises real desktop-home installation.
    # Local manual invocations keep the operator's SSH configuration untouched.
    if ($env:GITHUB_ACTIONS -eq 'true') {
        [void](Invoke-HacoHost @('/usr/local/bin/haco', 'ssh', 'setup', $EnvironmentName) 'Prepare ordinary desktop SSH settings')
        $managedConfig = Join-Path $env:USERPROFILE ".ssh/hacocoon/$EnvironmentName.conf"
        $managedBefore = [IO.File]::ReadAllText($managedConfig)
        $desktop = Invoke-Checked $NativeSSH @('-o', 'BatchMode=yes', '-o', 'ConnectTimeout=10', $alias, 'cat /workspace/windows-marker') 'Use generated desktop SSH alias'
        if ($desktop.Stdout.Trim() -ne 'windows-workspace-ok') { throw 'Generated SSH alias reached the wrong Workspace' }
        [void](Invoke-HacoHost @('/usr/local/bin/haco', 'env', 'stop', $EnvironmentName) 'Stop Environment before desktop reconnect')
        [void](Invoke-HacoHost @('/usr/local/bin/haco', 'ssh', 'setup', $EnvironmentName) 'Resume and reuse desktop SSH settings')
        if ([IO.File]::ReadAllText($managedConfig) -ne $managedBefore) { throw 'Reconnect unexpectedly rotated the managed SSH connection' }
        $desktop = Invoke-Checked $NativeSSH @('-o', 'BatchMode=yes', '-o', 'ConnectTimeout=10', $alias, 'cat /workspace/windows-marker') 'Reconnect using generated desktop SSH alias'
        if ($desktop.Stdout.Trim() -ne 'windows-workspace-ok') { throw 'Reconnect lost Workspace content' }
        Write-Host 'PASS: ordinary ssh setup, Windows-owned key/config, strict native SSH, stopped resume and connection reuse'
        & (Join-Path $PSScriptRoot 'test_vscode_environment.ps1') -EnvironmentName $EnvironmentName -Distro $Distro
        $projectSetupProbe = @'
set -eu
name=$1
recipe=$(mktemp /tmp/haco-project-setup-XXXXXX)
trap 'rm -f "$recipe"' EXIT
cat > "$recipe" <<'RECIPE'
set -eu
test "$PWD" = /workspace
test ! -e /init
test ! -e /var/lib/hacocoon-control.sock
count=0
if test -f .haco-setup-probe; then count=$(cat .haco-setup-probe); fi
count=$((count + 1))
printf '%s' "$count" > .haco-setup-probe
printf 'SETUP_COUNT=%s\n' "$count"
RECIPE
first=$(haco setup --script "$recipe" "$name")
printf '%s\n' "$first" | grep -qx SETUP_COUNT=1
second=$(haco setup "$name")
printf '%s\n' "$second" | grep -qx SETUP_COUNT=2
printf '%s\n' 'exit 17' > "$recipe"
if haco setup --script "$recipe" "$name"; then exit 1; fi
if haco setup "$name"; then exit 1; fi
cat > "$recipe" <<'RECIPE'
set -eu
test "$(cat /workspace/.haco-setup-probe)" = 2
rm /workspace/.haco-setup-probe
printf '%s\n' SETUP_UPDATED
RECIPE
updated=$(haco setup --script "$recipe" "$name")
printf '%s\n' "$updated" | grep -qx SETUP_UPDATED
haco setup --clear-script "$name"
haco setup "$name"
'@
        [void](Invoke-HacoHost @('/bin/bash', '-ec', $projectSetupProbe, '--', $EnvironmentName) 'Exercise saved project setup')
        Write-Host 'PROJECT SETUP SAVE / REPLAY / FAILURE / UPDATE / CLEAR: PASS'
        $previewProbe = @'
set -eu
name=$1
recipe=$(mktemp /tmp/haco-preview-XXXXXX)
trap 'rm -f "$recipe"' EXIT
cat > "$recipe" <<'RECIPE'
systemd-run --unit=haco-preview-probe --collect --service-type=exec /usr/bin/python3 -m http.server 3000 --bind 127.0.0.1 --directory /workspace
RECIPE
haco setup --script "$recipe" "$name" >/dev/null
haco setup --clear-script "$name" >/dev/null
haco open --port 3000 --no-browser "$name"
'@
        $previewResult = Invoke-HacoHost @('/bin/bash', '-ec', $previewProbe, '--', $EnvironmentName) 'Start preview through ordinary project setup'
        $previewUrl = $previewResult.Stdout.Trim()
        if ($previewUrl -notmatch '^http://127\.0\.0\.1:[0-9]{1,5}/$') { throw 'Preview returned an unsafe URL' }
        $previewResponse = Invoke-WebRequest -Uri ($previewUrl + 'windows-marker') -TimeoutSec 10
        if ($previewResponse.Content.Trim() -ne 'windows-workspace-ok') { throw 'Preview reached a different Workspace' }

        # Render through an actual browser engine using an isolated disposable profile.
        $edge = Join-Path ${env:ProgramFiles(x86)} 'Microsoft/Edge/Application/msedge.exe'
        if (Test-Path -LiteralPath $edge -PathType Leaf) {
            $browserProfile = Join-Path $Work 'preview-edge'
            $browserStart = [Diagnostics.ProcessStartInfo]::new()
            $browserStart.FileName = $edge
            $browserStart.UseShellExecute = $false
            $browserStart.CreateNoWindow = $true
            $browserStart.RedirectStandardOutput = $true
            $browserStart.RedirectStandardError = $true
            foreach ($argument in @('--headless', '--disable-gpu', '--no-first-run', '--disable-background-mode', "--user-data-dir=$browserProfile", '--dump-dom', ($previewUrl + 'windows-marker'))) {
                [void]$browserStart.ArgumentList.Add($argument)
            }
            $browserProcess = [Diagnostics.Process]::new()
            $browserProcess.StartInfo = $browserStart
            try {
                if (-not $browserProcess.Start()) { throw 'Could not start preview browser' }
                $browserOutput = $browserProcess.StandardOutput.ReadToEndAsync()
                $browserError = $browserProcess.StandardError.ReadToEndAsync()
                if (-not $browserProcess.WaitForExit(30000)) {
                    $browserProcess.Kill($true)
                    $browserProcess.WaitForExit()
                    throw 'Preview browser timed out'
                }
                $rendered = $browserOutput.GetAwaiter().GetResult()
                [void]$browserError.GetAwaiter().GetResult()
                if ($browserProcess.ExitCode -ne 0 -or $rendered -notmatch 'windows-workspace-ok') { throw 'Browser did not render the Workspace marker' }
                Write-Host 'WINDOWS EDGE HEADLESS PREVIEW RENDER: PASS'
            } finally {
                $browserProcess.Dispose()
                if (Test-Path -LiteralPath $browserProfile) {
                    $expectedBrowserProfile = [IO.Path]::GetFullPath((Join-Path $Work 'preview-edge'))
                    $actualBrowserProfile = (Resolve-Path -LiteralPath $browserProfile).Path
                    if ($actualBrowserProfile -ne $expectedBrowserProfile -or ((Get-Item -LiteralPath $browserProfile).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
                        throw 'Refusing unsafe preview browser profile cleanup'
                    }
                    Remove-Item -LiteralPath $actualBrowserProfile -Recurse -Force
                }
            }
        } else {
            Write-Host 'SKIP: Edge browser executable absent; Windows HTTP preview is tested separately'
        }
        $reusedPreview = Invoke-HacoHost @('/usr/local/bin/haco', 'open', '--port', '3000', '--no-browser', $EnvironmentName) 'Reuse preview connection'
        if ($reusedPreview.Stdout.Trim() -ne $previewUrl) { throw 'Preview did not reuse its connection' }
        [void](Invoke-HacoHost @('/usr/local/bin/haco', 'open', '--port', '3000', '--close', $EnvironmentName) 'Close preview connection')
        $previewRefused = $false
        try { [void](Invoke-WebRequest -Uri $previewUrl -TimeoutSec 3) } catch { $previewRefused = $true }
        if (-not $previewRefused) { throw 'Closed preview still accepts Windows HTTP requests' }
        Write-Host 'WINDOWS HTTP PREVIEW / REUSE / CONNECTION REFUSAL: PASS'
        $environmentDoctor = Invoke-HacoHost @('/usr/local/bin/haco', 'doctor', '--json', $EnvironmentName) 'Diagnose Environment prerequisites'
        $environmentReport = $environmentDoctor.Stdout | ConvertFrom-Json
        if ($environmentReport.environment -ne $EnvironmentName -or [string]::IsNullOrWhiteSpace($environmentReport.workspace.id)) { throw 'Environment doctor reported the wrong Workspace' }
        if (@($environmentReport.checks | Where-Object { $_.status -ne 'ok' }).Count -ne 0) { throw 'Environment doctor did not pass local prerequisite checks' }
        Write-Host 'ENVIRONMENT DOCTOR WORKSPACE / DNS / SSH PREREQUISITES: PASS'



    } else {
        Write-Host 'SKIP: automatic desktop SSH setup acceptance uses the disposable GHA Windows profile'
    }

    # A changed key must fail closed before any remote command is executed.
    $wrongKey = (Get-Content -Raw -LiteralPath $PublicKey).Trim() -split '\s+'
    [IO.File]::WriteAllText($KnownHosts, "[127.0.0.1]:$Port $($wrongKey[0]) $($wrongKey[1])`n", [Text.UTF8Encoding]::new($false))
    $mismatch = Invoke-Captured $NativeSSH @('-F', $ConfigPath, '-i', $PrivateKey, '-o', "UserKnownHostsFile=$KnownHosts", '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=10', $alias, 'echo MUST-NOT-EXECUTE')
    if ($mismatch.ExitCode -eq 0 -or $mismatch.Stdout.Contains('MUST-NOT-EXECUTE') -or $mismatch.Stderr -notmatch 'HOST IDENTIFICATION HAS CHANGED|Host key verification failed') { throw 'Changed host key did not fail safely' }
    Write-Host "Native client: $NativeSSH"
    Write-Host "Route: Windows 127.0.0.1:$Port -> WSL Physical Host -> Incus proxy -> haco-$EnvironmentName sshd"
    Write-Host 'Private key remained in its Windows directory; only the .pub was passed to haco-host.'

} finally {
    try { Update-SSHTestPolicy 'remove' } catch { $CleanupFailed = $true; Write-Warning $_ }
    if ($ConnectionId) {
        try {
            [void](Invoke-HacoHost @('/usr/local/bin/haco', 'env', 'disconnect', $EnvironmentName, $ConnectionId) 'Disconnect acceptance SSH transport')
        } catch {
            $CleanupFailed = $true; Write-Warning $_
        }
    }
    $EnvironmentGone = -not $EnvironmentAttempted
    if ($EnvironmentAttempted) {
        try {
            [void](Invoke-HacoHost @('/usr/local/bin/haco', 'env', 'delete', $EnvironmentName) 'Delete acceptance Environment')
            $EnvironmentGone = $true
        } catch {
            $CleanupFailed = $true; Write-Warning $_
        }
    }
    if ($ConnectionId -and $EnvironmentGone) {
        # Confirm actual WSL listener removal and Windows connection refusal,
        # independently of the controller cleanup response. Windows WSL
        # forwarding may report a timeout instead of ECONNREFUSED after removal.
        $listeners = Invoke-Wsl @('-u', 'root', '--exec', 'ss', '-H', '-ltn', "sport = :$Port") 'Verify Physical Host SSH listener was removed'
        $closed = Invoke-Captured $NativeSSH @('-F', $ConfigPath, '-i', $PrivateKey, '-o', "UserKnownHostsFile=$KnownHosts", '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=2', "haco-$EnvironmentName", 'echo MUST-NOT-EXECUTE')
        if (-not [string]::IsNullOrWhiteSpace($listeners.Stdout) -or $closed.ExitCode -eq 0 -or $closed.Stdout.Contains('MUST-NOT-EXECUTE')) {
            $CleanupFailed = $true
            Write-Warning 'SSH listener removal or Windows connection rejection could not be confirmed.'
        } else {
            Write-Host 'Cleanup: WSL loopback listener absent; Windows SSH connection rejected.'
        }
    }
    if ($WorkspaceCreated -and $EnvironmentGone) {
        try {
            [void](Invoke-Wsl @('--exec', 'rm', '-f', "$Workspace/windows-marker") 'Remove acceptance Workspace marker')
            [void](Invoke-Wsl @('--exec', 'rmdir', $Workspace) 'Remove acceptance Workspace')
        } catch {
            $CleanupFailed = $true; Write-Warning $_
        }
    }
    foreach ($path in @($KnownHosts, $ConfigPath, $PublicKey, $PrivateKey)) {
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            Remove-Item -LiteralPath $path -Force
        }
    }
    if (Test-Path -LiteralPath $Work -PathType Container) {
        # Only known test files were removed above; leave unexpected contents.
        [IO.Directory]::Delete($Work, $false)
    }
}

if ($CleanupFailed) { throw 'Windows SSH acceptance cleanup failed; inspect retained test resources' }
Write-Host 'WINDOWS DIRECT ENVIRONMENT SSH: PASS'

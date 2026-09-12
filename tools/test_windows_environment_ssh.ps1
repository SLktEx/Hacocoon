# Follow-on B1+B5 acceptance after the exact Windows installer journey has passed.
# The test proves a real Windows OpenSSH client can use Hacocoon's loopback-only
# SSH transport. Desktop SSH setup is also exercised on the disposable GHA user;
# local manual execution preserves the operator's SSH configuration.
#Requires -Version 7.0
param([string]$Distro = 'Hacocoon', [int]$Port = 0, [string]$ReclamationManifest)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$EnvironmentName = 'win-ssh-' + [guid]::NewGuid().ToString('N').Substring(0,16)
$Workspace = "/tmp/$EnvironmentName-workspace"
$Work = Join-Path ([IO.Path]::GetTempPath()) $EnvironmentName
$PrivateKey = Join-Path $Work 'id_ed25519'
$PublicKey = "$PrivateKey.pub"
$ConfigPath = Join-Path $Work 'ssh-config'
$KnownHosts = Join-Path $Work 'known_hosts'
$BaseDefinition = Join-Path $Work 'base.json'
$BuiltBaseName = 'win-base-' + $EnvironmentName.Substring(8)
$BuiltBaseFingerprint = $null
$ConnectionId = $null
$EnvironmentAttempted = $false
$WorkspaceCreated = $false
$CleanupFailed = $false
$DesktopFailures = [Collections.Generic.List[string]]::new()
. (Join-Path $PSScriptRoot 'windows_ssh_diagnostic.ps1')

function Invoke-Captured([string]$FileName, [string[]]$Arguments, [switch]$SSHProgress, [int]$TimeoutMilliseconds = 300000) {
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
    if (-not $process.WaitForExit($TimeoutMilliseconds)) {
        $process.Kill($true)
        [void]$process.WaitForExit(10000)
        $evidence = 'unavailable'
        if ($SSHProgress -and [Threading.Tasks.Task]::WaitAll([Threading.Tasks.Task[]]@($stdoutTask, $stderrTask), 10000)) {
            $evidence = Get-SSHProgressEvidence $stdoutTask.GetAwaiter().GetResult() $stderrTask.GetAwaiter().GetResult()
        }
        $process.Dispose()
        throw "Acceptance child timed out after ${TimeoutMilliseconds}ms; ssh_progress=$evidence"
    }
    if (-not [Threading.Tasks.Task]::WaitAll([Threading.Tasks.Task[]]@($stdoutTask, $stderrTask), 10000)) {
        $process.Dispose()
        throw "Acceptance child output did not close: $FileName"
    }
    $stdout = $stdoutTask.GetAwaiter().GetResult()
    $stderr = $stderrTask.GetAwaiter().GetResult()
    if ($SSHProgress) { $stderr = Get-SSHProgressEvidence $stdout $stderr }
    $exitCode = $process.ExitCode
    $process.Dispose()
    return [pscustomobject]@{
        ExitCode = $exitCode
        Stdout = $stdout
        Stderr = $stderr
    }
}

function Invoke-Checked([string]$FileName, [string[]]$Arguments, [string]$Description, [switch]$SSHProgress) {
    Write-Host "ACCEPTANCE: $Description"
    $result = Invoke-Captured $FileName $Arguments -SSHProgress:$SSHProgress
    if ($result.ExitCode -ne 0) {
        $details = @($result.Stderr.Trim())
        if (-not $SSHProgress) { $details += $result.Stdout.Trim() }
        $details = $details | Where-Object { $_ }
        $detail = $details -join "`n"
        $failure = [Exception]::new("$Description failed with exit $($result.ExitCode). $detail")
        $failure.Data['acceptance_exit_code'] = $result.ExitCode
        throw $failure
    }
    return $result
}

function Invoke-Wsl([string[]]$Arguments, [string]$Description) {
    return Invoke-Checked 'wsl.exe' (@('-d', $Distro) + $Arguments) $Description
}

function Invoke-HacoHost([string[]]$Arguments, [string]$Description) {
    return Invoke-Wsl (@('-u', 'root', '--exec', 'incus', 'exec', 'haco-host', '--project', 'hacocoon', '--') + $Arguments) $Description
}

# Report only fixture-owned phase and numeric process metadata, never raw guest
# output or exception messages captured by Invoke-Checked.
function Write-DesktopProbeFailure([string]$Label, [string]$Phase, [Management.Automation.ErrorRecord]$Failure) {
    $exitCode = 'unknown'
    if ($Failure.Exception.Data['acceptance_exit_code'] -is [int]) {
        $exitCode = [string]$Failure.Exception.Data['acceptance_exit_code']
    }
    Write-Host "${Label}: FAIL phase=$Phase exit=$exitCode line=$($Failure.InvocationInfo.ScriptLineNumber); continuing independent probes"
}

# This is the documented administrator Policy operation, scoped to this test
# Environment. Existing rules are preserved; no network/provider repair occurs.
function Update-SSHTestPolicy([string]$Action, [string]$TargetEnvironment = $EnvironmentName) {
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
    [void](Invoke-Wsl @('-u','root','--exec','python3','-c',$policyScript,$Action,$TargetEnvironment) 'Configure only this SSH test Environment package Policy')
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
    $definition = @{name=$BuiltBaseName; run="printf '#!/bin/sh\necho windows-base-tool-ok\n' > /usr/local/bin/haco-base-tool`nchmod 0755 /usr/local/bin/haco-base-tool`n"} | ConvertTo-Json -Compress
    [IO.File]::WriteAllText($BaseDefinition, $definition, [Text.UTF8Encoding]::new($false))
    $translatedDefinition = Invoke-Wsl @('--exec','wslpath','-u','-a',$BaseDefinition) 'Translate test Base definition'
    $built = Invoke-HacoHost @('/usr/local/bin/haco','base','build',$translatedDefinition.Stdout.Trim()) 'Build and register test Base through ordinary haco'
    $builtResult = $built.Stdout.Trim() | ConvertFrom-Json
    if ($builtResult.state -ne 'ready' -or $builtResult.base.name -ne $BuiltBaseName -or $builtResult.base.revision -notmatch '^sha256:[a-f0-9]{64}$') { throw 'Incomplete test Base build' }
    $BuiltBaseFingerprint = $builtResult.base.revision.Substring(7)
    $EnvironmentAttempted = $true
    [void](Invoke-Wsl @('--exec', '/usr/local/bin/haco', 'env', 'create', '--workspace', $Workspace, '--base', $BuiltBaseName, $EnvironmentName) 'Create acceptance Environment from built Base')

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
        '-v',
        '-F', $ConfigPath,
        '-i', $PrivateKey,
        '-o', "UserKnownHostsFile=$KnownHosts",
        '-o', 'BatchMode=yes',
        '-o', 'ConnectTimeout=10',
        $alias,
        'haco-base-tool && pwd && test -d /workspace && echo windows-ssh-ok && cat /workspace/windows-marker && test ! -e /init && test ! -e /var/lib/hacocoon-wsl && test ! -e /run/WSL && test ! -e /var/lib/hacocoon-control.sock && test -z "$WSL_INTEROP" && test -z "$(find /mnt -mindepth 1 -maxdepth 1 -print -quit)" && ! command -v cmd.exe && echo windows-ssh-command-complete'
    ) 'Connect from Windows OpenSSH to the Hacocoon Environment' -SSHProgress
    $remoteLines = $remote.Stdout -split "`r?`n"
    if ($remoteLines -notcontains 'windows-base-tool-ok' -or $remoteLines -notcontains 'windows-ssh-ok' -or $remoteLines -notcontains 'windows-workspace-ok' -or $remoteLines -notcontains 'windows-ssh-command-complete') {
        throw "Windows SSH did not complete the expected Workspace assertions; $($remote.Stderr)"
    }


    # The disposable GHA Windows user exercises real desktop-home installation.
    Write-Host 'BASE DEFINITION / NATIVE IMAGE / ORDINARY CREATE / WINDOWS SSH TOOL: PASS'
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
        $configurationProbe = @'
set -eu
umask 077
before=$(mktemp /tmp/haco-config-before-XXXXXX)
after=$(mktemp /tmp/haco-config-after-XXXXXX)
trap 'rm -f "$before" "$after"' EXIT
haco config > "$before"
haco config --file "$before" > "$after"
python3 - "$before" "$after" <<'PY'
import json, re, sys
with open(sys.argv[1]) as f: before = json.load(f)
with open(sys.argv[2]) as f: after = json.load(f)
assert before['policy'] == after['policy'], 'configuration meaning changed'
assert re.fullmatch(r'sha256:[a-f0-9]{64}', after['revision']), 'invalid receipt'
PY
'@
        $configurationProbe = $configurationProbe.Replace("`r", "")
        try {
            [void](Invoke-HacoHost @('/bin/bash', '-ec', $configurationProbe) 'Round-trip existing Policy through ordinary configuration commands')
            Write-Host 'INSTALLED CONFIGURATION INSPECT / PERSISTED RECEIPT / UNCHANGED POLICY: PASS'
        } catch {
            $DesktopFailures.Add('configuration')
            Write-Host 'INSTALLED CONFIGURATION: FAIL; continuing independent probes'
        }
        try {
            & (Join-Path $PSScriptRoot 'test_vscode_environment.ps1') -EnvironmentName $EnvironmentName -Distro $Distro
        } catch {
            $DesktopFailures.Add('vscode')
            Write-Host 'VS CODE REMOTE ENVIRONMENT: FAIL; continuing independent probes'
        }
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
        $projectSetupProbe = $projectSetupProbe.Replace("`r", "")
        try {
            [void](Invoke-HacoHost @('/bin/bash', '-ec', $projectSetupProbe, '--', $EnvironmentName) 'Exercise saved project setup')
            Write-Host 'PROJECT SETUP SAVE / REPLAY / FAILURE / UPDATE / CLEAR: PASS'
        } catch {
            $DesktopFailures.Add('project-setup')
            Write-Host 'PROJECT SETUP: FAIL; continuing independent probes'
        }

        try {
            $approvalProbePath = Join-Path $PSScriptRoot 'test_pending_approvals.py'
            $approvalProbeWsl = (Invoke-Wsl @('--exec', 'wslpath', '-u', '-a', $approvalProbePath) 'Locate installed approval acceptance fixture').Stdout.Trim()
            $approvalProbe = Invoke-HacoHost @('/usr/bin/python3', $approvalProbeWsl, $EnvironmentName) 'Review actual pending HTTPS requests through ordinary haco commands'
            if ($approvalProbe.Stdout.Trim() -ne 'PENDING_REVIEW_SAVED_ASK_DENY / ONE_SHOT_ALLOW / REASK_DENY: PASS') { throw 'Missing pending approval acceptance receipt' }
            Write-Host 'PENDING REVIEW SAVED ASK / CURRENT DENY / ONE-SHOT ALLOW / REASK / CLEANUP: PASS'
        } catch {
            $DesktopFailures.Add('approval-review')
            $reviewPhase = 'unknown'
            if ($_.Exception.Message -match 'PENDING APPROVAL REVIEW: FAIL phase=((?:configure|prepare|saved-ask-deny|one-shot-allow|reask-deny|cleanup)(?:-(?:configuration|python-prerequisite|clear-prerequisite|start-probe|wait-pending|validate-prompt|submit-review|validate-receipt|network-result|clear-recipe|verify-saved-policy))?(?:-(?:script-input|controller-request|project-setup|controller-client|logging-config|unit-busy|dns-failed|package-lock|after-prerequisite|package-install|package-update|command|timeout|validation|setup-(?:validate|lookup|recipe|start|execute|script|unknown)-(?:internal|invalid_argument|not_found|already_exists|unsupported|unavailable|denied|busy|incompatible_state|recovery_required)))?) cleanup_failed=(true|false)') {
                $reviewPhase = $Matches[1] + '-cleanup-failed-' + $Matches[2]
            }
            Write-DesktopProbeFailure 'PENDING APPROVAL REVIEW' $reviewPhase $_
            # Read only, through the existing pinned connection. Keep the original
            # failure even if diagnostics fail; no service restart or retry.
            try {
                . (Join-Path $PSScriptRoot 'windows_dns_diagnostic.ps1')
                $dnsState = Invoke-Captured $NativeSSH @('-F', $ConfigPath, '-i', $PrivateKey,
                    '-o', "UserKnownHostsFile=$KnownHosts", '-o', 'BatchMode=yes',
                    '-o', 'ConnectTimeout=10', $alias,
                    'systemctl show hacocoon-dns.service --property=Result --value')
                $dnsCategory = Get-DNSServiceFailureState $dnsState.Stdout $dnsState.ExitCode
                Write-Host "PENDING APPROVAL DNS RESULT: $dnsCategory"
            } catch {
                Write-Host 'PENDING APPROVAL DNS RESULT: unavailable'
            }
        }
        $previewProbe = @'
set -eu
name=$1
recipe=$(mktemp /tmp/haco-preview-XXXXXX)
trap 'rm -f "$recipe"' EXIT
cat > "$recipe" <<'RECIPE'
set -eu
if ! test -x /usr/bin/python3; then
  apt-get update
  apt-get install -y --no-install-recommends python3
fi
cp /workspace/windows-marker /workspace/haco-preview-marker.txt
printf '%s\n' PREVIEW_RUNTIME_READY
systemd-run --unit=haco-preview-probe --collect --service-type=exec /usr/bin/python3 -m http.server 3000 --bind 127.0.0.1 --directory /workspace
for attempt in $(seq 1 30); do
  if /usr/bin/python3 -c 'import socket; socket.create_connection(("127.0.0.1", 3000), timeout=1).close()'; then
    printf '%s\n' PREVIEW_SERVER_READY
    exit 0
  fi
  sleep 1
done
exit 1
RECIPE
if ! haco setup --script "$recipe" "$name" >&2; then
  printf "%s\n" PREVIEW_FAILURE_SETUP >&2; exit 1
fi
if ! haco setup --clear-script "$name" >/dev/null; then
  printf "%s\n" PREVIEW_FAILURE_CLEAR >&2; exit 1
fi
if ! haco open --port 3000 --no-browser "$name"; then
  printf "%s\n" PREVIEW_FAILURE_OPEN >&2; exit 1
fi
'@
        $previewProbe = $previewProbe.Replace("`r", "")
        $previewPhase = 'setup-and-open'
        try {
            $previewResult = Invoke-HacoHost @('/bin/bash', '-ec', $previewProbe, '--', $EnvironmentName) 'Start preview through ordinary project setup'
            $previewPhase = 'url-validation'
            $previewUrl = $previewResult.Stdout.Trim()
            if ($previewUrl -notmatch '^http://127\.0\.0\.1:[0-9]{1,5}/$') { throw 'Preview returned an unsafe URL' }
            $previewPhase = 'windows-http'
            $previewResponse = Invoke-WebRequest -Uri ($previewUrl + 'haco-preview-marker.txt') -TimeoutSec 10
            $previewPhase = 'workspace-marker'
            if ($previewResponse.Content.Trim() -ne 'windows-workspace-ok') { throw 'Preview reached a different Workspace' }

            # Render through an actual browser engine using an isolated disposable profile.
            $edge = Join-Path ${env:ProgramFiles(x86)} 'Microsoft/Edge/Application/msedge.exe'
            if (Test-Path -LiteralPath $edge -PathType Leaf) {
                $previewPhase = 'browser-render-and-cleanup'
                $browserProfile = Join-Path $Work 'preview-edge'
                $browserStart = [Diagnostics.ProcessStartInfo]::new()
                $browserStart.FileName = $edge
                $browserStart.UseShellExecute = $false
                $browserStart.CreateNoWindow = $true
                $browserStart.RedirectStandardOutput = $true
                $browserStart.RedirectStandardError = $true
                foreach ($argument in @('--headless', '--disable-gpu', '--no-first-run', '--disable-background-mode', "--user-data-dir=$browserProfile", '--dump-dom', ($previewUrl + 'haco-preview-marker.txt'))) {
                    [void]$browserStart.ArgumentList.Add($argument)
                }
                $browserProcess = [Diagnostics.Process]::new()
                $browserProcess.StartInfo = $browserStart
                $browserStarted = $false
                try {
                    if (-not $browserProcess.Start()) { throw 'Could not start preview browser' }
                    $browserStarted = $true
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
                    if ($browserStarted -and -not $browserProcess.HasExited) {
                        $browserProcess.Kill($true)
                        $browserProcess.WaitForExit()
                    }
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
            $previewPhase = 'reuse'
            $reusedPreview = Invoke-HacoHost @('/usr/local/bin/haco', 'open', '--port', '3000', '--no-browser', $EnvironmentName) 'Reuse preview connection'
            if ($reusedPreview.Stdout.Trim() -ne $previewUrl) { throw 'Preview did not reuse its connection' }
            $previewPhase = 'close'
            [void](Invoke-HacoHost @('/usr/local/bin/haco', 'open', '--port', '3000', '--close', $EnvironmentName) 'Close preview connection')
            $previewPhase = 'closed-connection-refusal'
            $previewRefused = $false
            try { [void](Invoke-WebRequest -Uri $previewUrl -TimeoutSec 3) } catch { $previewRefused = $true }
            if (-not $previewRefused) { throw 'Closed preview still accepts Windows HTTP requests' }
            Write-Host 'WINDOWS HTTP PREVIEW / REUSE / CONNECTION REFUSAL: PASS'
        } catch {
            $DesktopFailures.Add('preview')
            if ($previewPhase -eq 'setup-and-open' -and $_.Exception.Message -match 'PREVIEW_FAILURE_(SETUP|CLEAR|OPEN)') {
                $previewPhase = $Matches[1].ToLowerInvariant()
            }
            Write-DesktopProbeFailure 'WINDOWS HTTP PREVIEW' $previewPhase $_
        }
        try {
            . (Join-Path $PSScriptRoot 'test_windows_environment_transfer.ps1')
            Invoke-InstalledEnvironmentTransfer -BaseName $BuiltBaseName -PublicKeyWsl $PublicKeyWsl -PrivateKey $PrivateKey -NativeSSH $NativeSSH -Directory $Work -ReclamationManifest $ReclamationManifest
        } catch {
            $DesktopFailures.Add('environment-transfer')
            Write-Host 'INSTALLED ENV TRANSFER: FAIL; continuing independent probes'
        }
        $doctorPhase = 'invoke'
        try {
            Write-Host 'ACCEPTANCE: Diagnose Environment prerequisites'
            $environmentDoctor = Invoke-Captured 'wsl.exe' @('-d', $Distro, '-u', 'root', '--exec', 'incus', 'exec', 'haco-host', '--project', 'hacocoon', '--', '/usr/local/bin/haco', 'doctor', '--json', $EnvironmentName)
            $doctorPhase = 'json'
            $environmentReport = $environmentDoctor.Stdout | ConvertFrom-Json
            $doctorPhase = 'workspace-identity'
            if ($environmentReport.environment -ne $EnvironmentName -or [string]::IsNullOrWhiteSpace($environmentReport.workspace.id)) { throw 'Environment doctor reported the wrong Workspace' }
            $doctorPhase = 'prerequisite-status'
            foreach ($check in $environmentReport.checks) {
                if ($check.name -cin @('runtime', 'workspace', 'dns_service', 'ssh_service') -and $check.status -cin @('ok', 'failed', 'skipped')) {
                    Write-Host ('ENVIRONMENT DOCTOR CHECK: ' + $check.name + '=' + $check.status)
                }
            }
            if ($environmentDoctor.ExitCode -ne 0 -or @($environmentReport.checks | Where-Object { $_.status -ne 'ok' }).Count -ne 0) { throw 'Environment doctor did not pass local prerequisite checks' }
            Write-Host 'ENVIRONMENT DOCTOR WORKSPACE / DNS / SSH PREREQUISITES: PASS'
        } catch {
            $DesktopFailures.Add('doctor')
            Write-DesktopProbeFailure 'ENVIRONMENT DOCTOR' $doctorPhase $_
        }



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
            [void](Invoke-Wsl @('--exec', 'rm', '-f', "$Workspace/windows-marker", "$Workspace/haco-preview-marker.txt") 'Remove acceptance Workspace marker')
            [void](Invoke-Wsl @('--exec', 'rmdir', $Workspace) 'Remove acceptance Workspace')
        } catch {
            $CleanupFailed = $true; Write-Warning $_
        }
    }
    if ($BuiltBaseFingerprint -and $EnvironmentGone) {
        try {
            $cleanupBase = @'
import json, re, subprocess, sys
fingerprint, name = sys.argv[1:]
if not re.fullmatch(r'[a-f0-9]{64}',fingerprint) or not re.fullmatch(r'win-base-[a-f0-9]{16}',name):
    raise SystemExit('invalid fixture Base identity')
def query(path):
    return json.loads(subprocess.check_output(['incus','query',path],text=True))
image=query('/1.0/images/'+fingerprint+'?project=hacocoon')
props=image.get('properties',{})
if image.get('fingerprint')!=fingerprint or image.get('public') or image.get('type')!='container' or props.get('user.hacocoon.kind')!='base-image' or props.get('user.hacocoon.base-name')!=name or not re.fullmatch(r'env-[a-f0-9]{32}',props.get('user.hacocoon.build-instance','')):
    raise SystemExit('fixture Base ownership mismatch')
subprocess.run(['incus','image','delete',fingerprint,'--project','hacocoon'],check=True)
images=query('/1.0/images?project=hacocoon&recursion=1')
if any(i.get('fingerprint')==fingerprint for i in images):
    raise SystemExit('fixture Base deletion unconfirmed')
print('PASS exact-owned test Base image removed; shared parent retained')
'@
            [void](Invoke-Wsl @('-u','root','--exec','python3','-c',$cleanupBase,$BuiltBaseFingerprint,$BuiltBaseName) 'Delete only the verified test Base image')
        } catch { $CleanupFailed=$true; Write-Warning $_ }
    }
    foreach ($path in @($KnownHosts, $ConfigPath, $PublicKey, $PrivateKey, $BaseDefinition)) {
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
if ($DesktopFailures.Count -ne 0) {
    throw ("Desktop acceptance failed: " + ($DesktopFailures -join ', ') + "; independent probes and cleanup were attempted.")
}
Write-Host 'WINDOWS DIRECT ENVIRONMENT SSH: PASS'

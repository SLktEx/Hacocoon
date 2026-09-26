#Requires -Version 7.0
param(
    [Parameter(Mandatory=$true)]
    [string]$ReclamationManifest,
    [string]$Distro = 'Hacocoon'
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$fixtureName = 'reclaim-fixture-' + [guid]::NewGuid().ToString('N').Substring(0,16)
$work = Join-Path ([IO.Path]::GetTempPath()) $fixtureName
$privateKey = Join-Path $work 'id_ed25519'
$publicKey = "$privateKey.pub"
$baseDefinition = Join-Path $work 'base.json'
$baseName = 'win-base-' + [guid]::NewGuid().ToString('N').Substring(0,16)
$builtBaseFingerprint = $null

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
        $process.Dispose()
        throw "Acceptance child timed out after $($TimeoutMilliseconds)ms."
    }
    if (-not [Threading.Tasks.Task]::WaitAll([Threading.Tasks.Task[]]@($stdoutTask, $stderrTask), 10000)) {
        $process.Dispose()
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
        $failure = [Exception]::new("$Description failed with exit $($result.ExitCode).")
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

function Write-DesktopProbeFailure([string]$Label, [string]$Phase, [Management.Automation.ErrorRecord]$Failure) {
    $exitCode = 'unknown'
    if ($Failure.Exception.Data['acceptance_exit_code'] -is [int]) {
        $exitCode = [string]$Failure.Exception.Data['acceptance_exit_code']
    }
    Write-Host ($Label + ': FAIL phase=' + $Phase + ' exit=' + $exitCode + ' line=' + $Failure.InvocationInfo.ScriptLineNumber)
}

function Update-SSHTestPolicy([string]$Action, [string]$TargetEnvironment) {
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
    [void](Invoke-Wsl @('-u','root','--exec','python3','-c',$policyScript,$Action,$TargetEnvironment) 'Configure only this transfer fixture Environment package Policy')
}

if (Test-Path -LiteralPath $ReclamationManifest) {
    throw 'Reclamation manifest already exists.'
}
[void][IO.Directory]::CreateDirectory($work)
try {
    $nativeSSH = Join-Path $env:WINDIR 'System32\OpenSSH\ssh.exe'
    $nativeKeygen = Join-Path $env:WINDIR 'System32\OpenSSH\ssh-keygen.exe'
    foreach ($tool in @($nativeSSH, $nativeKeygen)) {
        if (-not (Test-Path -LiteralPath $tool -PathType Leaf)) { throw "Required Windows OpenSSH tool is missing: $tool" }
    }

    [void](Invoke-Checked $nativeKeygen @('-q','-t','ed25519','-N','','-f',$privateKey) 'Generate Windows-owned reclamation fixture SSH key')
    $translatedPublicKey = Invoke-Wsl @('--exec','wslpath','-u','-a',$publicKey) 'Translate Windows reclamation public-key path into WSL'
    $publicKeyWsl = $translatedPublicKey.Stdout.Trim()
    if (-not $publicKeyWsl.StartsWith('/mnt/')) { throw 'Windows public key did not translate below /mnt.' }
    [void](Invoke-HacoHost @('test','-r',$publicKeyWsl) 'Read reclamation fixture public key from trusted Host')

    $definition = @{name=$baseName; run="printf '#!/bin/sh\necho windows-base-tool-ok\n' > /usr/local/bin/haco-base-tool`nchmod 0755 /usr/local/bin/haco-base-tool`n"} | ConvertTo-Json -Compress
    [IO.File]::WriteAllText($baseDefinition, $definition, [Text.UTF8Encoding]::new($false))
    $translatedDefinition = Invoke-Wsl @('--exec','wslpath','-u','-a',$baseDefinition) 'Translate reclamation Base definition'
    $built = Invoke-HacoHost @('/usr/local/bin/haco','base','build','--json',$translatedDefinition.Stdout.Trim()) 'Build reclamation Base through ordinary haco'
    $builtResult = $built.Stdout.Trim() | ConvertFrom-Json
    if ($builtResult.state -ne 'ready' -or $builtResult.base.name -ne $baseName -or $builtResult.base.revision -notmatch '^sha256:[a-f0-9]{64}$') {
        throw 'Incomplete reclamation Base build.'
    }
    $builtBaseFingerprint = $builtResult.base.revision.Substring(7)

    . (Join-Path $PSScriptRoot 'test_windows_environment_transfer.ps1')
    Invoke-InstalledEnvironmentTransfer `
        -BaseName $baseName `
        -BaseRevision ('sha256:' + $builtBaseFingerprint) `
        -PublicKeyWsl $publicKeyWsl `
        -PrivateKey $privateKey `
        -NativeSSH $nativeSSH `
        -Directory $work `
        -ReclamationManifest $ReclamationManifest

    if (-not (Test-Path -LiteralPath $ReclamationManifest -PathType Leaf)) {
        throw 'Reclamation transfer fixture did not produce its manifest.'
    }
    Write-Host 'WINDOWS RECLAMATION TRANSFER FIXTURE: PASS'
} finally {
    foreach ($path in @($publicKey, $privateKey, $baseDefinition)) {
        if (Test-Path -LiteralPath $path -PathType Leaf) { Remove-Item -LiteralPath $path -Force }
    }
    if (Test-Path -LiteralPath $work -PathType Container) {
        [IO.Directory]::Delete($work, $false)
    }
}

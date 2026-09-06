# Follow-on API/packet acceptance after the exact BAT journey has passed.
# Policy configuration is the documented administrator operation. It does not
# prepare/repair networks, storage, services, accounts, or installer state.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$environmentName = 'm1-egress-' + [guid]::NewGuid().ToString('N').Substring(0,16)
$work = Join-Path ([IO.Path]::GetTempPath()) $environmentName
[IO.Directory]::CreateDirectory($work) | Out-Null
$binary = Join-Path $work 'installed-egress-check'
$policyCreated = $false
$policy = @{default='deny'; rules=@(@{
    capability='network.egress'; action='connect'; resource='github.com'
    environment=$environmentName; attributes=@{protocol='https'; port='443'}
    decision='allow'; reason='Installed Environment HTTPS acceptance'
})} | ConvertTo-Json -Depth 5 -Compress
$policyBytes = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($policy))
try {
    $oldGoos=$env:GOOS; $oldGoarch=$env:GOARCH; $oldCgo=$env:CGO_ENABLED
    try {
        $env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'
        go build -trimpath -o $binary ./tools/installed-egress-check
        if ($LASTEXITCODE -ne 0) { throw 'Cannot build installed-controller acceptance client' }
    } finally { $env:GOOS=$oldGoos; $env:GOARCH=$oldGoarch; $env:CGO_ENABLED=$oldCgo }
    $linuxBinary = (& wsl.exe -d Hacocoon --exec wslpath -a -u $binary).Trim()
    if ($LASTEXITCODE -ne 0 -or -not $linuxBinary.StartsWith('/')) { throw 'Cannot locate acceptance workload in WSL' }

    # Never overwrite a pre-existing administrator Policy. This exact allow is
    # independent of the installer and is loaded by its already-running daemon.
    $createPolicy = @'
import base64, os, sys
data = base64.b64decode(sys.argv[1], validate=True)
fd = os.open('/var/lib/hacocoon/policy.json', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
with os.fdopen(fd, 'wb') as f:
    f.write(data)
    f.flush()
    os.fsync(f.fileno())
'@
    & wsl.exe -d Hacocoon -u root --exec python3 -c $createPolicy $policyBytes
    if ($LASTEXITCODE -ne 0) { throw 'Cannot install the explicit acceptance Policy; existing Policy is preserved' }
    $policyCreated = $true

    # Default ordinary WSL user; no HACO_* overrides and no guest-local daemon.
    & wsl.exe -d Hacocoon --exec $linuxBinary check $environmentName
    if ($LASTEXITCODE -ne 0) { throw 'Installed controller / Environment egress acceptance failed' }
} finally {
    if ($policyCreated) {
        $removePolicy = @'
import base64, os, stat, sys
path = '/var/lib/hacocoon/policy.json'
expected = base64.b64decode(sys.argv[1], validate=True)
fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
with os.fdopen(fd, 'rb') as f:
    identity = os.fstat(f.fileno())
    if not stat.S_ISREG(identity.st_mode) or identity.st_uid != 0 or f.read(len(expected) + 1) != expected:
        raise SystemExit('Policy changed; retaining it for inspection')
current = os.lstat(path)
if (current.st_dev, current.st_ino) != (identity.st_dev, identity.st_ino):
    raise SystemExit('Policy identity changed; retaining it for inspection')
os.unlink(path)
'@
        & wsl.exe -d Hacocoon -u root --exec python3 -c $removePolicy $policyBytes
        if ($LASTEXITCODE -ne 0) { throw 'Cannot remove the owned acceptance Policy' }
    }
    [IO.File]::Delete($binary)
    [IO.Directory]::Delete($work)
}
$global:LASTEXITCODE = 0
Write-Host 'INSTALLED CONTROLLER ENVIRONMENT EGRESS: PASS'

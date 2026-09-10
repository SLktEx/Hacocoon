# Installed G1 acceptance; called by the existing Windows SSH fixture.
function Invoke-InstalledEnvironmentTransfer {
    param([string]$BaseName, [string]$PublicKeyWsl, [string]$PrivateKey, [string]$NativeSSH, [string]$Directory)
    if ($env:GITHUB_ACTIONS -ne 'true') { throw 'Transfer fixture requires the disposable GHA user' }
    $nonce = [guid]::NewGuid().ToString('N').Substring(0,16)
    $source = 'win-ssh-' + $nonce
    $destination = 'win-import-' + $nonce
    $resume = 'win-resume-' + $nonce
    $repository = 'transfer-repo-' + $nonce
    $workspace = 'transfer-work-' + $nonce
    $hostDirectory = '/tmp/haco-transfer-' + $nonce
    $bundle = $hostDirectory + '/saved.haco'
    $clientDirectory = Join-Path $Directory 'transfer'
    if (Test-Path -LiteralPath $clientDirectory) { throw 'Transfer client directory already exists' }
    [void][IO.Directory]::CreateDirectory($clientDirectory)
    $config = Join-Path $clientDirectory 'config'
    $known = Join-Path $clientDirectory 'known_hosts'
    $phase = 'seed-repository'
    $policyAdded = $false
    try {
        $seed = 'set -eu
umask 077
dir=$1
mkdir "$dir"
git init --quiet --initial-branch=main "$dir/repository"
printf initial > "$dir/repository/tracked"
git -C "$dir/repository" add tracked
git -C "$dir/repository" -c user.name=Transfer -c user.email=transfer@example.invalid commit --quiet -m initial
'
        [void](Invoke-HacoHost @('/bin/sh','-ec',$seed.Replace("`r",''),'--',$hostDirectory) 'Create isolated trusted Host transfer repository')
        $phase = 'managed-workspace'
        [void](Invoke-HacoHost @('/usr/local/bin/haco','repo','clone','--branch','main',$repository,('file://' + $hostDirectory + '/repository')) 'Register transfer source repository')
        [void](Invoke-HacoHost @('/usr/local/bin/haco','workspace','create','--repo',$repository,$workspace) 'Create managed transfer Workspace')
        $phase = 'source-create'
        [void](Invoke-HacoHost @('/usr/local/bin/haco','env','create','--workspace',('managed:' + $workspace),'--base',$BaseName,$source) 'Create managed source Environment')
        $sourceStatus = (Invoke-HacoHost @('/usr/local/bin/haco','env','status','--json',$source) 'Observe source identity').Stdout | ConvertFrom-Json
        $sourceStore = [string]$sourceStatus.environment.persistent_resource.id
        if ($sourceStore -notmatch '^oci:[a-z0-9][a-z0-9-]{0,56}$') { throw 'Source OCI Store missing' }
        $phase = 'source-ssh'
        Update-SSHTestPolicy 'add' $source
        $policyAdded = $true
        $first = (Invoke-HacoHost @('/usr/local/bin/haco','env','ssh','--key',$PublicKeyWsl,$source) 'Prepare source SSH through installed package Policy').Stdout | ConvertFrom-Json
        $configure = {
            param($Connection)
            if ($Connection.kind -ne 'ssh' -or $Connection.host -ne '127.0.0.1' -or $Connection.user -ne 'root' -or [int]$Connection.port -lt 1 -or [int]$Connection.port -gt 65535 -or [int]$Connection.target_port -ne 22) { throw 'Invalid transfer SSH boundary' }
            $parts = ([string]$Connection.host_public_key).Trim() -split '\s+'
            if ($parts.Count -ne 2 -or $parts[0] -ne 'ssh-ed25519' -or $parts[1] -notmatch '^[A-Za-z0-9+/=]+$') { throw 'Invalid transfer SSH host identity' }
            [IO.File]::WriteAllText($config, "Host transfer`n  HostName 127.0.0.1`n  Port $($Connection.port)`n  User root`n  StrictHostKeyChecking yes`n  GlobalKnownHostsFile none`n  IdentitiesOnly yes`n  ForwardAgent no`n  ProxyCommand none`n  ProxyJump none`n  PermitLocalCommand no`n", [Text.UTF8Encoding]::new($false))
            [IO.File]::WriteAllText($known, "[127.0.0.1]:$($Connection.port) $($parts[0]) $($parts[1])`n", [Text.UTF8Encoding]::new($false))
        }
        & $configure $first
        $sshArguments = @('-F',$config,'-i',$PrivateKey,'-o',"UserKnownHostsFile=$known",'-o','BatchMode=yes','-o','ConnectTimeout=10','transfer')
        $sourceScript = 'set -eu; cd /workspace; test -d .git; printf unpushed > committed-locally; git add committed-locally; git -c user.name=Transfer -c user.email=transfer@example.invalid commit --quiet -m local; printf uncommitted > tracked; printf untracked > untracked; printf rootfs-kept > /root/transfer-marker; printf oci-kept > /var/lib/hacocoon-oci/transfer-marker; git rev-parse HEAD'
        $commit = (Invoke-Checked $NativeSSH ($sshArguments + @($sourceScript)) 'Make real unpushed, uncommitted and untracked work over Windows SSH').Stdout.Trim()
        if ($commit -notmatch '^[a-f0-9]{40}$') { throw 'Source Git commit missing' }
        $phase = 'export'
        [void](Invoke-HacoHost @('/usr/local/bin/haco','env','stop',$source) 'Stop transfer source')
        $exported = (Invoke-HacoHost @('/usr/local/bin/haco','env','export','--json',$source,$bundle) 'Export stopped source from trusted Host client').Stdout | ConvertFrom-Json
        if ($exported.file -cne $bundle -or [long]$exported.result.bytes -le 0 -or $exported.result.sha256 -notmatch '^[a-f0-9]{64}$') { throw 'Incomplete exported bundle receipt' }
        $phase = 'source-delete'
        [void](Invoke-HacoHost @('/usr/local/bin/haco','env','delete',$source) 'Delete source Env while retaining data and bundle')
        Update-SSHTestPolicy 'remove' $source
        $policyAdded = $false
        $phase = 'import'
        $imported = (Invoke-HacoHost @('/usr/local/bin/haco','env','import','--json',$bundle,$destination) 'Import independent Environment through installed controller').Stdout | ConvertFrom-Json
        if ($imported.environment -ne $destination -or $imported.state -ne 'running' -or $imported.workspace -notmatch '^import-[a-f0-9]{16}$' -or $imported.oci -notmatch '^oci:import-[a-f0-9]{16}$' -or $imported.workspace -eq $workspace -or $imported.oci -eq $sourceStore) { throw 'Import receipt lost independent identities' }
        if (@($imported.offline) -notcontains $repository) { throw 'Source Host file route was not imported offline' }
        $digest = (Invoke-HacoHost @('sha256sum',$bundle) 'Verify saved bundle remained unchanged').Stdout -split '\s+'
        if ($digest[0] -cne $exported.result.sha256) { throw 'Saved bundle changed' }
        $status = (Invoke-HacoHost @('/usr/local/bin/haco','env','status','--json',$destination) 'Verify independent imported management identity').Stdout | ConvertFrom-Json
        $base = $status.environment.PSObject.Properties['base']
        if (($null -ne $base -and $null -ne $base.Value) -or $status.environment.runtime_ref -eq $sourceStatus.environment.runtime_ref -or $status.environment.workspace.id -eq $sourceStatus.environment.workspace.id) { throw 'Imported management identity or Base dependency is stale' }
        $phase = 'imported-ssh'
        # No package Policy is granted to the imported Env: sshd comes from saved rootfs.
        $second = (Invoke-HacoHost @('/usr/local/bin/haco','env','ssh','--key',$PublicKeyWsl,$destination) 'Prepare imported SSH without inheriting source grants').Stdout | ConvertFrom-Json
        if ($second.host_public_key -eq $first.host_public_key) { throw 'Imported Env reused the source SSH host key' }
        & $configure $second
        $check = 'set -eu; cd /workspace; test "$(git rev-parse HEAD)" = ' + $commit + '; test "$(cat committed-locally)" = unpushed; test "$(cat tracked)" = uncommitted; test "$(cat untracked)" = untracked; test "$(cat /root/transfer-marker)" = rootfs-kept; test "$(cat /var/lib/hacocoon-oci/transfer-marker)" = oci-kept; test ! -e /var/lib/hacocoon-control.sock; test ! -e /var/lib/incus/unix.socket; printf continued-over-ssh > continued; printf transfer-ssh-ok'
        $continued = Invoke-Checked $NativeSSH ($sshArguments + @($check)) 'Resume imported work with Windows SSH and pinned fresh host key'
        if ($continued.Stdout -cne 'transfer-ssh-ok') { throw 'Imported SSH work not confirmed' }
        $phase = 'retention-recreate'
        [void](Invoke-HacoHost @('/usr/local/bin/haco','env','disconnect',$destination,[string]$second.id) 'Revoke imported SSH')
        [void](Invoke-HacoHost @('/usr/local/bin/haco','env','delete',$destination) 'Delete imported Env while retaining resumed work')
        [void](Invoke-HacoHost @('/usr/local/bin/haco','env','create','--workspace',('managed:' + $imported.workspace),'--resource',[string]$imported.oci,'--base',$BaseName,$resume) 'Reattach imported Workspace and OCI to a fresh Env')
        $read = Invoke-Wsl @('-u','root','--exec','incus','exec',('haco-' + $resume),'--project','hacocoon','--','/bin/sh','-ec','test "$(cat /workspace/continued)" = continued-over-ssh; test "$(cat /var/lib/hacocoon-oci/transfer-marker)" = oci-kept; printf retained') 'Verify SSH work survived Env deletion and recreation'
        if ($read.Stdout -cne 'retained') { throw 'Retained data not confirmed' }
        $phase = 'owned-cleanup'
        [void](Invoke-HacoHost @('/usr/local/bin/haco','env','delete',$resume) 'Delete transfer recreation fixture')
        foreach ($id in @([string]$imported.oci, $sourceStore)) {
            [void](Invoke-HacoHost @('/usr/local/bin/haco','plugin','oci','store','delete','--yes',$id.Substring(4)) 'Explicitly delete exact transfer fixture Store')
        }
        foreach ($id in @([string]$imported.workspace, $workspace)) {
            [void](Invoke-HacoHost @('/usr/local/bin/haco','workspace','delete','--yes',$id) 'Explicitly delete exact transfer fixture Workspace')
        }
        [void](Invoke-HacoHost @('/usr/local/bin/haco','repo','delete','--yes',$repository) 'Delete exact transfer source repository registration')
        Write-Host 'INSTALLED ENV EXPORT / SOURCE DELETE / IMPORT / WINDOWS SSH / RETAINED WORK RECREATE: PASS'
        Write-Host ('Transfer bundle and raw local test repository retained inside trusted Host: ' + $hostDirectory)
    } catch {
        Write-DesktopProbeFailure 'INSTALLED ENV TRANSFER' $phase $_
        Write-Host ('Transfer fixture retained: source=' + $source + ' imported=' + $destination + ' resume=' + $resume + ' host_directory=' + $hostDirectory)
        throw [Exception]::new('Installed Environment transfer failed; see fixed phase diagnostics')
    } finally {
        if ($policyAdded) { Update-SSHTestPolicy 'remove' $source }
        foreach ($file in @($config,$known)) {
            if (Test-Path -LiteralPath $file -PathType Leaf) { Remove-Item -LiteralPath $file -Force }
        }
        [IO.Directory]::Delete($clientDirectory, $false)
    }
}

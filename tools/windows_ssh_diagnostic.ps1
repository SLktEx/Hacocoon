# Fixture diagnostics only. Never return raw SSH output, key paths or peer data.
# Observed markers diagnose progress; they do not establish trusted identity or PASS.
function Get-TransferFailureEvidence([string]$Message) {
    $observations = [Collections.Generic.List[string]]::new()
    foreach ($entry in @(
        @('destination-filesystem', 'destination requires anonymous-file support'),
        @('invalid-bundle', 'invalid or incomplete Environment transfer bundle'),
        @('volume-export', 'Incus export failed for'),
        @('volume-cleanup', 'Incus backup cleanup unconfirmed'),
        @('rootfs-transfer', 'rootfs image transfer unconfirmed'),
        @('rootfs-cleanup', 'rootfs export cleanup unconfirmed'),
        @('recovery-required', '\brecovery[ _]required\b'),
        @('incompatible-state', '\bincompatible[ _]state\b'),
        @('unavailable', '\b(?:runtime unavailable|unavailable:)'),
        @('unsupported', '\bunsupported\b'),
        @('permission-denied', '\bpermission denied\b'),
        @('file-exists', '\bfile exists\b'),
        @('no-space', '\bno space left on device\b'),
        @('timeout', '\b(?:context deadline exceeded|timed out)\b')
    )) {
        if ($Message -cmatch $entry[1]) { $observations.Add($entry[0]) }
    }
    if ($observations.Count -eq 0) { return 'unclassified' }
    return $observations -join ','
}

function Get-SSHProgressEvidence([string]$Stdout, [string]$Stderr) {
    $observations = [Collections.Generic.List[string]]::new()
    foreach ($entry in @(
        @('connected', '(?m)^debug1: Connection established\.\r?$'),
        @('authenticated', '(?m)^Authenticated to [^\r\n]+ using "publickey"\.\r?$'),
        @('session', '(?m)^debug1: Entering interactive session\.\r?$'),
        @('exit_received', '(?m)^debug1: Exit status [0-9]+\r?$')
    )) {
        if ($Stderr -cmatch $entry[1]) { $observations.Add($entry[0]) }
    }
    $lines = $Stdout -split "`r?`n"
    foreach ($marker in @('windows-base-tool-ok','windows-ssh-ok','windows-workspace-ok','windows-ssh-command-complete')) {
        if ($lines -ccontains $marker) { $observations.Add($marker) }
    }
    if ($observations.Count -eq 0) { return 'no-recognized-progress' }
    return $observations -join ','
}

# Fixture diagnostics only. Never return raw SSH output, key paths or peer data.
# Observed markers diagnose progress; they do not establish trusted identity or PASS.
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

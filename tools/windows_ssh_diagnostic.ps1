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
    if ($Stderr -cmatch '(?m)^\[failed\] operation=stream stage=target reason=(not_found|already_exists|invalid_argument|unsupported|unavailable|denied|busy|incompatible_state|recovery_required|canceled|failed)\r?$') {
        $observations.Add('stream_' + $Matches[1])
    }
    # Native WSL errors were observed as UTF-16 bytes decoded by the child
    # capture. Removing NULs is only for this fixed diagnostic, never a PASS.
    $wslOutput = ($Stdout + "`n" + $Stderr).Replace("`0", '')
    if ($wslOutput -cmatch '(?m)^Error code: Wsl/Service/E_UNEXPECTED\r?$') {
        $observations.Add('wsl_service_unexpected')
    }
    $lines = $Stdout -split "`r?`n"
    foreach ($marker in @('windows-base-tool-ok','windows-ssh-ok','windows-workspace-ok','windows-ssh-command-complete')) {
        if ($lines -ccontains $marker) { $observations.Add($marker) }
    }
    if ($observations.Count -eq 0) { return 'no-recognized-progress' }
    return $observations -join ','
}

# Preserve the existing strict negative-check criteria. A transport failure is
# unconfirmed refusal, not evidence of accepting a changed key and never PASS.
function Get-SSHHostKeyCheckOutcome([int]$ExitCode, [string]$Stdout, [string]$Stderr) {
    if ($ExitCode -eq 0) { return 'exit_zero' }
    if ($Stdout.Contains('MUST-NOT-EXECUTE')) { return 'command_marker' }
    if ($Stderr -notmatch 'HOST IDENTIFICATION HAS CHANGED|Host key verification failed') { return 'refusal_unconfirmed' }
    return 'refused'
}

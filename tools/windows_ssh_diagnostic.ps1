# Fixture diagnostics only. Never return raw SSH output, key paths or peer data.
function Test-EnvironmentDoctorCheck($Check) {
    # A path Workspace has no Git broker. Absence is applicable only to this
    # check; failed, skipped or unknown prerequisites still refuse acceptance.
    return $Check.status -ceq 'ok' -or ($Check.name -ceq 'git_broker' -and $Check.status -ceq 'not_applicable')
}

# Observed markers diagnose progress; they do not establish trusted identity or PASS.
function Get-SSHProgressEvidence([string]$Stdout, [string]$Stderr) {
    $observations = [Collections.Generic.List[string]]::new()
    foreach ($entry in @(
        @('proxy_started', '(?m)^debug1: Executing proxy command: [^\x00-\x1f\x7f]+\r?$'),
        @('local_version_sent', '(?m)^debug1: Local version string SSH-2\.0-[^\x00-\x1f\x7f]+\r?$'),
        @('remote_version_received', '(?m)^debug1: Remote protocol version 2\.0, remote software version [^\x00-\x1f\x7f]+\r?$'),
        @('banner_timeout', '(?m)^Connection timed out during banner exchange\r?$'),
        @('banner_invalid', '(?m)^kex_exchange_identification: banner line contains invalid characters\r?$'),
        @('banner_closed', '(?m)^kex_exchange_identification: Connection closed by remote host\r?$'),
        @('controller_unready', '(?m)^haco: controller readiness failed\r?$'),
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

# These receipts are observations only. Never allow serialization or a broken
# output sink to replace an acceptance exception or interrupt owned cleanup.
function Write-SSHAcceptancePhase([string]$Phase, [string]$State, [Diagnostics.Stopwatch]$Clock, $Capture = $null, [string]$Progress = '') {
    try {
        $phases = @('changed-host-key','cleanup-policy','cleanup-disconnect','cleanup-environment','cleanup-refusal','cleanup-workspace','cleanup-base','cleanup-local-files')
        $record = [ordered]@{
            component = 'ci'; operation = 'windows_ssh_acceptance'
            phase = $(if ($Phase -cin $phases) { $Phase } else { 'unknown' })
            state = $(if ($State -cin @('start','completed','failed')) { $State } else { 'unknown' })
            duration_ms = $Clock.ElapsedMilliseconds
        }
        if ($null -ne $Capture) {
            # Only completed Invoke-Captured results supply this numeric object.
            foreach ($field in @('stdout_chars','stderr_chars','capture_duration_ms','exit_code')) {
                $value = $Capture.$field
                if (($value -is [int] -or $value -is [long]) -and ($field -ceq 'exit_code' -or $value -ge 0)) { $record[$field] = $value }
            }
            $record['capture_complete'] = $Capture.capture_complete -is [bool] -and $Capture.capture_complete
        }
        $tokens = @('proxy_started','local_version_sent','remote_version_received','banner_timeout','banner_invalid','banner_closed','controller_unready',
            'connected','authenticated','session','exit_received','wsl_service_unexpected',
            'windows-base-tool-ok','windows-ssh-ok','windows-workspace-ok','windows-ssh-command-complete',
            'stream_not_found','stream_already_exists','stream_invalid_argument','stream_unsupported','stream_unavailable','stream_denied','stream_busy',
            'stream_incompatible_state','stream_recovery_required','stream_canceled','stream_failed','no-recognized-progress')
        $record['ssh_progress'] = @($Progress -split ',' | Where-Object { $_ -cin $tokens } | Select-Object -Unique)
        [Console]::Out.WriteLine(($record | ConvertTo-Json -Depth 3 -Compress))
        [Console]::Out.Flush()
    } catch { }
}

function Write-SSHAcceptanceSummary([bool]$PrimaryFailed, [string]$PrimaryPhase, [string[]]$DesktopFailures, [bool]$CleanupFailed, [bool]$EnvironmentGone, [System.Collections.IDictionary]$Cleanup, [Diagnostics.Stopwatch]$Clock) {
    try {
        $desktopNames = @('configuration','vscode','project-setup','approval-review','preview','environment-transfer','doctor')
        $outcomes = [ordered]@{}
        foreach ($name in @('policy','disconnect','environment','refusal','workspace','base','local_files')) {
            $value = $Cleanup[$name]
            $outcomes[$name] = $(if ($value -is [string] -and $value -cin @('not_attempted','unconfirmed','completed','failed','blocked')) { $value } else { 'unknown' })
        }
        $record = [ordered]@{
            component = 'ci'; operation = 'windows_ssh_acceptance'; phase = 'summary'
            state = $(if ($PrimaryFailed -or $CleanupFailed -or $DesktopFailures.Count) { 'failed' } else { 'completed' })
            primary_failed = $PrimaryFailed
            primary_phase = $(if ($PrimaryPhase -cin @('native-probes','native-tunnel','changed-host-key','complete')) { $PrimaryPhase } else { 'unknown' })
            desktop_failures = @($DesktopFailures | Where-Object { $_ -cin $desktopNames } | Select-Object -Unique)
            desktop_failure_unknown = @($DesktopFailures | Where-Object { $_ -cnotin $desktopNames }).Count -ne 0
            cleanup_failed = $CleanupFailed
            environment_absence = $(if ($EnvironmentGone -and $outcomes.environment -ceq 'completed') { 'delete_succeeded' } elseif ($EnvironmentGone -and $outcomes.environment -ceq 'not_attempted') { 'not_attempted' } else { 'unconfirmed' })
            cleanup = $outcomes; duration_ms = $Clock.ElapsedMilliseconds
        }
        [Console]::Out.WriteLine(($record | ConvertTo-Json -Depth 3 -Compress))
        [Console]::Out.Flush()
    } catch { }
}

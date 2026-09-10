# Minimize untrusted guest diagnostics; never echo arbitrary systemctl output.
function Get-DNSServiceFailureState([string]$Output, [int]$ExitCode) {
    if ($ExitCode -ne 0) { return 'unavailable' }
    switch -Exact ($Output.Trim()) {
        'success' { return 'success' }
        'start-limit-hit' { return 'start-limit-hit' }
        'exit-code' { return 'exit-code' }
        'timeout' { return 'timeout' }
        'signal' { return 'signal' }
        'core-dump' { return 'core-dump' }
        'resources' { return 'resources' }
        default { return 'unknown' }
    }
}

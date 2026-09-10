$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'windows_dns_diagnostic.ps1')
foreach ($state in @('success','start-limit-hit','exit-code','timeout','signal','core-dump','resources')) {
    if ((Get-DNSServiceFailureState ($state + "`n") 0) -cne $state) { throw 'Known state lost' }
}
foreach ($hostile in @('SECRET','start-limit-hit SECRET',"start-limit-hit`nsuccess",'')) {
    if ((Get-DNSServiceFailureState $hostile 0) -cne 'unknown') { throw 'Untrusted state escaped allowlist' }
}
if ((Get-DNSServiceFailureState 'start-limit-hit' 1) -cne 'unavailable') { throw 'Failed query treated as observation' }
Write-Host 'DNS failure diagnostic allowlist: PASS'

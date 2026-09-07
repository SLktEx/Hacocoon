#Requires -Version 7.0
param([string]$Distro = 'Hacocoon')
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot '../scripts/windows-review.ps1')
$scheme = Get-HacocoonReviewScheme $Distro
$directory = Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) ('Hacocoon\review\' + $scheme)
$adapter = [IO.Path]::GetFullPath((Join-Path $directory 'haco-review.exe'))
$configuration = Get-Content -Raw -LiteralPath (Join-Path $directory 'review.json') | ConvertFrom-Json
if ($configuration.distribution -ine $Distro) { throw 'Native review configuration targets another distribution' }
$registration = 'HKCU:\Software\Classes\' + $scheme
$owner = Get-ItemProperty -LiteralPath $registration
$command = (Get-Item -LiteralPath ($registration + '\shell\open\command')).GetValue('')
if ($owner.HacocoonDistribution -ine $Distro -or $command -cne ('"' + $adapter + '" "%1"')) { throw 'Native review registration differs from the installed adapter' }
function Invoke-ReviewProbe([string[]]$Arguments, [int]$ExitCode, [string]$Expected) {
    $info = [Diagnostics.ProcessStartInfo]::new()
    $info.FileName = $adapter
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardInput = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    foreach ($argument in $Arguments) { [void]$info.ArgumentList.Add($argument) }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $info
    $started = $false
    try {
        if (-not $process.Start()) { throw 'Native review process could not start' }
        $started = $true
        $output = $process.StandardOutput.ReadToEndAsync()
        $errorOutput = $process.StandardError.ReadToEndAsync()
        # No approval answer is sent. The newline only dismisses the stale receipt.
        $process.StandardInput.WriteLine('')
        $process.StandardInput.Close()
        if (-not $process.WaitForExit(60000)) { throw 'Native review timed out' }
        $receipt = $output.GetAwaiter().GetResult() + $errorOutput.GetAwaiter().GetResult()
        if ($process.ExitCode -ne $ExitCode -or -not $receipt.Contains($Expected)) { throw 'Native review response did not match the expected refusal' }
    } finally {
        if ($started -and -not $process.HasExited) { $process.Kill($true); $process.WaitForExit() }
        $process.Dispose()
    }
}
$id = [guid]::NewGuid().ToString('N')
$uri = $scheme + '://request/' + $id
Invoke-ReviewProbe @($uri) 1 'haco: request is no longer pending'
Invoke-ReviewProbe @($uri + '?answer=yes') 2 'Invalid Hacocoon review link.'
Invoke-ReviewProbe @($uri, '--yes') 2 'requires one Windows notification link'

# A pre-existing directory with another owner must remain untouched.
$collisionName = 'Hacocoon-Review-Collision-' + [guid]::NewGuid().ToString('N').Substring(0,8)
$collisionScheme = Get-HacocoonReviewScheme $collisionName
$collisionDirectory = [IO.Path]::GetFullPath((Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) ('Hacocoon\review\' + $collisionScheme)))
if (Test-Path -LiteralPath $collisionDirectory) { throw 'Collision fixture already exists' }
[void][IO.Directory]::CreateDirectory($collisionDirectory)
$collisionFile = Join-Path $collisionDirectory 'review.json'
$bundle = $null
$foreign = '{"distribution":"Unrelated"}'
[IO.File]::WriteAllText($collisionFile, $foreign, [Text.UTF8Encoding]::new($false))
try {
    # Reuse only the already verified binary and make a private checksum fixture.
    $bundle = Join-Path ([IO.Path]::GetTempPath()) ('haco-review-check-' + [guid]::NewGuid().ToString('N'))
    [void][IO.Directory]::CreateDirectory($bundle)
    Copy-Item -LiteralPath $adapter -Destination (Join-Path $bundle 'haco-review.exe')
    $digest = (Get-FileHash -LiteralPath $adapter -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText((Join-Path $bundle 'checksums.txt'), ($digest + '  haco-review.exe' + "`n"), [Text.UTF8Encoding]::new($false))
    $refused = $false
    try { Install-HacocoonDesktopReview $collisionName $bundle } catch { $refused = $_.Exception.Message -ceq 'Existing review directory belongs to another distribution' }
    if (-not $refused -or (Get-Content -Raw -LiteralPath $collisionFile) -cne $foreign -or
        (Test-Path -LiteralPath ('HKCU:\Software\Classes\' + $collisionScheme)) -or
        @(Get-ChildItem -LiteralPath $collisionDirectory).Count -ne 1) { throw 'Foreign review ownership was not preserved' }
} finally {
    if ($bundle) {
        foreach ($leaf in @('haco-review.exe','checksums.txt')) {
            $path = Join-Path $bundle $leaf
            if (Test-Path -LiteralPath $path -PathType Leaf) { Remove-Item -LiteralPath $path }
        }
        [IO.Directory]::Delete($bundle, $false)
    }
    Remove-Item -LiteralPath $collisionFile
    [IO.Directory]::Delete($collisionDirectory, $false)
}
Write-Host 'WINDOWS REVIEW FOREIGN OWNERSHIP REFUSAL: PASS'
Write-Host 'INSTALLED NATIVE REVIEW / EXACT REGISTRATION / STALE AND MALFORMED REFUSAL: PASS'
Write-Host 'SKIP: this console fixture does not observe a human toast click or fresh UI decision'

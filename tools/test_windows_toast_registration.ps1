#Requires -Version 5.1
param([Parameter(Mandatory=$true)][string]$AdapterPath)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot '../scripts/windows-review.ps1')
if ((Get-HacocoonReviewClassID 'Hacocoon') -cne '{d2677f30-7bd6-897e-929f-9a455e17c0ca}' -or
    (Get-HacocoonReviewClassID 'Hacocoon') -cne (Get-HacocoonReviewClassID 'hacocoon')) { throw 'Go/installer COM identity mismatch' }
$name = 'Hacocoon-Toast-Test-' + [guid]::NewGuid().ToString('N').Substring(0,12)
$scheme = Get-HacocoonReviewScheme $name
$class = Get-HacocoonReviewClassID $name
$classes = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Software\Classes', $true)
$classPath = 'CLSID\' + $class
$appPath = 'AppUserModelId\' + $scheme
$localRoot = [IO.Path]::GetFullPath([Environment]::GetFolderPath('LocalApplicationData'))
$directory = [IO.Path]::GetFullPath((Join-Path $localRoot ('Hacocoon\review\' + $scheme)))
if (-not $directory.StartsWith(($localRoot.TrimEnd('\')+'\'), [StringComparison]::OrdinalIgnoreCase)) { throw 'Test path escaped user application directory' }
if (Test-Path -LiteralPath $directory) { throw 'Test directory already exists' }
foreach ($path in @($classPath,$appPath,$scheme)) {
    $existing = $classes.OpenSubKey($path)
    if ($null -ne $existing) { $existing.Dispose(); throw 'Test registration already exists' }
}
$bundle = Join-Path ([IO.Path]::GetTempPath()) ('haco-toast-bundle-' + [guid]::NewGuid().ToString('N'))
[void][IO.Directory]::CreateDirectory($bundle)
try {
    Copy-Item -LiteralPath $AdapterPath -Destination (Join-Path $bundle 'haco-review.exe')
    $digest = Get-HacocoonReviewFileHash (Join-Path $bundle 'haco-review.exe')
    [IO.File]::WriteAllText((Join-Path $bundle 'checksums.txt'), ($digest+'  haco-review.exe'+"`n"), [Text.UTF8Encoding]::new($false))
    $foreign = $classes.CreateSubKey($classPath)
    $foreign.SetValue('HacocoonDistribution','FixtureForeignOwner');$foreign.Dispose()
    $refused = $false
    try { Install-HacocoonDesktopReview $name $bundle } catch { $refused = $_.Exception.Message -ceq 'Notification activator is owned by another registration' }
    if (-not $refused -or (Test-Path -LiteralPath (Join-Path $directory 'haco-review.exe'))) { throw 'Foreign COM owner was replaced' }
    $foreign = $classes.OpenSubKey($classPath)
    if ($foreign.GetValue('HacocoonDistribution') -cne 'FixtureForeignOwner') { $foreign.Dispose(); throw 'Foreign fixture changed' }
    $foreign.Dispose();$classes.DeleteSubKey($classPath, $true)
    # Resume the exact ownership configuration left before the fallible registration.
    Install-HacocoonDesktopReview $name $bundle
    Install-HacocoonDesktopReview $name $bundle
    $target = Join-Path $directory 'haco-review.exe'
    $server = $classes.OpenSubKey($classPath+'\LocalServer32')
    if ($server.GetValue('') -cne ('"'+$target+'" --toast-server')) { $server.Dispose(); throw 'COM command differs' }
    $server.Dispose()
    $app = $classes.OpenSubKey($appPath,$true)
    if ($app.GetValue('CustomActivator') -ine $class) { $app.Dispose(); throw 'COM identity differs' }
    $app.SetValue('CustomActivator',[guid]::NewGuid().ToString('B'));$app.Dispose()
    $refused = $false
    try { Install-HacocoonDesktopReview $name $bundle } catch { $refused = $_.Exception.Message -ceq 'Notification identity activator differs' }
    if (-not $refused -or (Get-HacocoonReviewFileHash $target) -cne $digest) { throw 'Mismatched activator was silently adopted' }
    Write-Host 'PASS native COM registration, owned resume, idempotence, foreign owner and mismatched activator refusal'
} finally {
    # Exact fresh fixture keys only; no recursive registry or filesystem deletion.
    foreach ($path in @($scheme+'\shell\open\command',$scheme+'\shell\open',$scheme+'\shell',$scheme,$classPath+'\LocalServer32',$classPath,$appPath)) {
        $key = $classes.OpenSubKey($path)
        if ($null -ne $key) { $key.Dispose();$classes.DeleteSubKey($path,$true) }
    }
    $classes.Dispose()
    foreach ($root in @($directory,$bundle)) {
        foreach ($leaf in @('haco-review.exe','review.json','checksums.txt')) {
            $path = Join-Path $root $leaf
            if (Test-Path -LiteralPath $path -PathType Leaf) { [IO.File]::Delete($path) }
        }
        if (Test-Path -LiteralPath $root) { [IO.Directory]::Delete($root,$false) }
    }
}

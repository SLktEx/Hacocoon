# Optional Windows desktop adapter; no controller or approval authority is stored here.
function Get-HacocoonReviewScheme([string]$Name) {
    if ($Name -cnotmatch '^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$') { throw 'Invalid review distribution' }
    $hash = [Security.Cryptography.SHA256]::Create()
    try { $bytes = $hash.ComputeHash([Text.Encoding]::UTF8.GetBytes($Name.ToLowerInvariant())) }
    finally { $hash.Dispose() }
    return 'hacocoon-review-' + (($bytes[0..7] | ForEach-Object { $_.ToString('x2') }) -join '')
}
function Get-HacocoonReviewClassID([string]$Name) {
    [void](Get-HacocoonReviewScheme $Name)
    $hash = [Security.Cryptography.SHA256]::Create()
    try { $bytes = $hash.ComputeHash([Text.Encoding]::UTF8.GetBytes("Hacocoon.ToastCOM`0" + $Name.ToLowerInvariant())) }
    finally { $hash.Dispose() }
    $bytes[6] = ($bytes[6] -band 15) -bor 128
    $bytes[8] = ($bytes[8] -band 63) -bor 128
    $hex = (($bytes[0..15] | ForEach-Object { $_.ToString('x2') }) -join '')
    return ([guid]::ParseExact($hex, 'N')).ToString('B')
}
# Use .NET directly: the supported Windows PowerShell installer environment may
# not have the module that supplies Get-FileHash loaded or available.
function Get-HacocoonReviewFileHash([string]$Path) {
    $stream = [IO.File]::OpenRead($Path)
    try {
        $hash = [Security.Cryptography.SHA256]::Create()
        try { $bytes = $hash.ComputeHash($stream) }
        finally { $hash.Dispose() }
    } finally { $stream.Dispose() }
    return ([BitConverter]::ToString($bytes) -replace '-', '').ToLowerInvariant()
}
function Install-HacocoonDesktopReview([string]$Name, [string]$BundleRoot) {
    $scheme = Get-HacocoonReviewScheme $Name
    $source = Join-Path $BundleRoot 'haco-review.exe'
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { throw 'Missing Windows review adapter' }
    $checksumFile = Join-Path $BundleRoot 'checksums.txt'
    $matches = @(Get-Content -LiteralPath $checksumFile | Where-Object { $_ -cmatch '^[a-f0-9]{64}  haco-review.exe$' })
    if ($matches.Count -ne 1 -or (Get-HacocoonReviewFileHash $source) -cne $matches[0].Substring(0,64)) { throw 'Windows review adapter checksum mismatch' }
    $localRoot = [Environment]::GetFolderPath('LocalApplicationData')
    if ([string]::IsNullOrWhiteSpace($localRoot)) { throw 'Windows user application directory unavailable' }
    $directory = [IO.Path]::GetFullPath((Join-Path $localRoot ('Hacocoon\review\' + $scheme)))
    # Only this user's application directory and ordinary descendants are accepted.
    $cursor = $directory
    while ($cursor.Length -ge $localRoot.Length) {
        if ((Test-Path -LiteralPath $cursor) -and ((Get-Item -LiteralPath $cursor).Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Refusing linked review directory' }
        if ($cursor -eq $localRoot) { break }
        $cursor = [IO.Path]::GetDirectoryName($cursor)
    }
    $configuration = Join-Path $directory 'review.json'
    if (Test-Path -LiteralPath $directory) {
        if (-not (Test-Path -LiteralPath $configuration -PathType Leaf)) { throw 'Existing review directory has no ownership configuration' }
        if ((Get-Item -LiteralPath $configuration).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Refusing linked review configuration' }
        $owned = Get-Content -Raw -LiteralPath $configuration | ConvertFrom-Json
        if ($owned.distribution -ine $Name) { throw 'Existing review directory belongs to another distribution' }
    } else {
        [void][IO.Directory]::CreateDirectory($directory)
        # Durable exact ownership is recorded before another fallible installation step.
        $json = @{ distribution = $Name } | ConvertTo-Json -Compress
        [IO.File]::WriteAllText($configuration, $json, [Text.UTF8Encoding]::new($false))
    }
    $target = Join-Path $directory 'haco-review.exe'
    if ((Test-Path -LiteralPath $target) -and ((Get-Item -LiteralPath $target).Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Refusing linked review executable' }
    $command = '"' + $target + '" "%1"'
    $registry = 'HKCU:\Software\Classes\' + $scheme
    if (Test-Path -LiteralPath $registry) {
        $existing = Get-ItemProperty -LiteralPath $registry
        if ($existing.HacocoonDistribution -ine $Name) { throw 'Review protocol is owned by another registration' }
        $existingCommand = (Get-Item -LiteralPath ($registry + '\shell\open\command')).GetValue('')
        if ($existingCommand -cne $command) { throw 'Review protocol command differs; inspect the existing registration' }
    }
    $appID = 'HKCU:\Software\Classes\AppUserModelId\' + $scheme
    $class = Get-HacocoonReviewClassID $Name
    $classKey = 'HKCU:\Software\Classes\CLSID\' + $class
    $serverCommand = '"' + $target + '" --toast-server'
    if (Test-Path -LiteralPath $classKey) {
        $classOwner = Get-ItemProperty -LiteralPath $classKey
        if ($classOwner.HacocoonDistribution -ine $Name) { throw 'Notification activator is owned by another registration' }
        $serverKey = $classKey + '\LocalServer32'
        if ((Test-Path -LiteralPath $serverKey) -and (Get-Item -LiteralPath $serverKey).GetValue('') -cne $serverCommand) { throw 'Notification activator command differs' }
    }
    if (Test-Path -LiteralPath $appID) {
        $owner = Get-ItemProperty -LiteralPath $appID
        if ($owner.HacocoonDistribution -ine $Name) { throw 'Notification identity is owned by another registration' }
        $activator = (Get-Item -LiteralPath $appID).GetValue('CustomActivator', $null)
        if ($null -ne $activator -and $activator -ine $class) { throw 'Notification identity activator differs' }
    }
    $temporary = Join-Path $directory ('adapter-' + [guid]::NewGuid().ToString('N') + '.tmp')
    try {
        Copy-Item -LiteralPath $source -Destination $temporary
        Move-Item -LiteralPath $temporary -Destination $target -Force
    } finally {
        if (Test-Path -LiteralPath $temporary -PathType Leaf) { Remove-Item -LiteralPath $temporary }
    }
    [void](New-Item -Path ($registry + '\shell\open\command') -Force)
    Set-Item -LiteralPath $registry -Value 'URL:Hacocoon approval review'
    [void](New-ItemProperty -LiteralPath $registry -Name 'URL Protocol' -Value '' -PropertyType String -Force)
    [void](New-ItemProperty -LiteralPath $registry -Name 'HacocoonDistribution' -Value $Name -PropertyType String -Force)
    Set-Item -LiteralPath ($registry + '\shell\open\command') -Value $command
    $appID = 'HKCU:\Software\Classes\AppUserModelId\' + $scheme
    [void](New-Item -Path $appID -Force)
    [void](New-ItemProperty -LiteralPath $appID -Name 'HacocoonDistribution' -Value $Name -PropertyType String -Force)
    [void](New-ItemProperty -LiteralPath $appID -Name 'DisplayName' -Value ('Hacocoon (' + $Name + ')') -PropertyType String -Force)
    # Persist ownership before publishing the launch target. A partial owned
    # registration can be resumed; a foreign registration is never overwritten.
    [void](New-Item -Path $classKey -Force)
    [void](New-ItemProperty -LiteralPath $classKey -Name 'HacocoonDistribution' -Value $Name -PropertyType String -Force)
    [void](New-Item -Path ($classKey + '\LocalServer32') -Force)
    Set-Item -LiteralPath ($classKey + '\LocalServer32') -Value $serverCommand
    [void](New-ItemProperty -LiteralPath $appID -Name 'CustomActivator' -Value $class -PropertyType String -Force)
    Write-Host 'Windows notification review registered for this Hacocoon distribution.'
}

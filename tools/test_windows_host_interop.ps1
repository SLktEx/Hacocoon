#Requires -Version 7.0
param([string]$Distro='Hacocoon', [switch]$RequireNonC, [string]$PersistenceManifest)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest

function Invoke-InteropProcess([string[]]$Arguments) {
    $start=[Diagnostics.ProcessStartInfo]::new('wsl.exe')
    $start.UseShellExecute=$false
    $start.RedirectStandardOutput=$true
    $start.RedirectStandardError=$true
    foreach($argument in (@('-d',$Distro,'-u','root','--exec')+$Arguments)){[void]$start.ArgumentList.Add($argument)}
    $process=[Diagnostics.Process]::Start($start)
    $stdout=$process.StandardOutput.ReadToEndAsync()
    $stderr=$process.StandardError.ReadToEndAsync()
    $process.WaitForExit()
    [pscustomobject]@{Code=$process.ExitCode;Out=$stdout.GetAwaiter().GetResult();Err=$stderr.GetAwaiter().GetResult()}
}
function Invoke-InteropGuest([string[]]$Arguments) {
    Invoke-InteropProcess (@('incus','exec','haco-host','--project','hacocoon','--')+$Arguments)
}
function Assert-InteropSuccess($Result,[string]$Description) {
    if($Result.Code -ne 0){throw "$Description failed: $($Result.Code) $($Result.Err)"}
    $Result.Out
}

$inventory=Invoke-InteropProcess @('findmnt','--json','--list','-o','TARGET,FSTYPE,OPTIONS')
$mounts=(Assert-InteropSuccess $inventory 'Read actual WSL mounts' | ConvertFrom-Json).filesystems
$drives=@($mounts | Where-Object {$_.target -match '^/mnt/[a-z]$' -and ($_.fstype -eq 'drvfs' -or ($_.fstype -eq '9p' -and $_.options.Contains('aname=drvfs;')))} | Select-Object -ExpandProperty target -Unique)
if($drives -notcontains '/mnt/c'){throw 'Required C drive is not available via WSL'}
if($RequireNonC -and @($drives | Where-Object {$_ -ne '/mnt/c'}).Count -eq 0){throw 'Required non-C drive is unavailable'}
Write-Host "Detected actual DrvFs drives: $($drives -join ', ')"

$cmd=Invoke-InteropGuest @('/bin/bash','-lc','cd /mnt/c && /mnt/c/Windows/System32/cmd.exe /c ver')
[void](Assert-InteropSuccess $cmd 'Direct absolute cmd.exe, without /init')
if($cmd.Out -notmatch 'Windows'){throw 'cmd.exe stdout did not contain Windows version'}

foreach($drive in $drives){
    $name='Hacocoon-Acceptance-'+[guid]::NewGuid().ToString('N')
    $windowsRoot=$drive.Substring($drive.Length-1).ToUpperInvariant()+':\'
    $windowsDirectory=Join-Path $windowsRoot $name
    $guestDirectory="$drive/$name"
    [void][IO.Directory]::CreateDirectory($windowsDirectory)
    try {
        [IO.File]::WriteAllText((Join-Path $windowsDirectory 'from-windows.txt'),'windows-to-host')
        $read=Assert-InteropSuccess (Invoke-InteropGuest @('cat',"$guestDirectory/from-windows.txt")) "$drive Windows-to-Host read"
        if($read -ne 'windows-to-host'){throw "Windows file mismatch on $drive"}
        [void](Assert-InteropSuccess (Invoke-InteropGuest @('/bin/sh','-c','printf host-to-windows > "$1/from-host.txt"','sh',$guestDirectory)) "$drive Host write")
        if([IO.File]::ReadAllText((Join-Path $windowsDirectory 'from-host.txt')) -ne 'host-to-windows'){throw "Host file mismatch on $drive"}
        $again=Assert-InteropSuccess (Invoke-InteropGuest @('cat',"$guestDirectory/from-host.txt")) "$drive Host readback"
        if($again -ne 'host-to-windows'){throw 'Host readback mismatch'}
        $argumentScript=Join-Path $windowsDirectory 'arguments with spaces.txt'
        [IO.File]::WriteAllText($argumentScript,"separate argument with spaces`nseparate wrong`n")
        $argumentResult=Invoke-InteropGuest @('findstr.exe','/L','/C:separate argument with spaces',$argumentScript)
        if($argumentResult.Code -ne 0 -or $argumentResult.Out.Trim() -ne 'separate argument with spaces'){throw "Native argv spacing failed on ${drive}: exit=$($argumentResult.Code); stdout=$($argumentResult.Out); stderr=$($argumentResult.Err)"}
        Write-Host "$drive native executable spaced path/argument: PASS"
        Write-Host "$drive read/write in both directions: PASS"
    } finally {
        foreach($file in @('from-windows.txt','from-host.txt','arguments with spaces.txt')) {
            $path=Join-Path $windowsDirectory $file
            if(Test-Path -LiteralPath $path){Remove-Item -LiteralPath $path}
        }
        # Only remove our now-empty directory; do not recurse into unexpected data.
        if(Test-Path -LiteralPath $windowsDirectory){[IO.Directory]::Delete($windowsDirectory,$false)}
    }
}

[void](Assert-InteropSuccess (Invoke-InteropGuest @('/bin/bash','-lc','cmd.exe /c ver')) 'cmd.exe via Windows PATH')
$power=Invoke-InteropGuest @('/bin/bash','-lc',"powershell.exe -NoProfile -NonInteractive -Command `"[Console]::Out.WriteLine('hello'); [Console]::Error.WriteLine('stderr-marker'); exit 23`"")
if($power.Code -ne 23 -or $power.Out.Trim() -ne 'hello' -or $power.Err -notmatch 'stderr-marker'){throw "PowerShell output/exit mismatch: $($power.Code) $($power.Out) $($power.Err)"}
$spaces=Invoke-InteropGuest @('/bin/bash','-lc',"powershell.exe -NoProfile -NonInteractive -Command `"[Console]::Out.WriteLine('argument with spaces')`"")
if((Assert-InteropSuccess $spaces 'Spaced argument').Trim() -ne 'argument with spaces'){throw 'Spaced argument mismatch'}
$pathTool=Invoke-InteropGuest @('/bin/bash','-lc','where.exe cmd.exe')
if((Assert-InteropSuccess $pathTool 'Windows PATH executable where.exe') -notmatch 'cmd.exe'){throw 'where.exe output mismatch'}
Write-Host 'Direct .exe / Windows PATH / stdout / stderr / exit 23 / spaces: PASS'
if ($PersistenceManifest) {
    foreach ($record in (Get-Content -Raw -LiteralPath $PersistenceManifest | ConvertFrom-Json)) {
        if ($record.guest -notmatch '^/mnt/[a-z]/Hacocoon-Persistence-[a-f0-9]{32}$') { throw 'Invalid persistence test path' }
        $driveRoot=$record.guest.Substring(5,1).ToUpperInvariant()+':\'
        $expectedWindows=Join-Path $driveRoot ($record.guest.Split('/')[-1])
        if ($record.windows -ne $expectedWindows) { throw 'Persistence paths do not identify the same Windows directory' }
        if ([IO.File]::ReadAllText((Join-Path $expectedWindows 'host.txt')) -ne 'retained-host-marker') { throw 'Windows persistence read mismatch' }
        $retained=Assert-InteropSuccess (Invoke-InteropGuest @('cat',"$($record.guest)/windows.txt")) 'Retained Windows file read from Host'
        if ($retained -ne 'retained-windows-marker') { throw 'Host persistence read mismatch' }
        Write-Host "$driveRoot pre-restart files retained in both directions: PASS"
    }
}
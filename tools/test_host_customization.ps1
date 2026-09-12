#Requires -Version 7.0
param([string]$Distro = 'Hacocoon')
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ($env:GITHUB_ACTIONS -ne 'true' -or -not $env:RUNNER_TEMP -or $Distro -ne 'Hacocoon') {
    throw 'Host recipe acceptance requires the disposable GHA installation.'
}
$nonce = [guid]::NewGuid().ToString('N')
$work = Join-Path $env:RUNNER_TEMP ('haco-host-recipe-' + $nonce)
[void][IO.Directory]::CreateDirectory($work)
$recipe = Join-Path $work 'setup.sh'
$full = [IO.Path]::GetFullPath($recipe)
if ($full -notmatch '^([A-Za-z]):\\') { throw 'Expected a projected Windows drive.' }
$projected = '/mnt/' + $Matches[1].ToLowerInvariant() + '/' + $full.Substring(3).Replace('\','/')
$marker = '/tmp/haco-host-recipe-' + $nonce
function Invoke-HostRecipeCommand([string[]]$Arguments) {
    $output = & wsl.exe -d $Distro -u root --exec incus exec haco-host --project hacocoon -- @Arguments 2>&1
    if ($LASTEXITCODE -ne 0) { throw 'Host recipe acceptance command failed.' }
    return ($output -join [Environment]::NewLine).Trim()
}
function Write-Recipe([string]$Value) {
    $unitCheck = 'case "$(< /proc/$$/cgroup)" in *hacocoon-user-setup.service*) ;; *) exit 42;; esac'
    [IO.File]::WriteAllText($recipe, ($unitCheck + [Environment]::NewLine + "printf '%s' '$Value' > '$marker'" + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
}
try {
    # Product-driving actions use only the installed, ordinary haco setup.
    Write-Recipe $nonce
    [void](Invoke-HostRecipeCommand @('/usr/local/bin/haco','setup','--script',$projected))
    if ((Invoke-HostRecipeCommand @('/bin/cat',$marker)) -ne $nonce) { throw 'Initial Host recipe did not execute.' }

    Write-Recipe ($nonce + '-updated')
    # Reset only our owned observation marker to prove saved-content replay.
    [void](Invoke-HostRecipeCommand @('/bin/rm','--',$marker))
    [void](Invoke-HostRecipeCommand @('/usr/local/bin/haco','setup'))
    if ((Invoke-HostRecipeCommand @('/bin/cat',$marker)) -ne $nonce) { throw 'Replay did not use the saved snapshot.' }

    [void](Invoke-HostRecipeCommand @('/usr/local/bin/haco','setup','--script',$projected))
    if ((Invoke-HostRecipeCommand @('/bin/cat',$marker)) -ne ($nonce + '-updated')) { throw 'Updated recipe did not execute.' }

    [void](Invoke-HostRecipeCommand @('/usr/local/bin/haco','setup','--clear-script'))
    [void](Invoke-HostRecipeCommand @('/bin/rm','--',$marker))
    [void](Invoke-HostRecipeCommand @('/usr/local/bin/haco','setup'))
    [void](Invoke-HostRecipeCommand @('/usr/bin/test','!','-e',$marker))
    Write-Host 'HOST CUSTOMIZATION SAVE / REPLAY / UPDATE / CLEAR: PASS'
} finally {
    [void](Invoke-HostRecipeCommand @('/usr/local/bin/haco','setup','--clear-script'))
    [void](Invoke-HostRecipeCommand @('/bin/rm','-f','--',$marker))
}

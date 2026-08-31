[CmdletBinding()]
param(
    [string]$InstanceName = "Hacocoon",
    [string]$BaseDistro = "Ubuntu-26.04",
    [string]$HacocoonVersion = "latest",
    [switch]$WebDownload,
    [switch]$SkipIncus,
    [switch]$GrantIncusAdmin
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$installer = Join-Path $PSScriptRoot "install-windows.ps1"
if (-not (Test-Path -LiteralPath $installer -PathType Leaf)) {
    throw "install-windows.ps1 is missing next to bootstrap-windows.ps1."
}

$arguments = @{
    InstanceName = $InstanceName
    BaseDistro = $BaseDistro
    HacocoonVersion = $HacocoonVersion
}
if ($WebDownload) { $arguments.WebDownload = $true }
if ($SkipIncus) { $arguments.SkipIncus = $true }
if ($GrantIncusAdmin) { $arguments.GrantIncusAdmin = $true }

& $installer @arguments

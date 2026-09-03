from pathlib import Path

root = Path(__file__).resolve().parents[1]
installer_path = root / "scripts" / "install-windows.ps1"
test_path = root / "tools" / "test_installer_packages.py"

installer = installer_path.read_text(encoding="utf-8")

start = installer.index("function Invoke-WslCaptureWithInput")
end = installer.index("function Invoke-WslRootShellScript", start)
installer = installer[:start] + installer[end:]

start = installer.index("function Write-WslUtf8File")
end = installer.index("function Get-SudoersPolicyFiles", start)
new_write = '''function Write-WslUtf8File([string]$Name, [string]$Path, [string]$Content, [switch]$Append) {
    # Never send installer-controlled bytes through the Windows native stdin
    # pipeline. Windows PowerShell 5.1 can change encoding/preambles there.
    # Base64 is argv-safe and decoded entirely inside WSL.
    $normalized = ($Content -replace "`r`n", "`n") -replace "`r", "`n"
    $encoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($normalized))
    $script = if ($Append) {
        'printf ''%s'' "$1" | base64 -d >> "$2"'
    } else {
        'printf ''%s'' "$1" | base64 -d > "$2"'
    }
    return Invoke-WslCapture @(
        "--distribution", $Name,
        "--user", "root",
        "--exec", "sh", "-eu", "-c", $script, "sh", $encoded, $Path
    )
}

'''
installer = installer[:start] + new_write + installer[end:]

old_main = '''try {
    Enable-BootstrapSudo $InstanceName $loginUser
    Write-Step "Running common Ubuntu install.sh inside '$InstanceName'"
    & wsl.exe --distribution $InstanceName --user $loginUser --exec env `
        "HACO_BUNDLE_ROOT=$linuxAssetRoot" `
        "HACO_BOOTSTRAP_SKIP_INCUS=$skipIncusValue" `
        "HACO_BOOTSTRAP_GRANT_INCUS_ADMIN=$grantIncusAdminValue" `
        "HACO_REQUIRE_PROVENANCE=$requireProvenance" `
        sh "$linuxAssetRoot/install.sh" $resolvedVersion
    if ($LASTEXITCODE -ne 0) {
        throw "Common Hacocoon Ubuntu installation failed inside WSL."
    }
} finally {
    Disable-BootstrapSudo $InstanceName
}
'''
new_main = '''$mainFailure = $null
try {
    Enable-BootstrapSudo $InstanceName $loginUser
    Write-Step "Running common Ubuntu install.sh inside '$InstanceName'"
    & wsl.exe --distribution $InstanceName --user $loginUser --exec env `
        "HACO_BUNDLE_ROOT=$linuxAssetRoot" `
        "HACO_BOOTSTRAP_SKIP_INCUS=$skipIncusValue" `
        "HACO_BOOTSTRAP_GRANT_INCUS_ADMIN=$grantIncusAdminValue" `
        "HACO_REQUIRE_PROVENANCE=$requireProvenance" `
        sh "$linuxAssetRoot/install.sh" $resolvedVersion
    if ($LASTEXITCODE -ne 0) {
        throw "Common Hacocoon Ubuntu installation failed inside WSL."
    }
} catch {
    $mainFailure = $_
    throw
} finally {
    try {
        Disable-BootstrapSudo $InstanceName
    } catch {
        if ($null -eq $mainFailure) { throw }
        Write-Warning "Bootstrap sudo cleanup also failed after the installer error: $($_.Exception.Message)"
    }
}
'''
if old_main not in installer:
    raise SystemExit("main bootstrap try/finally block not found")
installer = installer.replace(old_main, new_main, 1)
installer_path.write_text(installer, encoding="utf-8", newline="\n")

test = test_path.read_text(encoding="utf-8")
for old in (
    "                '$OutputEncoding = [Text.UTF8Encoding]::new($false)',\n",
    "                '$wrappedArguments = @($Arguments[0..$execIndex])',\n",
    "                'LC_ALL=C sed ',\n",
):
    if old not in test:
        raise SystemExit(f"old stdin contract marker missing: {old!r}")
    test = test.replace(old, "", 1)

needle = '''                'function Invoke-WslRootShellScript',
                'sh -eu "$tmp" "$@"',
'''
replacement = '''                'function Invoke-WslRootShellScript',
                'sh -eu "$tmp" "$@"',
                'Never send installer-controlled bytes through the Windows native stdin',
                'base64 -d >> "$2"',
                '"sh", $encoded, $Path',
                '$mainFailure = $null',
                'Bootstrap sudo cleanup also failed after the installer error',
'''
if needle not in test:
    raise SystemExit("required contract marker insertion point not found")
test = test.replace(needle, replacement, 1)

needle = '''                '$normalized | & wsl.exe @Arguments',
                '"--exec", "sh", "-s"',
'''
replacement = '''                '$normalized | & wsl.exe @Arguments',
                '"--exec", "sh", "-s"',
                'function Invoke-WslCaptureWithInput',
'''
if needle not in test:
    raise SystemExit("forbidden contract marker insertion point not found")
test = test.replace(needle, replacement, 1)
test_path.write_text(test, encoding="utf-8", newline="\n")

from pathlib import Path

root = Path(__file__).resolve().parents[1]
installer_path = root / "scripts" / "install-windows.ps1"
test_path = root / "tools" / "test_installer_packages.py"

installer = installer_path.read_text(encoding="utf-8")

start = installer.index("function Get-SudoersPolicyFiles")
end = installer.index("function Remove-LegacyHacocoonSudoDropIn", start)
new_policy_helpers = r'''function Get-SudoersPolicyFile([string]$Name) {
    # Ubuntu 26.04 sudo-rs prefers /etc/sudoers-rs when it exists and otherwise
    # falls back to /etc/sudoers. Manage only that effective policy instead of
    # mutating both files and creating two sources of Hacocoon state.
    foreach ($policy in @("/etc/sudoers-rs", "/etc/sudoers")) {
        $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "test", "-f", $policy)
        if ($probe.ExitCode -eq 0) { return $policy }
        if ($probe.ExitCode -ne 1) {
            throw "Failed to inspect sudo policy '$policy': $($probe.Stderr)"
        }
    }
    throw "No supported sudo policy file exists in '$Name'."
}

function Remove-HacocoonSudoPolicyBlock([string]$Name, [string]$MarkerName) {
    Assert-SafeName $MarkerName "sudo policy marker"
    $policy = Get-SudoersPolicyFile $Name
    $script = @'
set -eu
policy="$1"
marker_name="$2"
start="# BEGIN HACOCOON $marker_name"
end="# END HACOCOON $marker_name"
[ -f "$policy" ] || exit 0
grep -Fxq "$start" "$policy" || exit 0
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
# Hacocoon blocks are always appended. Strip every complete block and also
# recover an interrupted Hacocoon block at EOF. An unmatched END is not ours
# to guess about, so fail closed in that case.
awk -v start="$start" -v end="$end" '
  BEGIN { skip=0 }
  $0 == start { skip=1; next }
  $0 == end {
    if (!skip) exit 42
    skip=0
    next
  }
  !skip { print }
' "$policy" > "$tmp"
/usr/sbin/visudo -cf "$tmp" >/dev/null
install -o root -g root -m 0440 "$tmp" "$policy"
'@
    $probe = Invoke-WslRootShellScript $Name $script @($policy, $MarkerName)
    if ($probe.ExitCode -ne 0) {
        throw "Failed to remove Hacocoon sudo block '$MarkerName' from '$policy' (exit $($probe.ExitCode)): $($probe.Stderr)"
    }
}

'''
installer = installer[:start] + new_policy_helpers + installer[end:]

start = installer.index("function Set-HacocoonSudoPolicyBlock")
end = installer.index("function Add-HacocoonBootstrapSudoRule", start)
new_set = r'''function Set-HacocoonSudoPolicyBlock([string]$Name, [string]$MarkerName, [string]$Rule) {
    Assert-SafeName $MarkerName "sudo policy marker"
    $policy = Get-SudoersPolicyFile $Name
    Remove-HacocoonSudoPolicyBlock $Name $MarkerName
    $encodedRule = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($Rule))
    $script = @'
set -eu
policy="$1"
marker_name="$2"
rule_b64="$3"
start="# BEGIN HACOCOON $marker_name"
end="# END HACOCOON $marker_name"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat "$policy" > "$tmp"
printf '\n%s\n' "$start" >> "$tmp"
printf '%s' "$rule_b64" | base64 -d >> "$tmp"
printf '\n%s\n' "$end" >> "$tmp"
# Never expose a partially-written sudo policy. Validate the complete candidate
# first, then replace the active policy in one install(1) operation.
/usr/sbin/visudo -cf "$tmp" >/dev/null
install -o root -g root -m 0440 "$tmp" "$policy"
'@
    $probe = Invoke-WslRootShellScript $Name $script @($policy, $MarkerName, $encodedRule)
    if ($probe.ExitCode -ne 0) {
        throw "Failed to install Hacocoon sudo block '$MarkerName' in '$policy' atomically (exit $($probe.ExitCode)): $($probe.Stderr)"
    }
    return $policy
}

'''
installer = installer[:start] + new_set + installer[end:]

old_main = '''$mainFailure = $null
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
} finally {
    try {
        Disable-BootstrapSudo $InstanceName
    } catch {
        if ($null -eq $mainFailure) { throw }
        Write-Warning "Bootstrap sudo cleanup also failed after the installer error: $($_.Exception.Message)"
    }
}
if ($null -ne $mainFailure) {
    throw $mainFailure
}
'''
if old_main not in installer:
    raise SystemExit("main bootstrap error-preservation block not found")
installer = installer.replace(old_main, new_main, 1)
installer_path.write_text(installer, encoding="utf-8", newline="\n")

test = test_path.read_text(encoding="utf-8")
test = test.replace("                'Get-SudoersPolicyFiles',\n", "                'Get-SudoersPolicyFile',\n", 1)
test = test.replace("                '@(\"/etc/sudoers-rs\", \"/etc/sudoers\")',\n", "                'foreach ($policy in @(\"/etc/sudoers-rs\", \"/etc/sudoers\"))',\n", 1)
needle = "                'Bootstrap sudo cleanup also failed after the installer error',\n"
replacement = needle + "                '/usr/sbin/visudo -cf \"$tmp\"',\n                'install -o root -g root -m 0440 \"$tmp\" \"$policy\"',\n                'throw $mainFailure',\n"
if needle not in test:
    raise SystemExit("required marker insertion point missing")
test = test.replace(needle, replacement, 1)
needle = "                'function Invoke-WslCaptureWithInput',\n"
replacement = needle + "                'Get-SudoersPolicyFiles',\n                'Write-WslUtf8File $Name $policy $block -Append',\n"
if needle not in test:
    raise SystemExit("forbidden marker insertion point missing")
test = test.replace(needle, replacement, 1)
test_path.write_text(test, encoding="utf-8", newline="\n")

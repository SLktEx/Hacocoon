from pathlib import Path
import re

installer_path = Path("scripts/install-windows.ps1")
installer = installer_path.read_text(encoding="utf-8")
installer = installer.replace('$BootstrapSudoersPath = "/etc/sudoers.d/hacocoon-bootstrap"\n', '')

replacement = r'''function Get-SudoersPolicyFiles([string]$Name) {
    $policies = @()
    foreach ($policy in @("/etc/sudoers-rs", "/etc/sudoers")) {
        $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "test", "-f", $policy)
        if ($probe.ExitCode -eq 0) {
            $policies += $policy
        } elseif ($probe.ExitCode -ne 1) {
            throw "Failed to inspect sudo policy '$policy': $($probe.Stderr)"
        }
    }
    if ($policies.Count -eq 0) {
        throw "No supported sudo policy file exists in '$Name'."
    }
    return $policies
}

function Remove-HacocoonBootstrapSudoRule([string]$Name) {
    $markerStart = "# BEGIN HACOCOON BOOTSTRAP"
    $markerEnd = "# END HACOCOON BOOTSTRAP"
    $script = @'
set -eu
policy="$1"
start="$2"
end="$3"
[ -f "$policy" ] || exit 0
grep -Fxq "$start" "$policy" || exit 0
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
awk -v start="$start" -v end="$end" '
  BEGIN { skip=0; seen=0 }
  $0 == start {
    if (skip || seen) exit 41
    skip=1
    seen=1
    next
  }
  $0 == end {
    if (!skip) exit 42
    skip=0
    next
  }
  !skip { print }
  END { if (skip) exit 43 }
' "$policy" > "$tmp"
install -o root -g root -m 0440 "$tmp" "$policy"
'@
    foreach ($policy in @("/etc/sudoers-rs", "/etc/sudoers")) {
        $probe = Invoke-WslCaptureWithInput @(
            "--distribution", $Name,
            "--user", "root",
            "--exec", "sh", "-s", "--", $policy, $markerStart, $markerEnd
        ) $script
        if ($probe.ExitCode -ne 0) {
            throw "Failed to remove Hacocoon bootstrap sudo block from '$policy': $($probe.Stderr)"
        }
    }
}

function Add-HacocoonBootstrapSudoRule([string]$Name, [string]$LoginUser) {
    $markerStart = "# BEGIN HACOCOON BOOTSTRAP"
    $markerEnd = "# END HACOCOON BOOTSTRAP"
    $rule = "`n$markerStart`n$LoginUser ALL=(ALL:ALL) NOPASSWD: ALL`n$markerEnd`n"
    $policies = @(Get-SudoersPolicyFiles $Name)

    # Recover safely from a previously interrupted installer before granting
    # the temporary broad bootstrap capability again.
    Remove-HacocoonBootstrapSudoRule $Name

    foreach ($policy in $policies) {
        $probe = Write-WslUtf8File $Name $policy $rule -Append
        if ($probe.ExitCode -ne 0) {
            throw "Failed to append temporary bootstrap sudo rule to '$policy': $($probe.Stderr)"
        }
        $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "/usr/sbin/visudo", "-cf", $policy)
        if ($probe.ExitCode -ne 0) {
            throw "Sudo policy '$policy' rejected the temporary Hacocoon bootstrap rule: $($probe.Stderr)"
        }
    }
    return ($policies -join ", ")
}

'''
start_marker = "function Get-SudoersPolicyFiles([string]$Name) {"
end_marker = "function Get-InstalledDistros {"
start = installer.find(start_marker)
end = installer.find(end_marker, start)
if start < 0 or end < 0 or end <= start:
    raise SystemExit(f"unable to locate sudo policy function range: start={start}, end={end}")
installer = installer[:start] + replacement + installer[end:]

old_enable = r'''    # install.sh intentionally runs as the ordinary workspace owner. Give that
    # user temporary passwordless sudo only while the trusted installer runs;
    # the rule is removed in finally and replaced by the narrow haco-host rule.
    $sudoers = "$LoginUser ALL=NOPASSWD: ALL`n"
    $probe = Write-WslUtf8File $Name $BootstrapSudoersPath $sudoers
    if ($probe.ExitCode -ne 0) { throw "Failed to write temporary installer sudo rule: $($probe.Stderr)" }
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "chmod", "0440", $BootstrapSudoersPath)
    if ($probe.ExitCode -ne 0) { throw "Failed to protect temporary installer sudo rule." }
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "/usr/sbin/visudo", "-cf", $BootstrapSudoersPath)
    if ($probe.ExitCode -ne 0) { throw "Failed to validate temporary installer sudo rule: $($probe.Stderr)" }
    $policySet = Ensure-HacocoonSudoRuleLoaded $Name $BootstrapSudoersPath
    Write-Step "Validating temporary sudo rule through policy candidates: $policySet"
'''
new_enable = r'''    # install.sh intentionally runs as the ordinary workspace owner. Give that
    # user temporary passwordless sudo only while the trusted installer runs.
    # Write the marked rule directly into each supported policy file instead of
    # depending on distro-specific sudoers.d include behavior, then prove the
    # effective policy with a real non-interactive sudo command.
    $policySet = Add-HacocoonBootstrapSudoRule $Name $LoginUser
    Write-Step "Validating temporary sudo rule through policy files: $policySet"
'''
if installer.count(old_enable) != 1:
    raise SystemExit(f"expected one old Enable-BootstrapSudo body, found {installer.count(old_enable)}")
installer = installer.replace(old_enable, new_enable)
installer = installer.replace(
    "after loading policy candidates '$policySet'",
    "after updating policy files '$policySet'",
)
old_disable = r'''function Disable-BootstrapSudo([string]$Name) {
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "rm", "-f", $BootstrapSudoersPath)
    if ($probe.ExitCode -ne 0) {
        throw "Failed to remove temporary installer sudo rule."
    }
    Remove-HacocoonSudoRuleInclude $Name $BootstrapSudoersPath
}'''
new_disable = r'''function Disable-BootstrapSudo([string]$Name) {
    Remove-HacocoonBootstrapSudoRule $Name
}'''
if installer.count(old_disable) != 1:
    raise SystemExit(f"expected one old Disable-BootstrapSudo function, found {installer.count(old_disable)}")
installer = installer.replace(old_disable, new_disable)

for forbidden in (
    "BootstrapSudoersPath",
    "Ensure-HacocoonSudoRuleLoaded",
    "Remove-HacocoonSudoRuleInclude",
    "@include $RulePath",
):
    if forbidden in installer:
        raise SystemExit(f"old sudo drop-in mechanism survived patch: {forbidden}")
installer_path.write_text(installer, encoding="utf-8")

# Strengthen the Windows E2E cleanup assertion: the old drop-in must be absent
# and neither policy file may retain the temporary direct-policy marker.
workflow_path = Path(".github/workflows/windows-all-scripts-e2e.yml")
workflow = workflow_path.read_text(encoding="utf-8")
old = '''          & wsl.exe --distribution $instance --user root --exec test ! -e /etc/sudoers.d/hacocoon-bootstrap
          if ($LASTEXITCODE -ne 0) { throw "Temporary bootstrap sudo rule leaked after installer completion." }
'''
new = '''          & wsl.exe --distribution $instance --user root --exec test ! -e /etc/sudoers.d/hacocoon-bootstrap
          if ($LASTEXITCODE -ne 0) { throw "Legacy temporary bootstrap sudo drop-in leaked after installer completion." }
          & wsl.exe --distribution $instance --user root --exec sh -eu -c 'for policy in /etc/sudoers-rs /etc/sudoers; do [ ! -f "$policy" ] || ! grep -Fq "# BEGIN HACOCOON BOOTSTRAP" "$policy"; done'
          if ($LASTEXITCODE -ne 0) { throw "Temporary bootstrap sudo policy block leaked after installer completion." }
'''
if workflow.count(old) != 1:
    raise SystemExit(f"expected one old E2E sudo cleanup assertion, found {workflow.count(old)}")
workflow_path.write_text(workflow.replace(old, new), encoding="utf-8")

# Keep the package-level contract focused on direct, marked policy mutation and
# forbid regression to provider guessing or sudoers.d bootstrap indirection.
test_path = Path("tools/test_installer_packages.py")
test = test_path.read_text(encoding="utf-8")
old = '''                'Get-SudoersPolicyFiles',
                '@(\"/etc/sudoers-rs\", \"/etc/sudoers\")',
                'Validating temporary sudo rule through policy candidates',
                '/etc/sudoers-rs',
                'Ensure-HacocoonSudoRuleLoaded',
                'Remove-HacocoonSudoRuleInclude',
                '@include $RulePath',
                '$LoginUser ALL=NOPASSWD: ALL','''
new = '''                'Get-SudoersPolicyFiles',
                '@(\"/etc/sudoers-rs\", \"/etc/sudoers\")',
                'Add-HacocoonBootstrapSudoRule',
                'Remove-HacocoonBootstrapSudoRule',
                '# BEGIN HACOCOON BOOTSTRAP',
                '$LoginUser ALL=(ALL:ALL) NOPASSWD: ALL',
                'Validating temporary sudo rule through policy files','''
if test.count(old) != 1:
    raise SystemExit(f"expected one old package contract block, found {test.count(old)}")
test = test.replace(old, new)
needle = '''                '\"update-alternatives\"',
            ):'''
replacement_forbidden = '''                '\"update-alternatives\"',
                '@include $RulePath',
                '/etc/sudoers.d/hacocoon-bootstrap',
            ):'''
if test.count(needle) != 1:
    raise SystemExit(f"expected one forbidden provider tuple tail, found {test.count(needle)}")
test = test.replace(needle, replacement_forbidden)
test_path.write_text(test, encoding="utf-8")

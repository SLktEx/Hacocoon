from pathlib import Path

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

function Remove-HacocoonSudoPolicyBlock([string]$Name, [string]$MarkerName) {
    $markerStart = "# BEGIN HACOCOON $MarkerName"
    $markerEnd = "# END HACOCOON $MarkerName"
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
            throw "Failed to remove Hacocoon sudo block '$MarkerName' from '$policy': $($probe.Stderr)"
        }
    }
}

function Remove-LegacyHacocoonSudoDropIn([string]$Name, [string]$RulePath) {
    $script = @'
set -eu
rule_path="$1"
rm -f "$rule_path"
include_line="@include $rule_path"
for policy in /etc/sudoers-rs /etc/sudoers; do
  [ -f "$policy" ] || continue
  grep -Fxq "$include_line" "$policy" || continue
  tmp="$(mktemp)"
  trap 'rm -f "$tmp"' EXIT
  awk -v include_line="$include_line" '$0 != include_line { print }' "$policy" > "$tmp"
  install -o root -g root -m 0440 "$tmp" "$policy"
  rm -f "$tmp"
  trap - EXIT
done
'@
    $probe = Invoke-WslCaptureWithInput @(
        "--distribution", $Name,
        "--user", "root",
        "--exec", "sh", "-s", "--", $RulePath
    ) $script
    if ($probe.ExitCode -ne 0) {
        throw "Failed to remove legacy Hacocoon sudo drop-in '$RulePath': $($probe.Stderr)"
    }
}

function Set-HacocoonSudoPolicyBlock([string]$Name, [string]$MarkerName, [string]$Rule) {
    $policies = @(Get-SudoersPolicyFiles $Name)
    Remove-HacocoonSudoPolicyBlock $Name $MarkerName
    $markerStart = "# BEGIN HACOCOON $MarkerName"
    $markerEnd = "# END HACOCOON $MarkerName"
    $block = "`n$markerStart`n$Rule`n$markerEnd`n"

    foreach ($policy in $policies) {
        $probe = Write-WslUtf8File $Name $policy $block -Append
        if ($probe.ExitCode -ne 0) {
            throw "Failed to append Hacocoon sudo block '$MarkerName' to '$policy': $($probe.Stderr)"
        }
        $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "/usr/sbin/visudo", "-cf", $policy)
        if ($probe.ExitCode -ne 0) {
            throw "Sudo policy '$policy' rejected Hacocoon sudo block '$MarkerName': $($probe.Stderr)"
        }
    }
    return ($policies -join ", ")
}

function Add-HacocoonBootstrapSudoRule([string]$Name, [string]$LoginUser) {
    Remove-LegacyHacocoonSudoDropIn $Name "/etc/sudoers.d/hacocoon-bootstrap"
    return Set-HacocoonSudoPolicyBlock $Name "BOOTSTRAP" "$LoginUser ALL=(ALL:ALL) NOPASSWD: ALL"
}

function Remove-HacocoonBootstrapSudoRule([string]$Name) {
    Remove-HacocoonSudoPolicyBlock $Name "BOOTSTRAP"
    Remove-LegacyHacocoonSudoDropIn $Name "/etc/sudoers.d/hacocoon-bootstrap"
}

function Set-HacocoonLoginSudoRule([string]$Name, [string]$LoginUser, [string]$Haco) {
    Remove-LegacyHacocoonSudoDropIn $Name "/etc/sudoers.d/hacocoon-login"
    return Set-HacocoonSudoPolicyBlock $Name "LOGIN" "$LoginUser ALL=(ALL:ALL) NOPASSWD: $Haco host ensure, $Haco host shell"
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
    # Write a marked rule directly into the active policy file(s), prove it with
    # a real non-interactive sudo command, then remove it in finally.
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

old_login = r'''    $sudoers = "$LoginUser ALL=NOPASSWD: $haco host ensure, $haco host shell`n"
    $probe = Write-WslUtf8File $Name "/etc/sudoers.d/hacocoon-login" $sudoers
    if ($probe.ExitCode -ne 0) { throw "Failed to write the narrow Hacocoon WSL sudo rule: $($probe.Stderr)" }
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "chmod", "0440", "/etc/sudoers.d/hacocoon-login")
    if ($probe.ExitCode -ne 0) { throw "Failed to protect the narrow Hacocoon WSL sudo rule: $($probe.Stderr)" }
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "/usr/sbin/visudo", "-cf", "/etc/sudoers.d/hacocoon-login")
    if ($probe.ExitCode -ne 0) { throw "Failed to validate the narrow Hacocoon WSL sudo rule: $($probe.Stderr)" }
    $activePolicy = Ensure-HacocoonSudoRuleLoaded $Name "/etc/sudoers.d/hacocoon-login"
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", $LoginUser, "--exec", "sudo", "-n", $haco, "host", "ensure")
    if ($probe.ExitCode -ne 0) {
        throw "Narrow Hacocoon WSL sudo rule is not effective through '$activePolicy': $($probe.Stderr)"
    }
'''
new_login = r'''    $policySet = Set-HacocoonLoginSudoRule $Name $LoginUser $haco
    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", $LoginUser, "--exec", "sudo", "-n", $haco, "host", "ensure")
    if ($probe.ExitCode -ne 0) {
        throw "Narrow Hacocoon WSL sudo rule is not effective through policy files '$policySet': $($probe.Stderr)"
    }
'''
if installer.count(old_login) != 1:
    raise SystemExit(f"expected one old Configure-WslPost sudo block, found {installer.count(old_login)}")
installer = installer.replace(old_login, new_login)

for forbidden in (
    "BootstrapSudoersPath",
    "Ensure-HacocoonSudoRuleLoaded",
    "Remove-HacocoonSudoRuleInclude",
    "@include $RulePath",
):
    if forbidden in installer:
        raise SystemExit(f"old sudo drop-in mechanism survived patch: {forbidden}")
installer_path.write_text(installer, encoding="utf-8")

workflow_path = Path(".github/workflows/windows-all-scripts-e2e.yml")
workflow = workflow_path.read_text(encoding="utf-8")
old = '''          & wsl.exe --distribution $instance --user root --exec test ! -e /etc/sudoers.d/hacocoon-bootstrap
          if ($LASTEXITCODE -ne 0) { throw "Temporary bootstrap sudo rule leaked after installer completion." }
'''
new = '''          & wsl.exe --distribution $instance --user root --exec test ! -e /etc/sudoers.d/hacocoon-bootstrap
          if ($LASTEXITCODE -ne 0) { throw "Legacy temporary bootstrap sudo drop-in leaked after installer completion." }
          & wsl.exe --distribution $instance --user root --exec test ! -e /etc/sudoers.d/hacocoon-login
          if ($LASTEXITCODE -ne 0) { throw "Legacy Hacocoon login sudo drop-in leaked after installer completion." }
          & wsl.exe --distribution $instance --user root --exec sh -eu -c 'for policy in /etc/sudoers-rs /etc/sudoers; do [ ! -f "$policy" ] || ! grep -Fq "# BEGIN HACOCOON BOOTSTRAP" "$policy"; done'
          if ($LASTEXITCODE -ne 0) { throw "Temporary bootstrap sudo policy block leaked after installer completion." }
'''
if workflow.count(old) != 1:
    raise SystemExit(f"expected one old E2E sudo cleanup assertion, found {workflow.count(old)}")
workflow_path.write_text(workflow.replace(old, new), encoding="utf-8")

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
                'Set-HacocoonSudoPolicyBlock',
                'Remove-HacocoonSudoPolicyBlock',
                'Set-HacocoonLoginSudoRule',
                '# BEGIN HACOCOON $MarkerName',
                '$LoginUser ALL=(ALL:ALL) NOPASSWD: ALL',
                'Validating temporary sudo rule through policy files','''
if test.count(old) != 1:
    raise SystemExit(f"expected one old package contract block, found {test.count(old)}")
test = test.replace(old, new)
needle = '''                '\"update-alternatives\"',
            ):'''
replacement_forbidden = '''                '\"update-alternatives\"',
                '@include $RulePath',
            ):'''
if test.count(needle) != 1:
    raise SystemExit(f"expected one forbidden provider tuple tail, found {test.count(needle)}")
test = test.replace(needle, replacement_forbidden)
test_path.write_text(test, encoding="utf-8")

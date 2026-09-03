from pathlib import Path
import re

installer_path = Path("scripts/install-windows.ps1")
installer = installer_path.read_text(encoding="utf-8")

replacement = r'''function Get-SudoersPolicyFiles([string]$Name) {
    # Ubuntu 26.04 can ship more than one sudo implementation. Provider
    # symlink/version details have changed across images, so do not guess
    # which parser is active. Load the Hacocoon-owned include into every
    # supported policy file that actually exists, then verify behavior by
    # running sudo as the target user.
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

function Ensure-HacocoonSudoRuleLoaded([string]$Name, [string]$RulePath) {
    $includedirPattern = '^[[:space:]]*(@includedir|#includedir)[[:space:]]+/etc/sudoers\.d([[:space:]]|$)'
    $includeLine = "@include $RulePath"
    $loadedPolicies = @()

    foreach ($policy in @(Get-SudoersPolicyFiles $Name)) {
        $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "grep", "-Eq", $includedirPattern, $policy)
        if ($probe.ExitCode -eq 0) {
            $loadedPolicies += $policy
            continue
        }
        if ($probe.ExitCode -ne 1) {
            throw "Failed to inspect sudo policy '$policy': $($probe.Stderr)"
        }

        # Minimal Ubuntu/WSL policy may ignore /etc/sudoers.d. Include only
        # the Hacocoon-owned temporary rule instead of enabling unrelated
        # drop-ins.
        $probe = Invoke-WslCapture @("--distribution", $Name, "--user", "root", "--exec", "grep", "-Fx", $includeLine, $policy)
        if ($probe.ExitCode -eq 1) {
            $probe = Write-WslUtf8File $Name $policy ("`n$includeLine`n") -Append
            if ($probe.ExitCode -ne 0) {
                throw "Failed to include '$RulePath' from sudo policy '$policy': $($probe.Stderr)"
            }
        } elseif ($probe.ExitCode -ne 0) {
            throw "Failed to inspect Hacocoon sudo include in '$policy': $($probe.Stderr)"
        }
        $loadedPolicies += $policy
    }

    return ($loadedPolicies -join ", ")
}

'''

pattern = re.compile(
    r"function Get-ActiveSudoersPolicy\(\[string\]\$Name\) \{.*?\n\}\n\n"
    r"function Ensure-HacocoonSudoRuleLoaded\(\[string\]\$Name, \[string\]\$RulePath\) \{.*?\n\}\n\n",
    re.S,
)
installer, count = pattern.subn(lambda _: replacement, installer, count=1)
if count != 1:
    raise SystemExit(f"expected one provider/loader block, replaced {count}")

cleanup_pattern = re.compile(
    r"\n    \$activePolicy = Get-ActiveSudoersPolicy \$Name\n"
    r"    \$probe = Invoke-WslCapture @\(\"--distribution\", \$Name, \"--user\", \"root\", \"--exec\", \"/usr/sbin/visudo\", \"-cf\", \$activePolicy\)\n"
    r"    if \(\$probe\.ExitCode -ne 0\) \{\n"
    r"        throw \"Active sudo policy '\$activePolicy' is invalid after Hacocoon include cleanup: \$\(\$probe\.Stderr\)\"\n"
    r"    \}\n",
)
installer, count = cleanup_pattern.subn(
    "\n    # The temporary rule is removed before this cleanup. Explicit includes\n"
    "    # are removed from every supported policy file, so provider selection\n"
    "    # is intentionally irrelevant here.\n",
    installer,
    count=1,
)
if count != 1:
    raise SystemExit(f"expected one active-policy cleanup block, replaced {count}")

old = '$activePolicy = Ensure-HacocoonSudoRuleLoaded $Name $BootstrapSudoersPath\n    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", $LoginUser, "--exec", "sudo", "-n", "/usr/bin/true")'
new = '$policySet = Ensure-HacocoonSudoRuleLoaded $Name $BootstrapSudoersPath\n    Write-Step "Validating temporary sudo rule through policy candidates: $policySet"\n    $probe = Invoke-WslCapture @("--distribution", $Name, "--user", $LoginUser, "--exec", "sudo", "-n", "/usr/bin/true")'
if installer.count(old) != 1:
    raise SystemExit(f"expected one bootstrap validation block, found {installer.count(old)}")
installer = installer.replace(old, new)

old = "Temporary installer sudo rule is not effective for '$LoginUser' through '$activePolicy': $($probe.Stderr) Policy: $($policy.Stdout) $($policy.Stderr)"
new = "Temporary installer sudo rule is not effective for '$LoginUser' after loading policy candidates '$policySet': $($probe.Stderr) Policy: $($policy.Stdout) $($policy.Stderr)"
if installer.count(old) != 1:
    raise SystemExit(f"expected one bootstrap error message, found {installer.count(old)}")
installer = installer.replace(old, new)

for forbidden in (
    "Get-ActiveSudoersPolicy",
    '"readlink", "-f", "/usr/bin/sudo"',
    "/usr/lib/cargo/bin/sudo",
):
    if forbidden in installer:
        raise SystemExit(f"provider guessing survived patch: {forbidden}")
installer_path.write_text(installer, encoding="utf-8")


test_path = Path("tools/test_installer_packages.py")
test = test_path.read_text(encoding="utf-8")
old = '''                'Get-ActiveSudoersPolicy',
                '\"readlink\", \"-f\", \"/usr/bin/sudo\"',
                '/usr/lib/cargo/bin/sudo',
                '/etc/sudoers-rs','''
new = '''                'Get-SudoersPolicyFiles',
                '@(\"/etc/sudoers-rs\", \"/etc/sudoers\")',
                'Validating temporary sudo rule through policy candidates',
                '/etc/sudoers-rs','''
if test.count(old) != 1:
    raise SystemExit(f"expected one required provider marker block, found {test.count(old)}")
test = test.replace(old, new)

old = '''            if "$provider.Stdout -match '^sudo-rs'" in windows_installer:
                raise SystemExit(
                    "Windows installer regressed to human-facing sudo version-string provider detection"
                )'''
new = '''            for forbidden_provider_guess in (
                "$provider.Stdout -match '^sudo-rs'",
                '\"readlink\", \"-f\", \"/usr/bin/sudo\"',
                '\"update-alternatives\"',
            ):
                if forbidden_provider_guess in windows_installer:
                    raise SystemExit(
                        f"Windows installer regressed to sudo provider guessing: {forbidden_provider_guess!r}"
                    )'''
if test.count(old) != 1:
    raise SystemExit(f"expected one forbidden provider check, found {test.count(old)}")
test = test.replace(old, new)
test_path.write_text(test, encoding="utf-8")

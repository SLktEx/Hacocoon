from pathlib import Path

root = Path(__file__).resolve().parents[1]
installer_path = root / "scripts" / "install-windows.ps1"
test_path = root / "tools" / "test_installer_packages.py"

installer = installer_path.read_text(encoding="utf-8")

anchor = '''    return New-WslCaptureResult $exitCode $stdout $stderr
}

function Write-WslUtf8File'''
insert = '''    return New-WslCaptureResult $exitCode $stdout $stderr
}

function Invoke-WslRootShellScript([string]$Name, [string]$Script, [string[]]$ScriptArguments = @()) {
    # Shell source is control data, not stdin payload. Encode it into an argv-safe
    # base64 string, materialize it inside WSL, and execute the temporary file.
    # This avoids Windows PowerShell native-pipeline encoding/BOM behavior and
    # also avoids sh -s early-exit interactions with the stdin producer.
    $normalized = ($Script -replace "`r`n", "`n") -replace "`r", "`n"
    $encoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($normalized))
    $runner = 'tmp="$(mktemp)"; trap ''rm -f "$tmp"'' EXIT; printf ''%s'' "$1" | base64 -d > "$tmp"; shift; sh -eu "$tmp" "$@"'
    $arguments = @(
        "--distribution", $Name,
        "--user", "root",
        "--exec", "sh", "-eu", "-c", $runner, "sh", $encoded
    ) + @($ScriptArguments)
    return Invoke-WslCapture $arguments
}

function Write-WslUtf8File'''
if anchor not in installer:
    raise SystemExit("Invoke-WslCaptureWithInput insertion anchor not found")
installer = installer.replace(anchor, insert, 1)

replacements = {
'''        $probe = Invoke-WslCaptureWithInput @(
            "--distribution", $Name,
            "--user", "root",
            "--exec", "sh", "-s", "--", $policy, $markerStart, $markerEnd
        ) $script
''': '''        $probe = Invoke-WslRootShellScript $Name $script @($policy, $markerStart, $markerEnd)
''',
'''    $probe = Invoke-WslCaptureWithInput @(
        "--distribution", $Name,
        "--user", "root",
        "--exec", "sh", "-s", "--", $RulePath
    ) $script
''': '''    $probe = Invoke-WslRootShellScript $Name $script @($RulePath)
''',
'''    $probe = Invoke-WslCaptureWithInput @(
        "--distribution", $Name,
        "--user", "root",
        "--exec", "sh", "-s", "--", $LoginUser
    ) $script
''': '''    $probe = Invoke-WslRootShellScript $Name $script @($LoginUser)
''',
'''    $probe = Invoke-WslCaptureWithInput @("--distribution", $Name, "--user", "root", "--exec", "sh", "-s") $script
''': '''    $probe = Invoke-WslRootShellScript $Name $script
''',
}
for old, new in replacements.items():
    count = installer.count(old)
    if count != 1:
        raise SystemExit(f"expected one shell-stdin call, found {count}: {old.splitlines()[0]!r}")
    installer = installer.replace(old, new, 1)

installer = installer.replace(
    '''throw "Failed to remove Hacocoon sudo block '$MarkerName' from '$policy': $($probe.Stderr)"''',
    '''throw "Failed to remove Hacocoon sudo block '$MarkerName' from '$policy' (exit $($probe.ExitCode)): $($probe.Stderr)"''',
    1,
)
installer = installer.replace(
    '''throw "Failed to remove legacy Hacocoon sudo drop-in '$RulePath': $($probe.Stderr)"''',
    '''throw "Failed to remove legacy Hacocoon sudo drop-in '$RulePath' (exit $($probe.ExitCode)): $($probe.Stderr)"''',
    1,
)
installer_path.write_text(installer, encoding="utf-8", newline="\n")

test = test_path.read_text(encoding="utf-8")
needle = '''                'LC_ALL=C sed ',
'''
replacement = needle + '''                'function Invoke-WslRootShellScript',
                'sh -eu "$tmp" "$@"',
'''
if needle not in test:
    raise SystemExit("shell transport required marker insertion point not found")
test = test.replace(needle, replacement, 1)

needle = '''                '$normalized | & wsl.exe @Arguments',
'''
replacement = needle + '''                '"--exec", "sh", "-s"',
'''
if needle not in test:
    raise SystemExit("shell transport forbidden marker insertion point not found")
test = test.replace(needle, replacement, 1)
test_path.write_text(test, encoding="utf-8", newline="\n")

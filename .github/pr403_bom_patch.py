from pathlib import Path

root = Path(__file__).resolve().parents[1]
installer_path = root / "scripts" / "install-windows.ps1"
test_path = root / "tools" / "test_installer_packages.py"

installer = installer_path.read_text(encoding="utf-8")
old = '''        $normalized = ($InputText -replace "`r`n", "`n") -replace "`r", "`n"
        $stdout = @($normalized | & wsl.exe @Arguments 2> $stderrPath)
'''
new = '''        $normalized = ($InputText -replace "`r`n", "`n") -replace "`r", "`n"

        # Windows PowerShell 5.1 can still emit a UTF-8 preamble to native
        # stdin even when $OutputEncoding uses a BOM-free encoder. Wrap the
        # Linux command so WSL strips only a leading UTF-8 BOM before handing
        # stdin to the original command. This keeps shell scripts and base64
        # payloads byte-stable on both Windows PowerShell and PowerShell 7.
        $execIndex = -1
        for ($index = 0; $index -lt $Arguments.Count; $index++) {
            if ($Arguments[$index] -eq "--exec") {
                $execIndex = $index
                break
            }
        }
        if ($execIndex -lt 0 -or $execIndex -ge ($Arguments.Count - 1)) {
            throw "Invoke-WslCaptureWithInput requires a WSL --exec command."
        }
        $wrappedArguments = @($Arguments[0..$execIndex]) + @(
            "sh",
            "-c",
            'LC_ALL=C sed ''1s/^\\xEF\\xBB\\xBF//'' | "$@"',
            "sh"
        ) + @($Arguments[($execIndex + 1)..($Arguments.Count - 1)])
        $stdout = @($normalized | & wsl.exe @wrappedArguments 2> $stderrPath)
'''
if old not in installer:
    raise SystemExit("Invoke-WslCaptureWithInput target block not found")
installer = installer.replace(old, new, 1)
installer_path.write_text(installer, encoding="utf-8", newline="\n")

test = test_path.read_text(encoding="utf-8")
needle = '''                '$OutputEncoding = [Text.UTF8Encoding]::new($false)',
'''
replacement = needle + '''                'LC_ALL=C sed ''1s/^\\xEF\\xBB\\xBF//'' | "$@"',
'''
if needle not in test:
    raise SystemExit("required Windows contract insertion point not found")
test = test.replace(needle, replacement, 1)

needle = '''            forbidden_windows_contract = [
                "Complete normal Ubuntu user setup, then run this installer again.",
                "After completing the Ubuntu user setup, run install-windows.bat again.",
            ]
'''
replacement = '''            forbidden_windows_contract = [
                "Complete normal Ubuntu user setup, then run this installer again.",
                "After completing the Ubuntu user setup, run install-windows.bat again.",
                '$normalized | & wsl.exe @Arguments',
            ]
'''
if needle not in test:
    raise SystemExit("forbidden Windows contract insertion point not found")
test = test.replace(needle, replacement, 1)
test_path.write_text(test, encoding="utf-8", newline="\n")

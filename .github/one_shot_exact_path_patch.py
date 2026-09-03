from pathlib import Path

root = Path(__file__).resolve().parents[1]
path = root / "tools" / "windows-installer-user-path-e2e.py"
text = path.read_text(encoding="utf-8")

text = text.replace('INSTANCE = "Hacocoon"\n', 'INSTANCE = "Hacocoon"\nMANAGED_USER = "hacocoon"\n', 1)

start = text.index("def normal_user_name()")
end = text.index("def normalize_terminal", start)
text = text[:start] + text[end:]

start = text.index("def complete_ubuntu_first_launch()")
end = text.index("def run_host_session", start)
text = text[:start] + text[end:]

old = '''    user = normal_user_name()\n    passwd = observe_wsl_root("getent", "passwd", user, phase=phase)\n'''
new = '''    user = MANAGED_USER\n    passwd = observe_wsl_root("getent", "passwd", user, phase=phase)\n'''
if old not in text:
    raise SystemExit("normal-user assertion marker not found")
text = text.replace(old, new, 1)

needle = '''    groups = observe_wsl_root("id", "-nG", user, phase=phase).split()\n    if "hacocoon" not in groups:\n        raise RuntimeError(f"{phase}: normal WSL user lacks hacocoon group: {groups!r}")\n'''
replacement = needle + '''\n    bootstrap = observe_wsl_root(\n        "sh",\n        "-c",\n        "grep -h -F '# BEGIN HACOCOON BOOTSTRAP' /etc/sudoers-rs /etc/sudoers 2>/dev/null || true",\n        phase=phase,\n    )\n    if bootstrap.strip():\n        raise RuntimeError(f"{phase}: temporary bootstrap sudo rule remains installed")\n'''
if needle not in text:
    raise SystemExit("group assertion insertion point not found")
text = text.replace(needle, replacement, 1)

old = '''    print("==> ACTION USER TYPES: install-windows.bat")\n    first = run_bat(package_root, phase="first install")\n    require_output(first, r"wsl -d Hacocoon", phase="first install")\n    assert_wsl2(phase="after first BAT")\n\n    print("==> ACTION USER TYPES: wsl -d Hacocoon")\n    complete_ubuntu_first_launch()\n\n    print("==> ACTION USER TYPES: install-windows.bat")\n    second = run_bat(package_root, phase="install completion")\n    require_output(\n        second,\n        r"Hacocoon WSL installation complete",\n        phase="install completion",\n    )\n    assert_installed_host_state(phase="after install completion")\n'''
new = '''    print("==> ACTION USER TYPES: install-windows.bat")\n    first = run_bat(package_root, phase="first install")\n    require_output(\n        first,\n        r"Hacocoon WSL installation complete",\n        phase="first install",\n    )\n    assert_installed_host_state(phase="after first BAT")\n'''
if old not in text:
    raise SystemExit("two-stage first-install sequence not found")
text = text.replace(old, new, 1)

# The only later BAT invocation is the unchanged reinstall regression.
text = text.replace('    third = run_bat(package_root, phase="reinstall")\n', '    second = run_bat(package_root, phase="reinstall")\n', 1)
text = text.replace('        third,\n        r"Hacocoon WSL installation complete",\n', '        second,\n        r"Hacocoon WSL installation complete",\n', 1)

for forbidden in (
    "complete_ubuntu_first_launch",
    "normal_user_name",
    'phase="install completion"',
):
    if forbidden in text:
        raise SystemExit(f"two-stage exact-user path remains: {forbidden}")

path.write_text(text, encoding="utf-8", newline="\n")

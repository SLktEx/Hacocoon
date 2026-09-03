from pathlib import Path

path = Path(__file__).resolve().parents[1] / "tools" / "test_installer_packages.py"
text = path.read_text(encoding="utf-8")
old = "                '# BEGIN HACOCOON $MarkerName',\n"
new = "                '# BEGIN HACOCOON $marker_name',\n"
if old not in text:
    raise SystemExit("old dynamic sudo marker contract not found")
path.write_text(text.replace(old, new, 1), encoding="utf-8", newline="\n")

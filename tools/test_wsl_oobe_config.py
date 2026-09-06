#!/usr/bin/env python3
"""Exercise the actual managed-OOBE transform without changing a distribution."""
from pathlib import Path
import re
import subprocess
import unittest

INSTALLER = (Path(__file__).resolve().parents[1] / "scripts/install-windows.ps1").read_text()
TRANSFORM = re.search(r"# BEGIN MANAGED OOBE TRANSFORM\nawk -v uid=\"\$uid\" '\n(.*?)\n' \"\$config\"", INSTALLER, re.S)[1]
CONFIG = "[oobe]\ncommand = /usr/lib/wsl/wsl-setup\ndefaultUid = 1000\ndefaultName = Ubuntu-26.04\n\n[shortcut]\nicon = /usr/share/wsl/ubuntu.ico\n"


class ManagedOobeTests(unittest.TestCase):
    def transform(self, config):
        return subprocess.run(["awk", "-v", "uid=1007", TRANSFORM], input=config,
                              text=True, capture_output=True)

    def test_managed_identity_and_unrelated_metadata(self):
        result = self.transform(CONFIG)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, CONFIG.replace("command = /usr/lib/wsl/wsl-setup", "command=")
                         .replace("defaultUid = 1000", "defaultUid=1007"))
        self.assertEqual(self.transform(result.stdout).stdout, result.stdout)

    def test_unknown_or_ambiguous_setup_fails_before_replacement(self):
        cases = [CONFIG.replace("/usr/lib/wsl/wsl-setup", "/custom/setup"),
                 CONFIG + "[oobe]\ncommand=\ndefaultUid=2000\n",
                 CONFIG.replace("defaultUid = 1000\n", ""),
                 CONFIG.replace("command =", "command=\ncommand ="), ""]
        for config in cases:
            with self.subTest(config=config):
                self.assertNotEqual(self.transform(config).returncode, 0)


if __name__ == "__main__":
    unittest.main()

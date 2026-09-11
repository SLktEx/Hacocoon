"""Native regression: retained encrypted fixtures outlive a private /tmp."""
import os
from pathlib import Path
import re
import subprocess
import unittest
import uuid


@unittest.skipUnless(os.environ.get("HACO_E2E_ENCRYPTED_EVACUATION") == "1",
                     "requires explicit Linux systemd/tar/age acceptance")
class EncryptedFixtureRetentionTests(unittest.TestCase):
    def test_fixture_survives_private_tmp_service_exit(self):
        self.assertEqual(os.geteuid(), 0, "native durable fixture requires root")
        script = Path(__file__).resolve().with_name("test_encrypted_evacuation.py")
        result = subprocess.run([
            "systemd-run", "--unit=haco-encrypted-retention-" + uuid.uuid4().hex,
            "--wait", "--pipe", "--collect", "--property=PrivateTmp=yes",
            "--property=RuntimeMaxSec=150s", "--property=TimeoutStopSec=5s",
            "--setenv=HACO_E2E_ENCRYPTED_EVACUATION=1",
            "/usr/bin/python3", "-B", str(script)], capture_output=True, text=True, timeout=180)
        self.assertEqual(result.returncode, 0, "private-temp encrypted test failed")
        matches = re.findall(r"^Synthetic evacuation fixture retained at (/(?:tmp|var/lib)/haco-encrypted-evacuation-[a-z0-9_]+)$", result.stdout, re.M)
        self.assertEqual(len(matches), 1, "missing exact fixture path")
        root = Path(matches[0])
        self.assertTrue(root.is_dir() and not root.is_symlink(), "reported retained fixture disappeared with private /tmp")
        self.assertEqual(root.stat().st_mode & 0o777, 0o700)
        # Only metadata is inspected here; no identity is printed or transferred.
        for name in ["identity.txt", "wrong-identity.txt", "source/.config/credential", "restored/.config/credential"]:
            path = root / name
            self.assertTrue(path.is_file() and not path.is_symlink(), "retained fixture file missing")
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
        self.assertTrue((root / "output/data.tar.age").is_file())
        self.assertTrue((root / "output/receipt.json").is_file())
        print("PASS: encrypted fixture survived private temporary namespace exit at " + str(root), flush=True)
        print("No prior ciphertext restored; key transfer and recovery after WSL deletion remain unverified.", flush=True)


if __name__ == "__main__":
    unittest.main()

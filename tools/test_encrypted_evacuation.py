"""Opt-in real GNU tar/age evacuation transport test; synthetic data only."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

from evacuation_capture import capture_tree


@unittest.skipUnless(os.environ.get("HACO_E2E_ENCRYPTED_EVACUATION") == "1",
                     "requires explicit Linux tar/age evacuation acceptance")
class EncryptedEvacuationTests(unittest.TestCase):
    def test_encrypted_archive_and_failure_detection(self):
        for tool in ("tar", "age", "age-keygen"):
            self.assertIsNotNone(shutil.which(tool), "required tool missing: " + tool)
        self.assertEqual(os.geteuid(), 0, "native durable fixture requires root")
        # /tmp may be tmpfs or service-private and disappear after this run.
        # This keeps review evidence across normal service/WSL restarts, not WSL deletion.
        private = Path(tempfile.mkdtemp(prefix="haco-encrypted-evacuation-", dir="/var/lib"))
        os.chmod(private, 0o700)
        # Retain exact synthetic artifacts for review, including failed attempts.
        print("Synthetic evacuation fixture retained at " + str(private), flush=True)
        destination_root = os.environ.get("HACO_E2E_ENCRYPTED_OUTPUT_ROOT")
        destination = Path(tempfile.mkdtemp(prefix="haco-encrypted-output-", dir=destination_root)) if destination_root else private / "output"
        destination.mkdir(mode=0o700, exist_ok=bool(destination_root))
        source = private / "source"
        source.mkdir(mode=0o700)
        (source / ".config").mkdir(mode=0o700)
        marker = b"synthetic-credential-not-a-real-secret\n"
        (source / ".config" / "credential").write_bytes(marker)
        os.chmod(source / ".config" / "credential", 0o600)
        key = private / "identity.txt"
        def command(args, **options):
            return subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                  timeout=60, check=False, **options)
        self.assertEqual(command(["age-keygen", "-o", str(key)]).returncode, 0)
        os.chmod(key, 0o600)
        recipient = command(["age-keygen", "-y", str(key)])
        self.assertEqual(recipient.returncode, 0)
        public = recipient.stdout.decode().strip()
        archive = destination / "data.tar.age"
        env = os.environ.copy()
        env.pop("TAR_OPTIONS", None)
        # No plaintext archive is written outside the trusted Linux fixture.
        captured = capture_tree(str(source), str(destination), public, seconds=60)
        self.assertTrue(captured["archive_complete"])
        self.assertFalse(captured["backup_complete"])
        self.assertFalse(captured["external_retention_verified"])
        ciphertext = archive.read_bytes()
        self.assertNotIn(marker, ciphertext)
        digest = hashlib.sha256(ciphertext).hexdigest()
        restored = command(["age", "-d", "-i", str(key), str(archive)])
        self.assertEqual(restored.returncode, 0, "decrypt failed")
        extract = private / "restored"
        extract.mkdir(mode=0o700)
        # Only this test's own synthetic authenticated archive is extracted.
        unpack = command(["tar", "--acls", "--xattrs", "--numeric-owner", "-xpf", "-", "-C", str(extract)], input=restored.stdout, env=env)
        self.assertEqual(unpack.returncode, 0)
        self.assertEqual((extract / ".config" / "credential").read_bytes(), marker)
        self.assertEqual((extract / ".config" / "credential").stat().st_mode & 0o777, 0o600)
        wrong = private / "wrong-identity.txt"
        self.assertEqual(command(["age-keygen", "-o", str(wrong)]).returncode, 0)
        self.assertNotEqual(command(["age", "-d", "-i", str(wrong), str(archive)]).returncode, 0)
        damaged = private / "damaged.age"
        altered = bytearray(ciphertext)
        altered[-1] ^= 1
        damaged.write_bytes(altered)
        self.assertNotEqual(command(["age", "-d", "-i", str(key), str(damaged)]).returncode, 0)
        truncated = private / "truncated.age"
        truncated.write_bytes(ciphertext[:-32])
        self.assertNotEqual(command(["age", "-d", "-i", str(key), str(truncated)]).returncode, 0)
        self.assertEqual(hashlib.sha256(archive.read_bytes()).hexdigest(), digest)
        receipt = {"scope": "synthetic encrypted archive transport only", "backup_complete": False,
                   "archive": "data.tar.age", "sha256": digest, "bytes": len(ciphertext),
                   "decryption": "passed", "wrong_key_tamper_truncation": "refused"}
        with (destination / "receipt.json").open("x") as out:
            json.dump(receipt, out, indent=2)
            out.flush()
            os.fsync(out.fileno())
        print("Encrypted synthetic archive and receipt retained at " + str(destination), flush=True)


if __name__ == "__main__":
    unittest.main()
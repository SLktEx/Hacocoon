"""Real tar/age checks for explicit, synthetic Linux capture roots."""
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import evacuation_capture as subject


@unittest.skipUnless(os.environ.get("HACO_E2E_ENCRYPTED_EVACUATION") == "1", "requires explicit tar/age acceptance")
class NativeCaptureTests(unittest.TestCase):
    def setUp(self):
        for name in ("tar", "age", "age-keygen"):
            self.assertIsNotNone(shutil.which(name), "missing native tool: " + name)
        self.temp = tempfile.TemporaryDirectory(prefix="haco-capture-native-")
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.source, self.output = root / "source", root / "output"
        self.source.mkdir(mode=0o700)
        self.output.mkdir(mode=0o700)
        self.key = root / "identity.txt"
        subprocess.run(["age-keygen", "-o", str(self.key)], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.recipient = subprocess.run(["age-keygen", "-y", str(self.key)], check=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL).stdout.decode().strip()
        (self.source / "data").write_bytes(b"synthetic-only\n" * 1000)

    def capture(self, **options):
        return subject.capture_tree(str(self.source), str(self.output), self.recipient, **options)

    def assert_incomplete(self):
        self.assertTrue((self.output / "capture-intent.json").is_file())
        self.assertFalse((self.output / "capture-complete.json").exists())

    def test_ciphertext_preserves_files_links_modes_and_xattr(self):
        (self.source / "alias").symlink_to("data")
        (self.source.parent / "outside").write_bytes(b"outside-tree-synthetic-marker")
        (self.source / "external").symlink_to("../outside")
        os.link(self.source / "data", self.source / "hard")
        os.chmod(self.source / "data", 0o750)
        os.setxattr(self.source / "data", b"user.haco-capture", b"retained")
        result = self.capture()
        self.assertTrue(result["archive_complete"])
        self.assertFalse(result["backup_complete"])
        cipher = (self.output / "data.tar.age").read_bytes()
        self.assertNotIn(b"synthetic-only", cipher)
        plain = subprocess.run(["age", "-d", "-i", str(self.key), str(self.output / "data.tar.age")], check=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL).stdout
        self.assertNotIn(b"outside-tree-synthetic-marker", plain)
        with tarfile.open(fileobj=io.BytesIO(plain)) as archive:
            members = {m.name.removeprefix("./"): m for m in archive.getmembers()}
            self.assertEqual(archive.extractfile("./data").read(), (self.source / "data").read_bytes())
            self.assertEqual(members["data"].mode, 0o750)
            self.assertEqual(members["alias"].linkname, "data")
            self.assertEqual(members["external"].linkname, "../outside")
            self.assertTrue(members["data"].islnk() or members["hard"].islnk())
            regular = members["hard"] if members["data"].islnk() else members["data"]
            self.assertEqual(regular.pax_headers["SCHILY.xattr.user.haco-capture"], "retained")
        receipt = json.loads((self.output / "capture-complete.json").read_text())
        self.assertEqual(receipt["sha256"], result["sha256"])
        with self.assertRaises(subject.CaptureError):
            self.capture()
        self.assertEqual((self.output / "data.tar.age").read_bytes(), cipher)

    def test_changed_source_never_receives_completion(self):
        original = subject._tree
        calls = 0
        def observe(root):
            nonlocal calls
            calls += 1
            if calls == 2:
                (self.source / "data").write_bytes(b"changed-after-capture")
            return original(root)
        with patch.object(subject, "_tree", side_effect=observe):
            with self.assertRaisesRegex(subject.CaptureError, "source metadata changed"):
                self.capture()
        self.assert_incomplete()

    def test_replaced_ciphertext_is_not_reported_as_the_completed_archive(self):
        original = subject._tree
        calls = 0
        def observe(root):
            nonlocal calls
            calls += 1
            if calls == 2:
                (self.output / "data.tar.age").rename(self.output / "retained.age")
                (self.output / "data.tar.age").write_bytes(b"replacement")
            return original(root)
        with patch.object(subject, "_tree", side_effect=observe):
            with self.assertRaisesRegex(subject.CaptureError, "encrypted output changed"):
                self.capture()
        self.assert_incomplete()
        self.assertTrue((self.output / "retained.age").is_file())
        self.assertEqual((self.output / "data.tar.age").read_bytes(), b"replacement")

    def test_byte_limit_preserves_partial_ciphertext_without_completion(self):
        with self.assertRaisesRegex(subject.CaptureError, "byte limit"):
            self.capture(byte_limit=1)
        self.assert_incomplete()

    def test_failed_producer_is_not_hidden_by_successful_encryption(self):
        original = subject.subprocess.Popen
        def launch(args, **options):
            if args[0] == "tar":
                args = ["false"]
            return original(args, **options)
        with patch.object(subject.subprocess, "Popen", side_effect=launch):
            with self.assertRaisesRegex(subject.CaptureError, "tar capture failed"):
                self.capture()
        self.assert_incomplete()

    def test_deadline_stops_only_created_children_without_completion(self):
        original = subject.subprocess.Popen
        children = []
        def launch(args, **options):
            if args[0] == "age":
                args = [sys.executable, "-c", "import time; time.sleep(30)"]
            process = original(args, **options)
            children.append(process)
            return process
        with patch.object(subject.subprocess, "Popen", side_effect=launch):
            with self.assertRaises(subject.CaptureError):
                self.capture(seconds=0.2)
        self.assert_incomplete()
        self.assertTrue(all(p.poll() is not None for p in children))


if __name__ == "__main__":
    unittest.main()

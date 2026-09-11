"""Real tar checks for explicit, synthetic Linux capture roots."""
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


@unittest.skipUnless(os.environ.get("HACO_E2E_PLAIN_EVACUATION") == "1", "requires explicit tar acceptance")
class NativeCaptureTests(unittest.TestCase):
    def setUp(self):
        for name in ("tar", "sha256sum"):
            self.assertIsNotNone(shutil.which(name), "missing native tool: " + name)
        self.temp = tempfile.TemporaryDirectory(prefix="haco-capture-native-")
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.source, self.output = root / "source", root / "output"
        self.source.mkdir(mode=0o700)
        self.output.mkdir(mode=0o700)
        (self.source / "data").write_bytes(b"synthetic-only\n" * 1000)

    def capture(self, **options):
        return subject.capture_tree(str(self.source), str(self.output), **options)

    def assert_incomplete(self):
        self.assertTrue((self.output / "capture-intent.json").is_file())
        self.assertFalse((self.output / "capture-complete.json").exists())

    def test_archive_preserves_files_links_modes_and_xattr(self):
        (self.source / "alias").symlink_to("data")
        (self.source.parent / "outside").write_bytes(b"outside-tree-synthetic-marker")
        (self.source / "external").symlink_to("../outside")
        os.link(self.source / "data", self.source / "hard")
        os.chmod(self.source / "data", 0o750)
        os.setxattr(self.source / "data", b"user.haco-capture", b"retained")
        command = [sys.executable, str(Path(subject.__file__).resolve()),
                   str(self.source), str(self.output), "--quiesced"]
        process = subprocess.run(command, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=60)
        result = json.loads(process.stdout)
        self.assertTrue(result["archive_complete"])
        self.assertFalse(result["backup_complete"])
        plain = (self.output / "data.tar").read_bytes()
        self.assertFalse(result["encrypted"])
        self.assertIn(b"synthetic-only", plain)
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
        # A separate copy is checked with the maintained system tool, without a key.
        copied = self.source.parent / "copied"
        copied.mkdir(mode=0o700)
        for leaf in ("data.tar", "data.tar.sha256", "capture-complete.json"):
            shutil.copyfile(self.output / leaf, copied / leaf)
        def check_copy():
            return subprocess.run(["sha256sum", "--check", "--status", "data.tar.sha256"],
                                  cwd=copied, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30).returncode
        self.assertEqual(check_copy(), 0)
        (copied / "data.tar").write_bytes(plain[:-1])
        self.assertNotEqual(check_copy(), 0)
        (copied / "data.tar").write_bytes(plain)
        self.assertEqual(check_copy(), 0)
        (copied / "data.tar").unlink()
        self.assertNotEqual(check_copy(), 0)
        with self.assertRaises(subject.CaptureError):
            self.capture()
        self.assertEqual((self.output / "data.tar").read_bytes(), plain)

    @unittest.skipUnless(os.geteuid() == 0, "requires root for trusted xattrs")
    def test_non_user_attribute_survives_explicit_native_restore(self):
        key = "trusted.hacocoon-evacuation"
        os.setxattr(self.source / "data", key, b"synthetic-only")
        self.capture()
        restored = self.source.parent / "restored"
        restored.mkdir(mode=0o700)
        subprocess.run(["tar", "--acls", "--xattrs", "--xattrs-include=*",
                        "--numeric-owner", "-xpf", str(self.output / "data.tar"),
                        "-C", str(restored)], check=True, timeout=60,
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.assertEqual(os.getxattr(restored / "data", key), b"synthetic-only")
        self.assertEqual((restored / "data").read_bytes(), (self.source / "data").read_bytes())

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

    def test_replaced_archive_is_not_reported_as_the_completed_archive(self):
        original = subject._tree
        calls = 0
        def observe(root):
            nonlocal calls
            calls += 1
            if calls == 2:
                (self.output / "data.tar").rename(self.output / "retained.tar")
                (self.output / "data.tar").write_bytes(b"replacement")
            return original(root)
        with patch.object(subject, "_tree", side_effect=observe):
            with self.assertRaisesRegex(subject.CaptureError, "archive output changed"):
                self.capture()
        self.assert_incomplete()
        self.assertTrue((self.output / "retained.tar").is_file())
        self.assertEqual((self.output / "data.tar").read_bytes(), b"replacement")

    def test_checksum_collision_retains_existing_data_without_completion(self):
        original = subject._tree
        calls = 0
        def observe(root):
            nonlocal calls
            calls += 1
            if calls == 2:
                (self.output / "data.tar.sha256").write_bytes(b"existing-checksum")
            return original(root)
        with patch.object(subject, "_tree", side_effect=observe):
            with self.assertRaises(FileExistsError):
                self.capture()
        self.assert_incomplete()
        self.assertEqual((self.output / "data.tar.sha256").read_bytes(), b"existing-checksum")

    def test_byte_limit_preserves_partial_archive_without_completion(self):
        with self.assertRaisesRegex(subject.CaptureError, "byte limit"):
            self.capture(byte_limit=1)
        self.assert_incomplete()

    def test_failed_producer_is_not_reported_complete(self):
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
            if args[0] == "tar":
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

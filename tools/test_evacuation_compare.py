import contextlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import evacuation_compare as subject


@unittest.skipUnless(hasattr(os, "O_NOFOLLOW") and Path("/proc/self/mountinfo").exists(), "Linux tree scan required")
class ComparisonTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="haco-compare-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source, self.restored = self.root / "source", self.root / "restored"
        self.source.mkdir(mode=0o700)
        self.restored.mkdir(mode=0o700)
        (self.source / "file").write_bytes(b"private-content")

    def scan(self, path=None, **options):
        return subject.scan_tree(str(path or self.source), **options)

    def roundtrip(self, value):
        path = self.root / "manifest.json"
        path.write_text(json.dumps(value))
        return subject.read_manifest(path)

    def test_native_tar_restoration_compares_contents_modes_links_and_attributes(self):
        (self.source / ".git").mkdir()
        (self.source / ".git" / "HEAD").write_text("ref: refs/heads/test\n")
        (self.source / "untracked\nname").write_text("dirty content")
        (self.source / "alias").symlink_to("file")
        (self.source / "outside").symlink_to("../external-private")
        (self.root / "external-private").write_text("must not read me")
        os.link(self.source / "file", self.source / "hardlink")
        os.chmod(self.source / "file", 0o750)
        os.setxattr(self.source / "file", b"user.hacocoon", b"private-xattr")
        archive = self.root / "data.tar"
        subprocess.run(["tar", "--acls", "--xattrs", "--numeric-owner", "-cpf", str(archive), "-C", str(self.source), "."], check=True, capture_output=True)
        subprocess.run(["tar", "--acls", "--xattrs", "-xpf", str(archive), "-C", str(self.restored)], check=True, capture_output=True)
        left, right = self.scan(), self.scan(self.restored)
        self.assertTrue(left["complete"], left)
        self.assertTrue(right["complete"], right)
        comparison = subject.compare_manifests(self.roundtrip(left), self.roundtrip(right))
        self.assertTrue(comparison["matching"], comparison)
        self.assertFalse(comparison["backup_complete"])
        self.assertNotIn("private-content", json.dumps(left))
        self.assertNotIn("private-xattr", json.dumps(left))
        self.assertNotIn("must not read me", json.dumps(left))
        (self.restored / "file").write_bytes(b"altered")
        os.chmod(self.restored / "file", 0o700)
        os.setxattr(self.restored / "file", b"user.hacocoon", b"changed")
        (self.restored / "alias").unlink()
        (self.restored / "alias").symlink_to("different")
        (self.restored / "hardlink").unlink()
        (self.restored / "hardlink").write_bytes(b"altered")
        (self.restored / "added").write_text("new")
        (self.restored / "untracked\nname").unlink()
        changed = subject.compare_manifests(left, self.scan(self.restored))
        self.assertFalse(changed["matching"])
        differences = {row["path"]: row["fields"] for row in changed["differences"]}
        self.assertEqual(differences["added"], ["added-in-restored"])
        self.assertEqual(differences["untracked\nname"], ["missing-from-restored"])
        self.assertIn("content", differences["alias"])
        self.assertTrue({"content", "mode", "xattrs", "hardlink"}.issubset(differences["file"]))

    def test_partial_scan_never_matches_itself(self):
        result = self.scan(byte_limit=1)
        self.assertFalse(result["complete"])
        self.assertFalse(subject.compare_manifests(result, result)["matching"])
        self.assertEqual((self.source / "file").read_bytes(), b"private-content")

    def test_special_files_root_links_and_mounts_are_not_read(self):
        os.mkfifo(self.source / "pipe")
        self.assertFalse(self.scan()["complete"])
        (self.source / "pipe").unlink()
        (self.root / "link").symlink_to(self.source)
        self.assertFalse(self.scan(self.root / "link")["complete"])
        with patch("evacuation_files.mountpoints", return_value={str(self.source / "file")}):
            self.assertFalse(self.scan()["complete"])

    def test_swapped_leaf_and_changed_content_are_incomplete(self):
        original = subject.os.open
        def replaced(name, flags, *args, **kwargs):
            if name == "file":
                return original(str(self.root / "other"), flags)
            return original(name, flags, *args, **kwargs)
        (self.root / "other").write_bytes(b"outside")
        with patch.object(subject.os, "open", side_effect=replaced):
            self.assertFalse(self.scan()["complete"])
        read = subject.os.read
        changed = False
        def mutate(fd, count):
            nonlocal changed
            block = read(fd, count)
            if block and not changed:
                changed = True
                (self.source / "file").write_bytes(b"replacement")
            return block
        with patch.object(subject.os, "read", side_effect=mutate):
            self.assertFalse(self.scan()["complete"])

    def test_external_hardlink_is_not_mistaken_for_an_internal_group(self):
        os.link(self.source / "file", self.root / "external-hardlink")
        shutil.copyfile(self.source / "file", self.restored / "file")
        self.assertTrue(subject.compare_manifests(self.scan(), self.scan(self.restored))["matching"])

    def test_owner_difference_and_incomplete_or_malformed_manifests(self):
        left = self.scan()
        right = self.roundtrip(left)
        right["entries"]["file"]["uid"] += 1
        self.assertEqual(subject.compare_manifests(left, right)["differences"], [{"path": "file", "fields": ["uid"]}])
        for change in [lambda v: v.update(complete="true"), lambda v: v.update(errors=[{}]),
                       lambda v: v["entries"]["file"].update(content=None),
                       lambda v: v["entries"].update({"../escape": v["entries"]["file"]}),
                       lambda v: v["entries"]["file"].update(hardlink=[]),
                       lambda v: v["entries"]["file"].update(uid=True)]:
            bad = json.loads(json.dumps(left))
            change(bad)
            with self.assertRaises(ValueError):
                self.roundtrip(bad)
        path = self.root / "malformed.json"
        path.write_text('{"format":1,"format":1}')
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()) as diagnostic:
            self.assertEqual(subject.main(["compare", str(path), str(path)]), 2)
        self.assertNotIn(str(path), diagnostic.getvalue())


if __name__ == "__main__":
    unittest.main()

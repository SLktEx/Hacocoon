import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import evacuation_files as subject


@unittest.skipUnless(hasattr(os, "O_NOFOLLOW") and Path("/proc/self/mountinfo").exists(), "Linux metadata observation required")
class FileInventoryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def inventory(self, **kw):
        return subject.file_inventory(str(self.root), **kw)

    def test_manual_git_files_and_hardlinks_without_contents(self):
        (self.root / ".git").mkdir()
        (self.root / ".git" / "HEAD").write_text("private-content-marker")
        file = self.root / "untracked\nname"
        file.write_text("private-content-marker")
        os.link(file, self.root / "hardlink")
        result = self.inventory()
        self.assertTrue(result["enumeration_complete"])
        self.assertFalse(result["backup_complete"])
        records = {x["path"]: x for x in result["entries"]}
        self.assertIn(".git/HEAD", records)
        self.assertEqual(records["hardlink"]["inode"], records[file.name]["inode"])
        self.assertEqual(records["hardlink"]["links"], 2)
        self.assertNotIn("private-content-marker", json.dumps(result))
        self.assertEqual(json.loads(json.dumps(result)), result)

    def test_symlink_and_fifo_are_reported_without_following(self):
        (self.root / "outside").symlink_to("/proc")
        os.mkfifo(self.root / "pipe")
        result = self.inventory()
        self.assertFalse(result["enumeration_complete"])
        self.assertEqual({x["path"] for x in result["entries"]}, {".", "outside", "pipe"})
        self.assertEqual(len(result["deferred"]), 2)

    def test_root_symlink_component_is_refused(self):
        (self.root / "real").mkdir()
        (self.root / "real" / "nested").mkdir()
        (self.root / "link").symlink_to(self.root / "real")
        result = subject.file_inventory(str(self.root / "link" / "nested"))
        self.assertFalse(result["enumeration_complete"])
        self.assertEqual(result["entries"], [])

    def test_same_filesystem_mount_is_explicitly_deferred(self):
        (self.root / "mounted").mkdir()
        (self.root / "mounted" / "hidden").write_text("not traversed")
        with patch.object(subject, "mountpoints", return_value={str(self.root / "mounted")}):
            result = self.inventory()
        self.assertFalse(result["enumeration_complete"])
        self.assertNotIn("mounted/hidden", {x["path"] for x in result["entries"]})
        self.assertEqual(result["deferred"][0]["reason"], "separate-mount-or-filesystem")

    def test_opened_mount_identity_blocks_unlisted_bind_mount(self):
        child = self.root / "mounted"
        child.mkdir()
        (child / "hidden").write_text("not traversed")
        actual = subject.mount_id
        def identity(fd):
            return actual(fd) + (1 if os.fstat(fd).st_ino == child.stat().st_ino else 0)
        with patch.object(subject, "mountpoints", return_value=set()), patch.object(subject, "mount_id", side_effect=identity):
            result = self.inventory()
        self.assertFalse(result["enumeration_complete"])
        self.assertNotIn("mounted/hidden", {x["path"] for x in result["entries"]})
        self.assertEqual(result["deferred"][0]["reason"], "separate-mount-or-filesystem")

    def test_limits_keep_partial_inventory(self):
        for name in ["a", "b", "c"]:
            (self.root / name).write_text("data")
        result = self.inventory(entry_limit=2)
        self.assertFalse(result["enumeration_complete"])
        self.assertEqual(len(result["entries"]), 2)
        self.assertEqual(result["errors"][0]["reason"], "enumeration-budget-exhausted")
        self.assertFalse(self.inventory(seconds=0)["enumeration_complete"])

    def test_unreadable_child_keeps_siblings(self):
        child = self.root / "unreadable"
        child.mkdir()
        (self.root / "readable").write_text("data")
        original = os.scandir
        def denied(fd):
            if os.fstat(fd).st_ino == child.stat().st_ino:
                raise PermissionError("do not expose raw exception")
            return original(fd)
        with patch.object(subject.os, "scandir", side_effect=denied):
            result = self.inventory()
        self.assertFalse(result["enumeration_complete"])
        self.assertIn("readable", {x["path"] for x in result["entries"]})
        self.assertIn({"path": "unreadable", "reason": "directory-unreadable"}, result["errors"])
        self.assertNotIn("raw exception", json.dumps(result))

    def test_mount_table_change_and_depth_limit_are_incomplete(self):
        with patch.object(subject, "mountpoints", side_effect=[set(), {"/new"}]):
            result = self.inventory()
        self.assertFalse(result["enumeration_complete"])
        (self.root / "nested").mkdir()
        self.assertFalse(self.inventory(depth_limit=0)["enumeration_complete"])

    def test_replaced_directory_is_not_traversed(self):
        (self.root / "child").mkdir()
        replacement = self.root / "replacement"
        replacement.mkdir()
        (replacement / "marker").write_text("outside contents")
        original = os.open
        def swapped(name, flags, *args, **kwargs):
            if name == "child":
                return original(str(replacement), flags)
            return original(name, flags, *args, **kwargs)
        with patch.object(subject.os, "open", side_effect=swapped):
            result = self.inventory()
        self.assertFalse(result["enumeration_complete"])
        self.assertNotIn("child/marker", {x["path"] for x in result["entries"]})
        self.assertIn({"path": "child", "reason": "directory-replaced"}, result["errors"])


if __name__ == "__main__":
    unittest.main()

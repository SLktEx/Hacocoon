import importlib.util
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location("tooling", sys.argv.pop(1))
tooling = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tooling)


class HostToolingFastPathTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

        original_lstat = tooling.os.lstat
        original_fstat = tooling.os.fstat

        def owner(info):
            values = list(info)
            values[4] = 0
            return os.stat_result(values)

        for name, original in (("lstat", original_lstat), ("fstat", original_fstat)):
            patch = mock.patch.object(tooling.os, name, lambda arg, fn=original: owner(fn(arg)))
            patch.start()
            self.addCleanup(patch.stop)

    def payload(self):
        paths = []
        for index in range(len(tooling.BINARIES) + len(tooling.CNI)):
            path = self.root / f"tool-{index}"
            path.write_bytes((f"payload-{index}" * 128).encode())
            path.chmod(0o755)
            paths.append(str(path))
        return tuple(paths)

    def stamp(self, paths, arch="amd64"):
        stamp = self.root / "tooling.complete"
        with mock.patch.object(tooling, "tooling_targets", return_value=paths), \
                mock.patch.object(tooling, "tooling_stamp_path", return_value=str(stamp)):
            stamp.write_bytes(tooling.tooling_manifest(arch))
            stamp.chmod(0o644)
        return stamp

    def test_exact_stamp_reuses_payload_without_archive_revalidation(self):
        paths = self.payload()
        stamp = self.stamp(paths)
        with mock.patch.object(tooling, "architecture", return_value="amd64"), \
                mock.patch.object(tooling, "tooling_targets", return_value=paths), \
                mock.patch.object(tooling, "tooling_stamp_path", return_value=str(stamp)), \
                mock.patch.object(tooling, "download") as download, \
                mock.patch.object(tooling, "run") as run:
            tooling.tooling()
            download.assert_not_called()
            self.assertEqual(run.call_count, len(tooling.BINARIES))

    def test_changed_or_missing_payload_cannot_take_the_fast_path(self):
        paths = self.payload()
        stamp = self.stamp(paths)
        with mock.patch.object(tooling, "tooling_targets", return_value=paths), \
                mock.patch.object(tooling, "tooling_stamp_path", return_value=str(stamp)), \
                mock.patch.object(tooling, "run"):
            self.assertTrue(tooling.tooling_ready("amd64"))
            Path(paths[0]).write_bytes(b"changed")
            Path(paths[0]).chmod(0o755)
            self.assertFalse(tooling.tooling_ready("amd64"))
            Path(paths[0]).unlink()
            self.assertFalse(tooling.tooling_ready("amd64"))

    def test_archive_digest_pin_change_invalidates_the_stamp(self):
        paths = self.payload()
        stamp = self.stamp(paths)
        changed = dict(tooling.DIGESTS)
        changed["amd64"] = "0" * 64
        with mock.patch.object(tooling, "tooling_targets", return_value=paths), \
                mock.patch.object(tooling, "tooling_stamp_path", return_value=str(stamp)), \
                mock.patch.object(tooling, "DIGESTS", changed), \
                mock.patch.object(tooling, "run"):
            self.assertFalse(tooling.tooling_ready("amd64"))

    def test_safe_owned_completion_stamp_can_be_refreshed_after_full_validation(self):
        stamp = self.root / "tooling.complete"
        stamp.write_bytes(b"old-format\n")
        stamp.chmod(0o644)
        with mock.patch.object(tooling, "directory"):
            tooling.publish_completion_stamp(str(stamp), b"new-format\n")
        self.assertEqual(stamp.read_bytes(), b"new-format\n")
        self.assertEqual(stamp.stat().st_mode & 0o777, 0o644)

    def test_unsafe_payload_fails_closed_even_with_a_valid_stamp(self):
        paths = self.payload()
        stamp = self.stamp(paths)
        Path(paths[-1]).chmod(0o777)
        with mock.patch.object(tooling, "tooling_targets", return_value=paths), \
                mock.patch.object(tooling, "tooling_stamp_path", return_value=str(stamp)), \
                mock.patch.object(tooling, "run"):
            with self.assertRaises(ValueError):
                tooling.tooling_ready("amd64")

    def test_missing_stamp_uses_full_reconciliation_path(self):
        paths = self.payload()
        missing = self.root / "missing.complete"
        with mock.patch.object(tooling, "tooling_targets", return_value=paths), \
                mock.patch.object(tooling, "tooling_stamp_path", return_value=str(missing)), \
                mock.patch.object(tooling, "run"):
            self.assertFalse(tooling.tooling_ready("amd64"))


if __name__ == "__main__":
    unittest.main()

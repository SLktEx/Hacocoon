import contextlib
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import evacuation_capture as subject


@unittest.skipUnless(hasattr(os, "O_NOFOLLOW"), "Linux directory identities required")
class CapturePreflightTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / "source"
        self.output = self.root / "output"
        self.source.mkdir(mode=0o700)
        self.output.mkdir(mode=0o700)
        (self.source / "work").write_text("retained")

    def test_invalid_budgets_never_execute(self):
        with patch.object(subject.subprocess, "Popen", side_effect=AssertionError("no process")):
            for options in ({"byte_limit": 0}, {"seconds": 0}, {"seconds": 86401}):
                with self.assertRaises(ValueError):
                    subject.capture_tree(str(self.source), str(self.output), **options)
        self.assertEqual(list(self.output.iterdir()), [])

    def test_unknown_output_is_never_overwritten(self):
        marker = self.output / "existing"
        marker.write_text("keep")
        with self.assertRaisesRegex(subject.CaptureError, "empty and private"):
            subject.capture_tree(str(self.source), str(self.output))
        self.assertEqual(marker.read_text(), "keep")
        self.assertEqual(len(list(self.output.iterdir())), 1)

    def test_symlink_source_and_destination_inside_source_refused(self):
        link = self.root / "link"
        link.symlink_to(self.source, target_is_directory=True)
        with self.assertRaises(OSError):
            subject.capture_tree(str(link), str(self.output))
        with self.assertRaises(ValueError):
            subject.capture_tree(str(self.source), str(self.source / "output"))

    def test_aliased_destination_identity_is_refused_before_output(self):
        value = self.source.stat()
        with patch.object(subject, "_identity", return_value=(value.st_dev, value.st_ino)):
            with self.assertRaisesRegex(subject.CaptureError, "inside source"):
                subject.capture_tree(str(self.source), str(self.output))
        self.assertEqual(list(self.output.iterdir()), [])

    def test_special_files_and_incomplete_inventory_refused_before_capture(self):
        os.mkfifo(self.source / "fifo")
        with patch.object(subject.subprocess, "Popen", side_effect=AssertionError("no process")):
            with self.assertRaisesRegex(subject.CaptureError, "separate review"):
                subject.capture_tree(str(self.source), str(self.output))
        self.assertEqual(list(self.output.iterdir()), [])

    def test_launch_failure_retains_intent_and_never_writes_completion(self):
        with patch.object(subject.subprocess, "Popen", side_effect=OSError("unavailable")):
            with self.assertRaises(OSError):
                subject.capture_tree(str(self.source), str(self.output))
        self.assertTrue((self.output / "capture-intent.json").is_file())
        self.assertTrue((self.output / "data.tar").is_file())
        self.assertFalse((self.output / "capture-complete.json").exists())
        self.assertFalse((self.output / "data.tar.sha256").exists())
        self.assertEqual((self.source / "work").read_text(), "retained")


class CaptureCommandTests(unittest.TestCase):
    def test_quiescence_confirmation_is_required_before_capture(self):
        with patch.object(subject, "capture_tree") as capture, contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit) as raised:
                subject.main(["/source", "/destination"])
        self.assertEqual(raised.exception.code, 2)
        capture.assert_not_called()

    def test_completion_output_remains_explicitly_partial(self):
        result = {"archive_complete": True, "backup_complete": False}
        stdout = io.StringIO()
        with patch.object(subject, "capture_tree", return_value=result) as capture, contextlib.redirect_stdout(stdout):
            self.assertEqual(subject.main(["/source", "/destination", "--quiesced", "--byte-limit", "100", "--seconds", "20"]), 0)
        capture.assert_called_once_with("/source", "/destination", byte_limit=100, seconds=20)
        self.assertEqual(json.loads(stdout.getvalue()), result)

    def test_failure_is_nonzero_and_does_not_disclose_exception_data(self):
        for error in (subject.CaptureError("private source"), OSError("private path"), ValueError("private argument"), subject.subprocess.TimeoutExpired("private command", 1)):
            stdout, stderr = io.StringIO(), io.StringIO()
            with self.subTest(error=type(error).__name__), patch.object(subject, "capture_tree", side_effect=error), contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                self.assertEqual(subject.main(["/source", "/destination", "--quiesced"]), 1)
            self.assertEqual(stdout.getvalue(), "")
            self.assertIn("Capture failed", stderr.getvalue())
            self.assertNotIn("private", stderr.getvalue())


if __name__ == "__main__":
    unittest.main()

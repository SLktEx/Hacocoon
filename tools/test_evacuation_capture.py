import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import evacuation_capture as subject

RECIPIENT = "age1" + "q" * 58


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

    def test_invalid_recipient_or_budgets_never_execute(self):
        with patch.object(subject.subprocess, "Popen", side_effect=AssertionError("no process")):
            for recipient in ("-plugin", "age1\nsecret", "ssh-rsa secret"):
                with self.assertRaises(ValueError):
                    subject.capture_tree(str(self.source), str(self.output), recipient)
            with self.assertRaises(ValueError):
                subject.capture_tree(str(self.source), str(self.output), RECIPIENT, byte_limit=0)
        self.assertEqual(list(self.output.iterdir()), [])

    def test_unknown_output_is_never_overwritten(self):
        marker = self.output / "existing"
        marker.write_text("keep")
        with self.assertRaisesRegex(subject.CaptureError, "empty and private"):
            subject.capture_tree(str(self.source), str(self.output), RECIPIENT)
        self.assertEqual(marker.read_text(), "keep")
        self.assertEqual(len(list(self.output.iterdir())), 1)

    def test_symlink_source_and_destination_inside_source_refused(self):
        link = self.root / "link"
        link.symlink_to(self.source, target_is_directory=True)
        with self.assertRaises(OSError):
            subject.capture_tree(str(link), str(self.output), RECIPIENT)
        with self.assertRaises(ValueError):
            subject.capture_tree(str(self.source), str(self.source / "output"), RECIPIENT)

    def test_aliased_destination_identity_is_refused_before_output(self):
        value = self.source.stat()
        with patch.object(subject, "_identity", return_value=(value.st_dev, value.st_ino)):
            with self.assertRaisesRegex(subject.CaptureError, "inside source"):
                subject.capture_tree(str(self.source), str(self.output), RECIPIENT)
        self.assertEqual(list(self.output.iterdir()), [])

    def test_special_files_and_incomplete_inventory_refused_before_capture(self):
        os.mkfifo(self.source / "fifo")
        with patch.object(subject.subprocess, "Popen", side_effect=AssertionError("no process")):
            with self.assertRaisesRegex(subject.CaptureError, "separate review"):
                subject.capture_tree(str(self.source), str(self.output), RECIPIENT)
        self.assertEqual(list(self.output.iterdir()), [])

    def test_launch_failure_retains_intent_and_never_writes_completion(self):
        with patch.object(subject.subprocess, "Popen", side_effect=OSError("unavailable")):
            with self.assertRaises(OSError):
                subject.capture_tree(str(self.source), str(self.output), RECIPIENT)
        self.assertTrue((self.output / "capture-intent.json").is_file())
        self.assertTrue((self.output / "data.tar.age").is_file())
        self.assertFalse((self.output / "capture-complete.json").exists())
        self.assertEqual((self.source / "work").read_text(), "retained")


if __name__ == "__main__":
    unittest.main()

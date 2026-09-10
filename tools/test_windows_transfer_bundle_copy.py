"""Execute the Windows transfer fixture's exact exclusive-copy script locally."""
import pathlib
import re
import subprocess
import sys
import tempfile
import unittest

class BundleCopyTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        text = (pathlib.Path(__file__).parent / "test_windows_environment_transfer.ps1").read_text(encoding="utf-8")
        match = re.search(r'\$copyBundle = @"\n(.*?)\n"@', text, re.S)
        if not match:
            raise AssertionError("copy script missing")
        cls.script = match.group(1)

    def test_copy_and_existing_target_refusal(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            source, target = root / "source bytes.haco", root / "saved bytes.haco"
            source.write_bytes(b"fixture bundle\x00data")
            result = subprocess.run([sys.executable, "-c", self.script, str(source), str(target)], capture_output=True)
            self.assertEqual(result.returncode, 0)
            self.assertEqual(target.read_bytes(), source.read_bytes())
            target.write_bytes(b"retained target")
            result = subprocess.run([sys.executable, "-c", self.script, str(source), str(target)], capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(target.read_bytes(), b"retained target")
            self.assertEqual(source.read_bytes(), b"fixture bundle\x00data")

    def test_target_symlink_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            source, target, retained = root / "source", root / "target", root / "retained"
            source.write_bytes(b"source")
            retained.write_bytes(b"keep")
            try:
                target.symlink_to(retained)
            except OSError:
                self.skipTest("symlink creation unavailable on this host")
            result = subprocess.run([sys.executable, "-c", self.script, str(source), str(target)], capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(retained.read_bytes(), b"keep")
            self.assertTrue(target.is_symlink())

if __name__ == "__main__":
    unittest.main()

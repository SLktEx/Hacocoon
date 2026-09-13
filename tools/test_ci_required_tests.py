#!/usr/bin/env python3
import unittest
import os
from pathlib import Path
import subprocess
import sys
from ci_required_tests import verify


class RequiredTests(unittest.TestCase):
    def test_command_exit_and_real_top_level_receipt_are_both_required(self):
        wrapper = str(Path(__file__).with_name("ci_required_tests.py"))
        for output, status, expected in (
            ("--- PASS: TestContract (0.00s)", 0, 0),
            ("--- SKIP: TestContract (0.00s)", 0, 1),
            ("    --- PASS: TestContract (0.00s)", 0, 1),
            ("--- PASS: TestContract (0.00s)", 1, 1),
        ):
            result = subprocess.run(
                [sys.executable, wrapper, "--expect", "TestContract", "--", sys.executable,
                 "-c", "import sys; print(sys.argv[1]); sys.exit(int(sys.argv[2]))", output, str(status)],
                capture_output=True, text=True, timeout=10,
                env=dict(os.environ, GITHUB_ACTIONS="false"))
            self.assertEqual(result.returncode, expected, result.stderr)

    def test_require_actual_execution(self):
        for results in ({}, {"TestOther": ["PASS"]}, {"TestContract": ["SKIP"]},
                        {"TestContract": ["FAIL", "PASS"]}, {"TestContract": ["PASS", "PASS"]}):
            self.assertFalse(verify(["TestContract"], results, 0))
        self.assertFalse(verify(["TestContract"], {"TestContract": ["PASS"]}, 1))
        self.assertFalse(verify([], {}, 0))
        self.assertTrue(verify(["TestContract"], {"TestContract": ["PASS"]}, 0))


if __name__ == "__main__":
    unittest.main()

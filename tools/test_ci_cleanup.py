#!/usr/bin/env python3
"""Regression: failed observation may never authorize a destructive cleanup."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

LIBRARY = Path(__file__).with_name("ci-incus-cleanup-library.sh").resolve()


class CleanupTests(unittest.TestCase):
    def check_case(self, mode, expected, deleted, project="hacocoon"):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            command = root / "incus"
            command.write_text('''#!/bin/bash
case "$1 $2" in
  'list --project')
    case "$MODE" in
      list-failed) exit 1 ;;
      unexpected) echo unrelated-instance ;;
      malformed-instance) echo 'haco-owned:other' ;;
      *) echo haco-owned ;;
    esac ;;
  'project delete') touch "$MARKER" ;;
  'project list')
    case "$MODE" in
      absence-unknown) exit 1 ;;
      absence-empty) exit 0 ;;
      absence-malformed) echo 'backend error: default' ;;
      remains) echo hacocoon ;;
      *) echo default ;;
    esac ;;
  *) exit 2 ;;
esac
''')
            command.chmod(0o755)
            env = dict(os.environ, PATH=str(root) + os.pathsep + os.environ["PATH"],
                       MODE=mode, MARKER=str(root / "deleted"))
            result = subprocess.run(["bash", "-c", 'set -euo pipefail; source "$1"; ci_delete_project "$2"', "test", str(LIBRARY), project], env=env, capture_output=True, timeout=10)
            self.assertEqual(result.returncode == 0, expected, result.stderr)
            self.assertEqual((root / "deleted").exists(), deleted)

    def test_failed_inventory_blocks_delete(self):
        self.check_case("list-failed", False, False)

    def test_unexpected_owner_blocks_delete(self):
        self.check_case("unexpected", False, False)
        self.check_case("malformed-instance", False, False)

    def test_remote_or_malformed_target_never_reaches_delete(self):
        for project in ("haco-e2e-remote:default", "haco-e2e-../default", "--force", "haco-e2e-a\ndefault"):
            with self.subTest(project=project):
                self.check_case("absent", False, False, project)

    def test_cleanup_requires_positive_absence(self):
        self.check_case("absence-unknown", False, True)
        self.check_case("absence-empty", False, True)
        self.check_case("absence-malformed", False, True)
        self.check_case("remains", False, True)
        self.check_case("absent", True, True)


if __name__ == "__main__":
    unittest.main()

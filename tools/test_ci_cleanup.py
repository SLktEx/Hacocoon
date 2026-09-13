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


class StorageCleanupTests(unittest.TestCase):
    def test_uncertainty_stops_before_storage_or_project_deletion(self):
        source = LIBRARY.with_name("ci-incus-storage-cli.sh").read_text()
        functions = source[source.index("delete_owned_instances() {"):source.index('case "${1:-}" in')]
        for mode in ("projects-failed", "pools-failed", "instances-failed", "unexpected", "malformed", "delete-failed", "remains", "images-failed", "images-malformed", "absent"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                script = root / "test.sh"
                script.write_text('''#!/bin/bash
set -euo pipefail
PROJECT=hacocoon INSTANCE=haco-test POOL=haco-local-default
INCUS_BACKING=/not-a-real-backing CLI_ROOT=unused WORKSPACE=unused RUN_WORKSPACE=unused
HACO_BIN=unused CONTROLLER_BIN=unused
require_github_hosted_runner() { :; }
haco_stop_test_controller() { :; }
fail() { echo "$*" >&2; exit 1; }
sudo() { return 0; }
rm() { :; }
incus() {
  case "$1 $2" in
    'project list')
      [[ "$MODE" != projects-failed ]] || return 1
      echo default
      [[ -f "$STATE/project" ]] || echo hacocoon ;;
    'storage list')
      [[ "$MODE" != pools-failed ]] || return 1
      [[ -f "$STATE/pool" ]] || echo haco-local-default ;;
    'list --project')
      [[ "$MODE" != instances-failed ]] || return 1
      if [[ ! -f "$STATE/instance" || "$MODE" == remains ]]; then echo haco-test; fi
      case "$MODE" in
        unexpected) echo haco-aggregate-failure ;;
        malformed) echo 'haco-run-remote:other' ;;
      esac ;;
    'delete haco-test')
      echo instance >> "$STATE/deleted"
      [[ "$MODE" != delete-failed ]] || return 1
      touch "$STATE/instance" ;;
    'image list')
      [[ "$MODE" != images-failed ]] || return 1
      if [[ "$MODE" == images-malformed ]]; then echo 'remote:other'; return 0; fi
      if [[ ! -f "$STATE/image" ]]; then
        if [[ "${!#}" == F ]]; then printf '%064d\n' 1; else printf '%012d\n' 1; fi
      fi ;;
    'image delete') echo image >> "$STATE/deleted"; touch "$STATE/image" ;;
    'storage delete') echo pool >> "$STATE/deleted"; touch "$STATE/pool" ;;
    'project delete') echo project >> "$STATE/deleted"; touch "$STATE/project" ;;
    *) echo "unexpected command: $*" >&2; return 2 ;;
  esac
  return 0
}
''' + functions + "\ncleanup\n")
                result = subprocess.run(["bash", str(script)], env=dict(os.environ, MODE=mode, STATE=str(root)), capture_output=True, timeout=10)
                deleted = (root / "deleted").read_text().splitlines() if (root / "deleted").exists() else []
                self.assertEqual(result.returncode == 0, mode == "absent", result.stderr)
                if mode == "absent":
                    self.assertEqual(deleted, ["instance", "image", "pool", "project"])
                else:
                    self.assertNotIn("pool", deleted)
                    self.assertNotIn("project", deleted)
                    if mode in ("unexpected", "malformed"):
                        self.assertEqual(deleted, [])


if __name__ == "__main__":
    unittest.main()

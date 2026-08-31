#!/usr/bin/env python3
"""Regression tests for repository checkpoint-bump serialization."""

from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "tools"))

from checkpoint_source import parse_checkpoint_source  # noqa: E402
from milestone_lock import (  # noqa: E402
    FALLBACK_LOCK_NAME,
    LOCK_NAME,
    LockHeldError,
    acquire_lock,
    milestone_lock_path,
    release_lock,
)

MIRROR_PATHS = (
    "docs/status/checkpoints.yaml",
    "docs/status/versioning-and-release-status.md",
    "docs/status/versioning-and-release-status.ja.md",
    "docs/IMPLEMENTATION_STATUS.md",
    "docs/IMPLEMENTATION_STATUS.ja.md",
    "internal/buildinfo/checkpoint_generated.go",
)


def copy_repo(destination: Path) -> Path:
    repo = destination / "repo"
    shutil.copytree(
        ROOT,
        repo,
        ignore=shutil.ignore_patterns(
            ".git",
            "dist",
            "__pycache__",
            ".pytest_cache",
            ".mypy_cache",
            FALLBACK_LOCK_NAME,
        ),
    )
    return repo


def run(repo: Path, *argv: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        list(argv),
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )


class MilestoneLockUnitTest(unittest.TestCase):
    def test_lock_is_exclusive_and_release_allows_reacquire(self):
        with tempfile.TemporaryDirectory(prefix="haco-milestone-lock-unit-") as temp:
            root = Path(temp)
            first = acquire_lock(root)
            self.assertEqual(first, root / FALLBACK_LOCK_NAME)
            self.assertTrue(first.is_dir())

            with self.assertRaises(LockHeldError) as caught:
                acquire_lock(root)
            message = str(caught.exception)
            self.assertIn("checkpoint bump already in progress", message)
            self.assertIn("pid=", message)
            self.assertIn("started=", message)
            self.assertIn(f"root={root.resolve()}", message)
            self.assertIn("remove this stale lock directory manually", message)

            release_lock(first)
            second = acquire_lock(root)
            self.assertEqual(second, first)
            release_lock(second)
            self.assertFalse(first.exists())

    def test_git_metadata_directory_hosts_lock(self):
        with tempfile.TemporaryDirectory(prefix="haco-milestone-lock-git-") as temp:
            root = Path(temp)
            git_dir = root / ".git"
            git_dir.mkdir()
            lock = acquire_lock(root)
            self.assertEqual(lock, git_dir / LOCK_NAME)
            release_lock(lock)


class MilestoneLockBlackBoxTest(unittest.TestCase):
    def test_failed_real_wrapper_releases_lock(self):
        with tempfile.TemporaryDirectory(prefix="haco-milestone-lock-failure-") as temp:
            repo = copy_repo(Path(temp))
            source = parse_checkpoint_source(repo / "docs/status/checkpoints.yaml")
            current_minor = int(source.current.removeprefix("v0."))
            skipped_checkpoint = f"v0.{current_minor + 2}"

            result = run(
                repo,
                "tools/bump-milestone",
                skipped_checkpoint,
                "Rejected Gate",
            )
            output = result.stdout + result.stderr
            self.assertNotEqual(result.returncode, 0, output)
            self.assertIn(
                f"next checkpoint must be v0.{current_minor + 1}", output
            )
            self.assertFalse(milestone_lock_path(repo).exists())

    def test_second_real_wrapper_fails_while_first_holds_lock(self):
        with tempfile.TemporaryDirectory(prefix="haco-milestone-lock-e2e-") as temp:
            repo = copy_repo(Path(temp))
            source = parse_checkpoint_source(repo / "docs/status/checkpoints.yaml")
            current_minor = int(source.current.removeprefix("v0."))
            next_checkpoint = f"v0.{current_minor + 1}"

            checker = repo / "tools/check_docs.py"
            checker.write_text(
                """#!/usr/bin/env python3
from pathlib import Path
import sys
import time

entered = Path('.milestone-checker-entered')
release = Path('.milestone-checker-release')
entered.write_text('ready\\n', encoding='utf-8')
deadline = time.monotonic() + 20
while not release.exists():
    if time.monotonic() >= deadline:
        print('timed out waiting for test release', file=sys.stderr)
        raise SystemExit(99)
    time.sleep(0.05)
print('DOC CONSISTENCY OK')
""",
                encoding="utf-8",
            )

            first = subprocess.Popen(
                ["tools/bump-milestone", next_checkpoint, "Concurrent Gate A"],
                cwd=repo,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
            release_marker = repo / ".milestone-checker-release"
            try:
                entered_marker = repo / ".milestone-checker-entered"
                deadline = time.monotonic() + 30
                while not entered_marker.exists():
                    if first.poll() is not None:
                        stdout, stderr = first.communicate()
                        self.fail(
                            "first bump exited before validator barrier: "
                            f"rc={first.returncode}\\n{stdout}{stderr}"
                        )
                    if time.monotonic() >= deadline:
                        self.fail("timed out waiting for first bump validator barrier")
                    time.sleep(0.05)

                lock_path = milestone_lock_path(repo)
                self.assertTrue(lock_path.is_dir(), lock_path)
                before_second = {
                    relative: (repo / relative).read_bytes()
                    for relative in MIRROR_PATHS
                }

                second = run(
                    repo,
                    "tools/bump-milestone",
                    next_checkpoint,
                    "Concurrent Gate B",
                )
                output = second.stdout + second.stderr
                self.assertNotEqual(second.returncode, 0, output)
                self.assertIn("checkpoint bump already in progress", output)
                self.assertIn("pid=", output)
                self.assertIn("remove this stale lock directory manually", output)

                after_second = {
                    relative: (repo / relative).read_bytes()
                    for relative in MIRROR_PATHS
                }
                self.assertEqual(before_second, after_second)
                self.assertIsNone(first.poll(), "first bump should still own the lock")
            finally:
                release_marker.write_text("release\n", encoding="utf-8")

            stdout, stderr = first.communicate(timeout=30)
            output = stdout + stderr
            self.assertEqual(first.returncode, 0, output)
            self.assertIn(f"checkpoint advanced to {next_checkpoint}", output)
            self.assertFalse(milestone_lock_path(repo).exists())

            source = parse_checkpoint_source(repo / "docs/status/checkpoints.yaml")
            self.assertEqual(source.current, next_checkpoint)
            self.assertEqual(source.milestones[-1].gate, "Concurrent Gate A")


if __name__ == "__main__":
    unittest.main(verbosity=2)

#!/usr/bin/env python3
"""Black-box regression tests for tools/check_docs.py.

The suite copies the checked-out repository once, introduces one intentional
consistency defect at a time, and requires the real checker to reject it.
"""

from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "tools"))

from checkpoint_source import parse_checkpoint_source  # noqa: E402


class CheckDocsRegressionTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tempdir = tempfile.TemporaryDirectory(prefix="haco-check-docs-")
        cls.repo = Path(cls.tempdir.name) / "repo"
        shutil.copytree(
            ROOT,
            cls.repo,
            ignore=shutil.ignore_patterns(
                ".git",
                "dist",
                "bin",
                "__pycache__",
                ".pytest_cache",
                ".mypy_cache",
            ),
        )
        cls.source = parse_checkpoint_source(cls.repo / "docs/status/checkpoints.yaml")
        cls.current = cls.source.current
        cls.current_minor = int(cls.current.removeprefix("v0."))
        if cls.current_minor <= 1:
            raise RuntimeError("regression fixtures require a checkpoint after v0.1")
        cls.previous = f"v0.{cls.current_minor - 1}"
        cls.current_gate = cls.source.milestones[-1].gate

        baseline = cls.run_checker()
        if baseline.returncode != 0:
            raise RuntimeError(
                "check_docs.py baseline must pass before negative tests:\n"
                + baseline.stdout
                + baseline.stderr
            )

    @classmethod
    def tearDownClass(cls):
        cls.tempdir.cleanup()

    @classmethod
    def run_checker(cls):
        return subprocess.run(
            [sys.executable, "tools/check_docs.py"],
            cwd=cls.repo,
            text=True,
            capture_output=True,
            check=False,
        )

    def mutate_and_require_failure(self, relative_path, mutate, expected):
        path = self.repo / relative_path
        original = path.read_text()
        mutated = mutate(original)
        self.assertNotEqual(original, mutated, f"fixture did not mutate {relative_path}")
        try:
            path.write_text(mutated)
            result = self.run_checker()
            output = result.stdout + result.stderr
            self.assertNotEqual(
                result.returncode,
                0,
                f"checker unexpectedly accepted broken fixture {relative_path}",
            )
            self.assertIn(expected, output)
        finally:
            path.write_text(original)

    def test_baseline_repository_passes(self):
        result = self.run_checker()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("DOC CONSISTENCY OK", result.stdout)

    def test_rejects_stale_japanese_current_checkpoint(self):
        self.mutate_and_require_failure(
            "docs/status/versioning-and-release-status.ja.md",
            lambda text: text.replace(
                f"現在のmilestone位置は **{self.current}**",
                f"現在のmilestone位置は **{self.previous}**",
                1,
            ),
            "current milestone mirror",
        )

    def test_rejects_checkpoint_gate_table_drift(self):
        self.mutate_and_require_failure(
            "docs/status/versioning-and-release-status.md",
            lambda text: text.replace(
                f"| {self.current} | {self.current_gate} |",
                f"| {self.current} | Intentional Drift Gate |",
                1,
            ),
            "checkpoint table does not mirror docs/status/checkpoints.yaml",
        )

    def test_rejects_stale_generated_build_checkpoint(self):
        self.mutate_and_require_failure(
            "internal/buildinfo/checkpoint_generated.go",
            lambda text: text.replace(
                f'const GeneratedCheckpoint = "{self.current}"',
                f'const GeneratedCheckpoint = "{self.previous}"',
                1,
            ),
            "generated checkpoint",
        )

    def test_rejects_concrete_checkpoint_copy_in_readme(self):
        self.mutate_and_require_failure(
            "README.md",
            lambda text: text + f"\nCurrent development checkpoint: {self.current}.\n",
            "concrete checkpoint numbers belong in version/status authorities",
        )

    def test_rejects_stale_yaml_current(self):
        self.mutate_and_require_failure(
            "docs/status/checkpoints.yaml",
            lambda text: re.sub(
                rf'^current: "{re.escape(self.current)}"$',
                f'current: "{self.previous}"',
                text,
                count=1,
                flags=re.MULTILINE,
            ),
            "must equal newest milestone",
        )

    def test_rejects_non_contiguous_yaml(self):
        if self.current_minor <= 2:
            self.skipTest("requires at least v0.3")
        missing_prefix = f'  "{self.previous}": '

        def remove_previous(text):
            lines = text.splitlines(keepends=True)
            return "".join(line for line in lines if not line.startswith(missing_prefix))

        self.mutate_and_require_failure(
            "docs/status/checkpoints.yaml",
            remove_previous,
            "milestones must be contiguous from v0.1",
        )


if __name__ == "__main__":
    unittest.main(verbosity=2)

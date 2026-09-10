"""Exercise push-fixture guards without any Git or network operations."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]

class PushTargetTest(unittest.TestCase):
    def run_fixture(self, event, unavailable=False):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            calls = root / "git-calls"
            for name, body in {
                "go": "exit 0\n",
                "gh": "exit 1\n" if unavailable else "echo 'gh: Not Found (HTTP 404)' >&2\nexit 1\n",
                "git": 'printf "%s\\n" "$@" > "$CALLS"\nexit 97\n',
            }.items():
                path = root / name
                path.write_text("#!/bin/sh\n" + body)
                path.chmod(0o700)
            env = {
                "PATH": str(root) + ":/usr/bin:/bin", "HOME": str(root),
                "GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": event,
                "GITHUB_REF": "refs/heads/main", "GITHUB_REPOSITORY": "SLktEx/Hacocoon",
                "GITHUB_RUN_ID": "123456", "GITHUB_RUN_ATTEMPT": "1",
                "HACO_GITHUB_TOKEN": "fixture-only", "CALLS": str(calls),
            }
            result = subprocess.run(["bash", str(ROOT / "test/e2e/git_github_real_push.sh")],
                                    env=env, capture_output=True, text=True, timeout=15)
            return result.returncode, calls.read_text().splitlines() if calls.exists() else []

    def test_source_repository_cannot_become_push_target(self):
        code, calls = self.run_fixture("workflow_dispatch")
        self.assertEqual(code, 97)
        self.assertEqual(calls[:3], ["clone", "-q", "https://github.com/SLktEx/Hacocoon-test.git"])

    def test_main_push_event_cannot_execute_git(self):
        code, calls = self.run_fixture("push")
        self.assertNotEqual(code, 0)
        self.assertEqual(calls, [])

    def test_unknown_remote_state_cannot_execute_git(self):
        code, calls = self.run_fixture("workflow_dispatch", unavailable=True)
        self.assertEqual(code, 1)
        self.assertEqual(calls, [])

    def test_workflow_has_no_automatic_or_repository_write_authority(self):
        text = (ROOT / ".github/workflows/real-git-push-e2e.yml").read_text()
        self.assertNotIn("  push:", text)
        self.assertNotIn("contents: write", text)
        self.assertNotIn("github.token", text)
        self.assertIn("workflow_dispatch:", text)
        self.assertIn("secrets.HACO_TEST_REPOSITORY_TOKEN", text)
        self.assertIn("SKIP real Git push", text)
        script = (ROOT / "test/e2e/git_github_real_push.sh").read_text()
        self.assertNotIn("--method DELETE", script)

if __name__ == "__main__":
    unittest.main()
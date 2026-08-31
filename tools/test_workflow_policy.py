#!/usr/bin/env python3
from __future__ import annotations

import unittest
from pathlib import Path

from check_workflow_policy import check_text
from test_workflow_policy_base import *  # noqa: F401,F403


class WindowsRunnerPolicyTests(unittest.TestCase):
    def test_accepts_pinned_windows_2025_github_hosted_runner(self) -> None:
        workflow = """name: windows-acceptance
on:
  pull_request:
permissions:
  contents: read
jobs:
  windows:
    runs-on: windows-2025
    steps:
      - run: Write-Host ok
"""
        self.assertEqual(check_text(Path("fixture.yml"), workflow), [])

    def test_rejects_mutable_windows_latest_runner(self) -> None:
        workflow = """name: windows-acceptance
on:
  pull_request:
permissions:
  contents: read
jobs:
  windows:
    runs-on: windows-latest
    steps:
      - run: Write-Host nope
"""
        found = [violation.message for violation in check_text(Path("fixture.yml"), workflow)]
        self.assertTrue(any("not approved" in message for message in found), found)


if __name__ == "__main__":
    unittest.main()

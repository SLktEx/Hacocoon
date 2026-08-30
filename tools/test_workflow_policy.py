#!/usr/bin/env python3
from __future__ import annotations

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from check_workflow_policy import check_text  # noqa: E402

SHA = "a" * 40


def messages(text: str) -> list[str]:
    return [violation.message for violation in check_text(Path("fixture.yml"), text)]


class WorkflowPolicyTests(unittest.TestCase):
    def assert_rejected(self, text: str, needle: str) -> None:
        found = messages(text)
        self.assertTrue(any(needle in message for message in found), found)

    def test_accepts_intended_pr_workflow(self) -> None:
        workflow = f"""name: test
on:
  pull_request:
permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-26.04
    steps:
      - uses: actions/checkout@{SHA}
        with:
          persist-credentials: false
      - uses: actions/setup-go@{SHA}
        with:
          cache: false
      - run: go test ./...
"""
        self.assertEqual(messages(workflow), [])

    def test_accepts_safe_inline_yaml_forms(self) -> None:
        workflow = f"""name: test
"on": [pull_request]
permissions: {{contents: read, id-token: none}}
jobs:
  test:
    runs-on: ubuntu-26.04
    steps:
      - uses: actions/checkout@{SHA}
        with: {{persist-credentials: false}}
      - uses: actions/setup-go@{SHA}
        with: {{cache: false}}
"""
        self.assertEqual(messages(workflow), [])

    def test_rejects_pull_request_target(self) -> None:
        self.assert_rejected(
            "on:\n  pull_request_target:\njobs:\n  test:\n    runs-on: ubuntu-26.04\n",
            "pull_request_target",
        )

    def test_rejects_inline_pull_request_target(self) -> None:
        self.assert_rejected(
            "on: [pull_request_target]\njobs:\n  test:\n    runs-on: ubuntu-26.04\n",
            "pull_request_target",
        )

    def test_rejects_workflow_run(self) -> None:
        self.assert_rejected(
            "on:\n  workflow_run:\njobs:\n  test:\n    runs-on: ubuntu-26.04\n",
            "workflow_run",
        )

    def test_rejects_write_permission_in_pr(self) -> None:
        self.assert_rejected(
            "on:\n  pull_request:\npermissions:\n  contents: write\njobs:\n  test:\n    runs-on: ubuntu-26.04\n",
            "must be read/none",
        )

    def test_rejects_inline_write_permission_in_pr(self) -> None:
        self.assert_rejected(
            "on: [pull_request]\npermissions: {contents: write}\njobs:\n  test:\n    runs-on: ubuntu-26.04\n",
            "must be read/none",
        )

    def test_rejects_write_all_permission_in_pr(self) -> None:
        self.assert_rejected(
            "on: pull_request\npermissions: write-all\njobs:\n  test:\n    runs-on: ubuntu-26.04\n",
            "must be read-only",
        )

    def test_rejects_oidc_write_in_pr(self) -> None:
        self.assert_rejected(
            "on:\n  pull_request:\npermissions:\n  contents: read\n  id-token: write\njobs:\n  test:\n    runs-on: ubuntu-26.04\n",
            "id-token",
        )

    def test_rejects_ubuntu_24_04_runner(self) -> None:
        self.assert_rejected(
            "on:\n  pull_request:\njobs:\n  test:\n    runs-on: ubuntu-24.04\n",
            "not approved",
        )

    def test_rejects_self_hosted_runner(self) -> None:
        self.assert_rejected(
            "on:\n  pull_request:\njobs:\n  test:\n    runs-on: self-hosted\n",
            "not approved",
        )

    def test_rejects_block_sequence_self_hosted_runner(self) -> None:
        self.assert_rejected(
            "on: [pull_request]\njobs:\n  test:\n    runs-on:\n      - self-hosted\n      - linux\n",
            "not approved",
        )

    def test_rejects_inline_sequence_self_hosted_runner(self) -> None:
        self.assert_rejected(
            "on: [pull_request]\njobs:\n  test:\n    runs-on: [self-hosted, linux]\n",
            "not approved",
        )

    def test_rejects_runner_expression(self) -> None:
        self.assert_rejected(
            "on: [pull_request]\njobs:\n  test:\n    runs-on: ${{ matrix.runner }}\n",
            "not approved",
        )

    def test_rejects_checkout_without_credential_hardening(self) -> None:
        self.assert_rejected(
            f"on:\n  pull_request:\njobs:\n  test:\n    runs-on: ubuntu-26.04\n    steps:\n      - uses: actions/checkout@{SHA}\n",
            "persist-credentials: false",
        )

    def test_rejects_mutable_checkout_ref(self) -> None:
        self.assert_rejected(
            "on:\n  pull_request:\njobs:\n  test:\n    runs-on: ubuntu-26.04\n    steps:\n      - uses: actions/checkout@v4\n",
            "full commit SHA",
        )

    def test_rejects_mutable_third_party_action_ref(self) -> None:
        self.assert_rejected(
            "on:\n  pull_request:\njobs:\n  test:\n    runs-on: ubuntu-26.04\n    steps:\n      - uses: vendor/action@main\n",
            "full commit SHA",
        )

    def test_rejects_job_level_reusable_workflow(self) -> None:
        self.assert_rejected(
            f"on: [pull_request]\njobs:\n  delegated:\n    uses: vendor/repo/.github/workflows/ci.yml@{SHA}\n",
            "reusable workflow jobs are not permitted",
        )

    def test_rejects_cross_run_artifact_download(self) -> None:
        self.assert_rejected(
            f"on:\n  pull_request:\njobs:\n  test:\n    runs-on: ubuntu-26.04\n    steps:\n      - uses: actions/download-artifact@{SHA}\n        with:\n          run-id: 123\n",
            "cross-run/external artifact",
        )

    def test_rejects_inline_cross_run_artifact_download(self) -> None:
        self.assert_rejected(
            f"on: [pull_request]\njobs:\n  test:\n    runs-on: ubuntu-26.04\n    steps:\n      - uses: actions/download-artifact@{SHA}\n        with: {{repository: attacker/repo}}\n",
            "cross-run/external artifact",
        )

    def test_rejects_real_incus_e2e_in_pr(self) -> None:
        self.assert_rejected(
            "on:\n  pull_request:\njobs:\n  test:\n    runs-on: ubuntu-26.04\n    env:\n      HACO_E2E_INCUS: 1\n",
            "HACO_E2E_INCUS",
        )

    def test_rejects_experimental_ec2_in_pr(self) -> None:
        self.assert_rejected(
            "on:\n  pull_request:\njobs:\n  test:\n    runs-on: ubuntu-26.04\n    env:\n      HACO_EXPERIMENTAL_EC2: true\n",
            "HACO_EXPERIMENTAL_EC2",
        )

    def test_rejects_secret_injection_in_pr(self) -> None:
        self.assert_rejected(
            "on:\n  pull_request:\njobs:\n  test:\n    runs-on: ubuntu-26.04\n    env:\n      TOKEN: ${{ secrets.RELEASE_TOKEN }}\n",
            "secrets are not permitted",
        )

    def test_rejects_yaml_merge_alias(self) -> None:
        self.assert_rejected(
            "on: [pull_request]\ndefaults: &dangerous\n  runs-on: self-hosted\njobs:\n  test:\n    <<: *dangerous\n",
            "YAML merge keys are not permitted",
        )

    def test_rejects_duplicate_security_key(self) -> None:
        self.assert_rejected(
            "on: [pull_request]\njobs:\n  test:\n    runs-on: ubuntu-26.04\n    runs-on: self-hosted\n",
            "duplicate YAML key",
        )


if __name__ == "__main__":
    unittest.main()

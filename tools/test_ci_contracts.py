#!/usr/bin/env python3
import json
from pathlib import Path
import unittest

from check_ci_contracts import check, ROOT


class ContractTests(unittest.TestCase):
    def setUp(self):
        self.specs = json.loads((ROOT / "tools/ci_contracts.json").read_text())

    def test_repository_contracts(self):
        for spec in self.specs.values():
            self.assertEqual(check((ROOT / ".github/workflows" / spec["file"]).read_text(), spec), [])

    def test_removed_trigger_paths_job_and_evidence_rejected(self):
        spec = self.specs["ubuntu-installer-e2e"]
        source = (ROOT / ".github/workflows" / spec["file"]).read_text()
        mutations = [
            source.replace("  pull_request:", "  push:"),
            source.replace("  pull_request:", '  pull_request:\n    paths: ["cmd/**"]'),
            source.replace("  ubuntu-user-path:", "  removed:"),
            source.replace("  ubuntu-user-path:\n", "  ubuntu-user-path:\n    if: false\n"),
            source.replace("  ubuntu-user-path:\n", "  ubuntu-user-path:\n    continue-on-error: true\n"),
            source.replace("    needs: [ubuntu-user-path]", "    needs: []"),
            source.replace("    if: always()", "    if: success()"),
            source.replace("check-latest: false", "check-latest: true"),
            source.replace("go-version: 1.27.0", "go-version: 1.27.x"),
            source.replace("[product] Exercise installed product CLI lifecycle", "removed product lifecycle"),
            source.replace('      - name: "[product] Exercise installed product CLI lifecycle"', '      - name: "[product] Exercise installed product CLI lifecycle"\n        if: false'),
            source.replace("run: python3 tools/ci_history.py --gate", "if: false\n        run: python3 tools/ci_history.py --gate"),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutation[:70]):
                self.assertTrue(check(mutation, spec))


if __name__ == "__main__":
    unittest.main()

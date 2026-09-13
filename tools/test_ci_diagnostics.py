#!/usr/bin/env python3
import json
import subprocess
import unittest
from unittest.mock import patch
from ci_diagnostics import inventory, probe


class DiagnosticsTests(unittest.TestCase):
    def test_only_state_is_retained(self):
        raw = json.dumps([{"name": "haco-ci", "status": "Running", "config": {"password": "secret"},
                           "devices": {"private": "secret"}, "description": "secret"}])
        with patch("subprocess.run", return_value=subprocess.CompletedProcess([], 0, raw, "Authorization: secret")):
            result = inventory(["incus"], ["name", "status"])
        self.assertEqual(result, {"state": "observed", "objects": [{"name": "haco-ci", "status": "Running"}]})

    def test_invalid_or_failed_inventory_is_not_empty_success(self):
        for code, raw in ((1, "[]"), (0, "not-json"), (0, "{}"), (0, '"secret"')):
            with patch("subprocess.run", return_value=subprocess.CompletedProcess([], code, raw, "secret")):
                self.assertNotEqual(inventory([], ["name"])["state"], "observed")

    def test_version_probe_omits_unexpected_output(self):
        with patch("subprocess.run", return_value=subprocess.CompletedProcess([], 1, "Authorization: Bearer secret\nextra", "private")):
            self.assertEqual(probe([]), {"exit_code": 1, "value": "omitted"})


if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3
import json
import subprocess
import sys
import unittest
from unittest.mock import patch
from ci_diagnostics import inventory, probe


class DiagnosticsTests(unittest.TestCase):
    def test_command_failures_remain_unavailable_or_timed_out(self):
        for error, state in ((OSError("private"), "unavailable"),
                             (subprocess.TimeoutExpired(["fixture"], 10), "timeout")):
            with self.subTest(error=type(error).__name__):
                with patch("subprocess.run", side_effect=error):
                    self.assertEqual(probe([]), {"state": state})
                    self.assertEqual(inventory([], ["name"]), {"state": "unavailable"})

    def test_probe_preserves_status_with_invalid_subprocess_bytes(self):
        for stdout, expected in ((b"version 1.2\n", "version 1.2"), (b"\xff", "omitted")):
            with self.subTest(stdout=stdout):
                command = [sys.executable, "-I", "-S", "-c",
                           f"import os; os.write(1, {stdout!r}); os.write(2, b'private\\xff'); raise SystemExit(7)"]
                with patch("threading.excepthook") as thread_error:
                    self.assertEqual(probe(command), {"exit_code": 7, "value": expected})
                thread_error.assert_not_called()

    def test_inventory_ignores_invalid_stderr_but_rejects_invalid_json(self):
        for stdout, status, expected in (
            (b'[{"name":"haco-ci","status":"Running"}]', 0,
             {"state": "observed", "objects": [{"name": "haco-ci", "status": "Running"}]}),
            (b"\xff", 0, {"state": "unavailable"}),
            (b'[{"name":"haco-\xff","status":"Running"}]', 0, {"state": "unavailable"}),
            (b'[{"name":"haco-ci","description":"private\xff"}]', 0, {"state": "unavailable"}),
            (b"[]", 7, {"state": "unavailable", "exit_code": 7}),
        ):
            with self.subTest(stdout=stdout):
                command = [sys.executable, "-I", "-S", "-c",
                           f"import os; os.write(1, {stdout!r}); os.write(2, b'private\\xff'); raise SystemExit({status})"]
                with patch("threading.excepthook") as thread_error:
                    self.assertEqual(inventory(command, ["name", "status"]), expected)
                thread_error.assert_not_called()

    def test_only_state_is_retained(self):
        raw = json.dumps([{"name": "haco-ci", "status": "Running", "config": {"password": "secret"},
                           "devices": {"private": "secret"}, "description": "secret"}])
        with patch("subprocess.run", return_value=subprocess.CompletedProcess([], 0, raw.encode("utf-8"), b"Authorization: secret")):
            result = inventory(["incus"], ["name", "status"])
        self.assertEqual(result, {"state": "observed", "objects": [{"name": "haco-ci", "status": "Running"}]})

    def test_invalid_or_failed_inventory_is_not_empty_success(self):
        for code, raw in ((1, "[]"), (-9, "[]"), (0, "not-json"), (0, "{}"), (0, '"secret"')):
            with patch("subprocess.run", return_value=subprocess.CompletedProcess([], code, raw.encode("utf-8"), b"secret")):
                self.assertNotEqual(inventory([], ["name"])["state"], "observed")

    def test_version_probe_omits_unexpected_output(self):
        with patch("subprocess.run", return_value=subprocess.CompletedProcess([], 1, b"Authorization: Bearer secret\nextra", b"private")):
            self.assertEqual(probe([]), {"exit_code": 1, "value": "omitted"})


if __name__ == "__main__":
    unittest.main()

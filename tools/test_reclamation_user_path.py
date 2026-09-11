#!/usr/bin/env python3
"""Refusal tests for the native gate's resume decision, without WSL mutation."""
import importlib.util
from pathlib import Path
import json
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("reclaim_gate", Path(__file__).with_name("windows-reclamation-user-path-e2e.py"))
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)
OP = "{11111111-1111-4111-8111-111111111111}"


class ReclamationUserPathTests(unittest.TestCase):
    def test_process_identity_must_be_observed(self):
        helper = r"C:\fixture\haco-wsl.exe"
        self.assertFalse(gate.helper_is_running([], helper))
        self.assertTrue(gate.helper_is_running({"ExecutablePath": helper.upper()}, helper))
        self.assertFalse(gate.helper_is_running([{"ExecutablePath": r"C:\other\haco-wsl.exe"}], helper))
        for rows in (None, {}, [None], [{"ExecutablePath": None}], [{"ExecutablePath": ""}]):
            with self.assertRaises(RuntimeError):
                gate.helper_is_running(rows, helper)

    def test_pending_and_failed_never_authorize_resume(self):
        self.assertFalse(gate.require_complete({"operation": OP, "state": "pending"}, OP))
        for result in ({"operation": OP, "state": "failed"},
                       {"operation": OP, "state": "complete"},
                       {"operation": "foreign", "state": "complete"},
                       {"operation": OP, "state": "unknown"}):
            with self.assertRaises(RuntimeError):
                gate.require_complete(result, OP)

    def test_failed_status_stops_observation_without_reentry(self):
        with patch.object(gate, "read_json", return_value={"operation": OP, "state": "failed"}) as read, patch.dict(gate.os.environ, {"SystemRoot": r"C:\Windows"}):
            with self.assertRaises(RuntimeError):
                gate.wait_for_worker(Path("fixture-haco-wsl.exe"), OP, OP)
            self.assertEqual(read.call_count, 1)
            self.assertEqual(read.call_args.args[0][1:], ["_status", OP, OP])

    def test_failure_summary_does_not_emit_child_secrets(self):
        result = {"state": "failed", "linux_started": True, "linux": {"failure": "secret-token", "incus_btrfs_loop": {"status": "failed"}}, "observation": {"StopAttempted": False, "Resumed": "secret-token", "Failure": "secret-token", "NativeError": "secret-token"}, "credentials": "secret-token"}
        summary = gate.failure_summary(result)
        self.assertNotIn("secret-token", json.dumps(summary))
        self.assertEqual(summary["linux_pool"], "failed")
        self.assertIsNone(summary["windows_resumed"])
        self.assertEqual(gate.failure_summary(None), {"worker_result": "unrecognized"})

    def test_all_stages_are_required(self):
        result = {"operation": OP, "state": "complete", "linux_started": True,
                  "linux": {"incus_btrfs_loop": {"status": "complete"}, "wsl_ext4": {"status": "complete"}},
                  "observation": {"StopAttempted": True, "StopRequested": True, "ResumeAttempted": True, "Resumed": True,
                    "Compaction": {"Attempted": True, "Completed": True, "Virtual": {"Capacity": 1024}}}}
        self.assertTrue(gate.require_complete(result, OP))
        result["observation"]["Resumed"] = False
        with self.assertRaises(RuntimeError):
            gate.require_complete(result, OP)


if __name__ == "__main__":
    unittest.main()

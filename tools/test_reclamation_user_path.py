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
    def test_status_waits_for_completion_despite_echo_and_prompt_repaint(self):
        output = ""
        chunks = [
            "root@haco-host:~# \n",
            'haco reclaim --status; printf \'\\nHACO_ABSENT_STATUS_EXIT:%s\\n\' "$?"\n',
            "root@haco-host:~# \n",
            "No saved reclamation result. No operation was started by this status check.\n",
            "HACO_ABSENT_STATUS_EX",
            "IT:",
        ]
        for chunk in chunks:
            output += chunk
            self.assertFalse(gate.absent_status_completed(output))
        self.assertTrue(gate.absent_status_completed(output + "0\n"))

    def test_status_completion_requires_success_and_response(self):
        message = "No saved reclamation result. No operation was started by this status check.\n"
        for output in ("HACO_ABSENT_STATUS_EXIT:0\n",
                       message + "HACO_ABSENT_STATUS_EXIT:1\n",
                       "HACO_ABSENT_STATUS_EXIT:0\n" + message):
            with self.assertRaises(RuntimeError):
                gate.absent_status_completed(output)

    def test_absent_history_refuses_existing_or_partial_evidence(self):
        gate.require_absent_history({"operation": "", "state": "none"})
        for result in (None, {}, {"state": "none"},
                       {"operation": OP, "state": "pending"},
                       {"operation": OP, "state": "failed"},
                       {"operation": "", "state": "none", "linux_started": True}):
            with self.assertRaises(RuntimeError):
                gate.require_absent_history(result)

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

    def test_worker_completion_between_status_and_process_observation(self):
        complete = {"operation": OP, "state": "complete", "linux_started": True,
                    "linux": {"incus_btrfs_loop": {"status": "complete"}, "wsl_ext4": {"status": "complete"}},
                    "observation": {"StopAttempted": True, "StopRequested": True, "ResumeAttempted": True, "Resumed": True,
                                    "Compaction": {"Attempted": True, "Completed": True, "Virtual": {"Capacity": 1024}}}}
        # The worker publishes completion and exits after the first status read.
        observations = [{"operation": OP, "state": "pending"}, [], complete]
        with patch.object(gate, "read_json", side_effect=observations) as read, patch.dict(gate.os.environ, {"SystemRoot": r"C:\Windows"}), patch("builtins.print"):
            gate.wait_for_worker(Path("fixture-haco-wsl.exe"), OP, OP)
            self.assertEqual(read.call_count, 3)
            self.assertEqual(read.call_args.args[0][1:], ["_status", OP, OP])

    def test_absent_worker_requires_fresh_complete_result(self):
        for final in ({"operation": OP, "state": "pending"},
                      {"operation": OP, "state": "failed"},
                      {"operation": OP, "state": "complete"},
                      {"operation": "foreign", "state": "complete"}):
            with self.subTest(final=final), patch.object(gate, "read_json", side_effect=[{"operation": OP, "state": "pending"}, [], final]) as read, patch.dict(gate.os.environ, {"SystemRoot": r"C:\Windows"}), patch("builtins.print"):
                with self.assertRaises(RuntimeError):
                    gate.wait_for_worker(Path("fixture-haco-wsl.exe"), OP, OP)
                self.assertEqual(read.call_count, 3)

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

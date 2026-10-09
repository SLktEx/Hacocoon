#!/usr/bin/env python3
"""Incus 7.0 API/CLI observation and fail-closed capability-probe regressions."""
import contextlib
import errno
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import incus_vm_probe as probe

def envelope(metadata):
    return json.dumps({"type": "sync", "status": "Success", "status_code": 200,
                       "operation": "", "error_code": 0, "error": "", "metadata": metadata})


NAME = "hci-123-1-vm"
MISSING = ('Error: Instance type "virtual-machine" is not supported on this server: '
           'KVM support is missing (no /dev/kvm)\n')


class IncusFixture:
    # Shapes are from Incus v7.0.1 shared/api/{instance,server,operation}.go,
    # not human-readable CLI tables. No management authority enters the guest.
    def __init__(self, failure="", stderr="", code=1):
        self.failure, self.stderr, self.code = failure, stderr, code
        self.row = None
        self.calls = []
        self.operation_reads = 0
        self.environment = {"server_version": "7.0.1", "kernel": "Linux", "kernel_version": "7.0.0-azure",
                            "kernel_architecture": "x86_64", "os_name": "Ubuntu", "os_version": "26.04",
                            "driver": "lxc | qemu", "server_certificate": "secret"}

    def __call__(self, args, timeout=30):
        self.calls.append(args)
        value = ""
        if args[0] == "query":
            if args[-1].startswith("/1.0/instances?"):
                if self.failure == "inventory":
                    return subprocess.CompletedProcess(args, 1, "[]", "private")
                value = json.dumps([self.row] if self.row else [])
            elif args[-1].startswith("/1.0/operations?"):
                self.operation_reads += 1
                if self.failure == "operations":
                    value = '{"running": [{"status_code": 103}]}'
                elif self.failure == "operations_invalid":
                    value = '{"running": [{"status_code": "200"}]}'
                else:
                    value = "{}"
            elif args[-1] == "/1.0":
                value = json.dumps({"environment": self.environment})
            else:
                raise AssertionError(args)
            assert args[1] == "--raw"
            value = envelope(json.loads(value))
        elif args[0] == "profile":
            value = "default\n"
        elif args[0] == "launch":
            if self.failure == "launch":
                return subprocess.CompletedProcess(args, self.code, "", self.stderr)
            self.row = {"name": NAME, "type": "virtual-machine", "status": "Running",
                        "config": {probe.OWNER: args[-1].split("=", 1)[1], "volatile.base_image": "a" * 64}}
            if self.failure == "partial_launch":
                return subprocess.CompletedProcess(args, 1, "", "storage failure")
            if self.failure == "launch_timeout":
                raise probe.ProbeError("command_timeout")
            if self.failure == "owner":
                self.row["config"][probe.OWNER] = "someone-else"
        elif args[0] == "exec":
            value = "vm-systemd-ready\n"
            if self.failure == "guest":
                return subprocess.CompletedProcess(args, 1, value, "unavailable")
        elif args[0] == "stop":
            if self.failure != "stop":
                self.row["status"] = "Stopped"
        elif args[0] == "start":
            self.row["status"] = "Running"
        elif args[0] == "delete":
            if self.failure == "delete":
                return subprocess.CompletedProcess(args, 1, "", "private")
            if self.failure != "remains":
                self.row = None
        else:
            raise AssertionError(args)
        return subprocess.CompletedProcess(args, 0, value, "")


class ProbeTests(unittest.TestCase):
    def run_case(self, fixture, kvm=None, tick=200):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "result.json"
            fixture.output = output
            with patch.object(probe, "command", side_effect=fixture), \
                    patch.object(probe, "kvm_observation", return_value=kvm or {"state": "present"}), \
                    patch.object(probe, "cpu_observation", return_value={"vmx": True, "svm": False}), \
                    patch.object(probe.time, "sleep"), \
                    patch.object(probe.time, "monotonic", side_effect=range(0, 10000, tick)), \
                    contextlib.redirect_stdout(io.StringIO()) as logs:
                code = probe.run_probe(NAME, output)
            receipt = json.loads(output.read_text())
            self.assertNotIn("private", logs.getvalue())
            self.assertNotIn("secret", logs.getvalue())
            return code, receipt

    def test_success_is_vm_exec_systemd_stop_start_and_verified_cleanup(self):
        fixture = IncusFixture()
        code, result = self.run_case(fixture)
        self.assertEqual(code, 0)
        self.assertEqual(result["status"], "supported")
        self.assertEqual(result["cleanup"], "verified_absent")
        self.assertEqual(result["image_fingerprint"], "a" * 64)
        launch = next(args for args in fixture.calls if args[0] == "launch")
        self.assertIn("--vm", launch)
        self.assertIn("--no-profiles", launch)
        self.assertNotIn("--network", launch)
        self.assertEqual([args[0] for args in fixture.calls if args[0] != "query"],
                         ["profile", "launch", "exec", "stop", "start", "exec", "delete"])
        # Reservation exists before launch, and no rerun can overwrite it.
        self.assertTrue(result["launch_attempted"])

    def test_client_kvm_permission_is_not_daemon_support(self):
        code, result = self.run_case(IncusFixture(), {"state": "present", "client_errno": errno.EACCES})
        self.assertEqual((code, result["status"]), (0, "supported"))

    def test_missing_kvm_requires_independent_observation_and_exact_refusal(self):
        for stderr in (MISSING, MISSING.replace("Error: ", "Error: Failed instance creation: ")):
            fixture = IncusFixture("launch", stderr)
            code, result = self.run_case(fixture, {"state": "missing", "errno": errno.ENOENT})
            self.assertEqual((code, result["status"]), (0, "unsupported"))
            self.assertEqual(result["reason"], "missing_kvm_device")
            self.assertEqual(result["cleanup"], "verified_absent")
            self.assertIn("launch", [args[0] for args in fixture.calls])
            self.assertNotIn("exec", [args[0] for args in fixture.calls])
        for state in ("present", "observation_error"):
            code, result = self.run_case(IncusFixture("launch", MISSING), {"state": state})
            self.assertEqual((code, result["status"]), (1, "error"))

    def test_fixture_network_unknown_and_signaled_failures_are_not_unsupported(self):
        for message in ("image download failed", "network timeout", "missing QEMU", "no space left",
                        "KVM failed", MISSING + "a network failure", "private credential\n" + MISSING):
            with self.subTest(message=message):
                code, result = self.run_case(IncusFixture("launch", message), {"state": "missing"})
                self.assertEqual((code, result["status"]), (1, "error"))
        code, result = self.run_case(IncusFixture("launch", MISSING, -15), {"state": "missing"})
        self.assertEqual((code, result["status"]), (1, "error"))

    def test_partial_creation_and_timeout_still_clean_exact_owner(self):
        for failure in ("partial_launch", "launch_timeout"):
            fixture = IncusFixture(failure)
            code, result = self.run_case(fixture)
            self.assertEqual((code, result["status"], result["cleanup"]), (1, "error", "verified_absent"))
            self.assertIsNone(fixture.row)

    def test_timeout_waits_for_late_created_owned_vm_before_deletion(self):
        fixture = IncusFixture()
        pending = None
        observations = 0

        def delayed(args, timeout=30):
            nonlocal pending, observations
            if args[0] == "launch":
                saved = json.loads(delayed.output.read_text())
                self.assertTrue(saved["launch_attempted"])
                self.assertEqual(saved["owner"], args[-1].split("=", 1)[1])
                fixture(args, timeout)
                pending, fixture.row = fixture.row, None
                raise probe.ProbeError("command_timeout")
            if args[0] == "query" and args[-1].startswith("/1.0/operations?"):
                fixture.calls.append(args)
                observations += 1
                if observations == 1:
                    return subprocess.CompletedProcess(args, 0, envelope({"running": [{"status_code": 103}]}), "")
                if observations == 2:
                    fixture.row = pending
                    return subprocess.CompletedProcess(args, 0, envelope({"success": [{"status_code": 200}]}), "")
            return fixture(args, timeout)

        code, result = self.run_case(delayed, tick=1)
        self.assertEqual((code, result["status"], result["cleanup"]), (1, "error", "verified_absent"))
        self.assertIsNone(fixture.row)
        self.assertGreaterEqual(observations, 3)
        self.assertEqual(sum(args[0] == "delete" for args in fixture.calls), 1)

    def test_unverified_cleanup_always_fails(self):
        for failure in ("delete", "remains", "operations", "operations_invalid", "owner"):
            fixture = IncusFixture(failure)
            code, result = self.run_case(fixture)
            self.assertEqual((code, result["status"]), (1, "error"), failure)
            self.assertNotEqual(result["cleanup"], "verified_absent")
            if failure in ("owner", "operations", "operations_invalid"):
                self.assertNotIn("delete", [args[0] for args in fixture.calls])

    def test_failed_inventory_never_launches_or_deletes(self):
        fixture = IncusFixture("inventory")
        code, result = self.run_case(fixture)
        self.assertEqual(code, 1)
        self.assertFalse(result["launch_attempted"])
        self.assertEqual([args[0] for args in fixture.calls], ["query"])

    def test_missing_or_malformed_server_identity_is_inconclusive(self):
        for field in ("server_version", "kernel", "kernel_version", "kernel_architecture", "os_name", "os_version"):
            for invalid in (None, [], 123, "", "private\nextra"):
                fixture = IncusFixture("launch", MISSING)
                fixture.environment[field] = invalid
                code, result = self.run_case(fixture, {"state": "missing"})
                self.assertEqual((code, result["status"], result["reason"]), (1, "error", "invalid_server_identity"))
                self.assertFalse(result["launch_attempted"])
        fixture = IncusFixture()
        fixture.environment["driver"] = {"unexpected": "optional"}
        code, result = self.run_case(fixture)
        self.assertEqual((code, result["status"]), (0, "supported"))
        self.assertNotIn("driver", result["server"])

    def test_existing_name_is_not_adopted(self):
        fixture = IncusFixture()
        fixture.row = {"name": NAME, "type": "virtual-machine", "config": {probe.OWNER: "other"}}
        code, result = self.run_case(fixture)
        self.assertEqual((code, result["reason"]), (1, "name_already_exists"))
        self.assertNotIn("delete", [args[0] for args in fixture.calls])

    def test_guest_and_lifecycle_failure_do_not_claim_support(self):
        for failure in ("guest", "stop"):
            code, result = self.run_case(IncusFixture(failure))
            self.assertEqual((code, result["status"]), (1, "error"))
            self.assertEqual(result["cleanup"], "verified_absent")

    def test_receipt_paths_and_names_cannot_select_arbitrary_resources(self):
        for name in ("--force", "remote:vm", "../hci-1-1-vm", NAME + "\n", "hci-1-1-container"):
            with self.subTest(name=name), patch.object(probe, "command") as command:
                with self.assertRaises(probe.ProbeError):
                    probe.cleanup({"name": name, "owner": "a" * 32, "launch_attempted": True})
                command.assert_not_called()

    def test_receipt_cannot_be_overwritten(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "result.json"
            output.write_text("prior receipt")
            with self.assertRaises(FileExistsError):
                probe.run_probe(NAME, output)
            self.assertEqual(output.read_text(), "prior receipt")

    def test_failed_query_with_plausible_json_is_not_absence(self):
        for code, raw in ((1, "[]"), (0, "{}"), (0, '[{"name":"x"},{"name":"x"}]'),
                          (0, '[null]'), (0, "bad json")):
            with patch.object(probe, "command", return_value=subprocess.CompletedProcess([], code, envelope(json.loads(raw)) if raw != "bad json" else raw, "")):
                with self.assertRaises(probe.ProbeError):
                    probe.instance(NAME)

    def test_empty_operation_metadata_and_raw_error_envelopes(self):
        with patch.object(probe, "command", return_value=subprocess.CompletedProcess([], 0, envelope({}), "")):
            probe.wait_for_operations()
        for change in ({"type": "error", "error_code": 500, "error": "private"},
                       {"type": "async"}, {"status_code": "200"}, {"error_code": "0"},
                       {"status_code": 403}, {"metadata": None}):
            response = json.loads(envelope({}))
            response.update(change)
            with patch.object(probe, "command", return_value=subprocess.CompletedProcess([], 0, json.dumps(response), "")):
                with self.assertRaises(probe.ProbeError):
                    probe.wait_for_operations()
        for raw in ("", "{}", "null", "[]"):
            with patch.object(probe, "command", return_value=subprocess.CompletedProcess([], 0, raw, "")):
                with self.assertRaises(probe.ProbeError):
                    probe.wait_for_operations()

    def test_commands_cannot_inherit_launch_yaml_from_stdin(self):
        with patch.object(probe.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, "", "")) as run:
            probe.command(["launch", "images:ubuntu/26.04", NAME, "--vm"])
            self.assertEqual(run.call_args.kwargs["stdin"], subprocess.DEVNULL)

    def test_receipt_save_does_not_follow_predictable_temporary_symlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            output, unrelated = directory / "result.json", directory / "unrelated"
            unrelated.write_text("unchanged")
            output.with_name(output.name + ".tmp").symlink_to(unrelated)
            probe.save(output, {"owner": "a" * 32})
            self.assertEqual(unrelated.read_text(), "unchanged")
            self.assertFalse(output.is_symlink())
            self.assertEqual(output.stat().st_mode & 0o777, 0o600)

    def test_command_failures_keep_raw_output_private(self):
        for error in (OSError("private"), subprocess.TimeoutExpired(["incus"], 1, output="secret")):
            with patch.object(probe.subprocess, "run", side_effect=error):
                with self.assertRaises(probe.ProbeError) as raised:
                    probe.command(["query", "/1.0"])
                self.assertNotIn("private", str(raised.exception))
                self.assertNotIn("secret", str(raised.exception))

    def test_kvm_missing_permission_and_wrong_device_are_distinct(self):
        for number, state in ((errno.ENOENT, "missing"), (errno.EACCES, "observation_error")):
            with patch.object(probe.os, "stat", side_effect=OSError(number, "private")):
                self.assertEqual(probe.kvm_observation()["state"], state)
        with tempfile.NamedTemporaryFile() as temporary:
            info = Path(temporary.name).stat()
            with patch.object(probe.os, "stat", return_value=info), patch.object(probe.os, "open") as opened:
                self.assertFalse(probe.kvm_observation()["character_device"])
                opened.assert_not_called()


if __name__ == "__main__":
    unittest.main()

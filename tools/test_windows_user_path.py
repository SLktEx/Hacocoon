#!/usr/bin/env python3
"""Component regressions for read-only Windows user-path assertions."""
import importlib.util
import json
from pathlib import Path
import sys
import unittest
from unittest.mock import patch
from unittest.mock import Mock

spec = importlib.util.spec_from_file_location("windows_user_path", Path(__file__).with_name("windows-installer-user-path-e2e.py"))
gate = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = gate
spec.loader.exec_module(gate)


class EnvironmentBoundaryTest(unittest.TestCase):
    def test_retention_flags_are_not_allowed_in_the_product_environment(self):
        for name in ("HACO_E2E_RECLAIM_RETENTION", "HACO_RECLAIM_RETENTION_MANIFEST"):
            with patch.dict(gate.os.environ, {name: "fixture"}, clear=True):
                with self.assertRaisesRegex(RuntimeError, "refuses Hacocoon environment overrides"):
                    gate.inherited_child_environment()
        with patch.dict(gate.os.environ, {"RUNNER_TEMP": "fixture-directory"}, clear=True):
            self.assertEqual(gate.inherited_child_environment(), {"RUNNER_TEMP": "fixture-directory"})


class LanguageAssertionTest(unittest.TestCase):
    def test_only_one_exact_normalized_observation_passes(self):
        for language in ('en', 'ja'):
            gate.assert_host_language(f'HACO_HOST_UI:{language}\n', language)
            gate.assert_host_language(f'HACO_HOST_UI:{language}\r\n', language)
        for output in ('', 'HACO_HOST_UI:ja_JP\n', 'HACO_HOST_UI:en\n',
                       "root@haco-host:~# printf 'HACO_HOST_UI:ja'\n",
                       'HACO_HOST_UI:ja\nHACO_HOST_UI:ja\n'):
            with self.subTest(output=output), self.assertRaises(RuntimeError):
                gate.assert_host_language(output, 'ja')


class DoctorAssertionTest(unittest.TestCase):
    build = {"checkpoint": "v0.26", "version": "0.26.1-candidate", "commit": "candidate", "build_date": "2026-09-06T00:00:00Z"}

    def report(self):
        return {"protocol_version": 1, "controller": dict(self.build), "checks": [
            {"name": name, "status": "ok"} for name in
            ["runtime", "storage", "storage_mount", "trusted_host", "trusted_network", "trusted_connectivity"]
        ]}

    def test_accepts_complete_report(self):
        gate.assert_doctor_report(json.dumps(self.report()), self.build)

    def test_rejects_failure_skips_missing_and_duplicates(self):
        for mutation in ("failed", "skipped", "pending", "missing", "duplicate", "protocol", "identity"):
            with self.subTest(mutation=mutation):
                report = self.report()
                if mutation in ("failed", "skipped", "pending"):
                    report["checks"][4]["status"] = mutation
                elif mutation == "missing":
                    report["checks"].pop()
                elif mutation == "duplicate":
                    report["checks"][1]["name"] = report["checks"][0]["name"]
                elif mutation == "protocol":
                    report["protocol_version"] = 0
                else:
                    report["controller"]["commit"] = ""
                with self.assertRaises(RuntimeError):
                    gate.assert_doctor_report(json.dumps(report), self.build)

    def test_rejects_unstamped_or_mismatched_controller(self):
        for field, value in (("version", "dev"), ("build_date", "unknown"), ("commit", "stale"), ("checkpoint", "stale")):
            with self.subTest(field=field):
                report = self.report()
                report["controller"][field] = value
                with self.assertRaises(RuntimeError):
                    gate.assert_doctor_report(json.dumps(report), self.build)

    def test_rejects_unstamped_client_even_when_controller_matches(self):
        for field in self.build:
            with self.subTest(field=field):
                report = self.report()
                report["controller"][field] = "unknown"
                with self.assertRaises(RuntimeError):
                    gate.assert_doctor_report(json.dumps(report), report["controller"])

    def test_rejects_non_json(self):
        with self.assertRaises(json.JSONDecodeError):
            gate.assert_doctor_report("not a report", self.build)


class ProxyListenerAssertionTest(unittest.TestCase):
    listener = 'LISTEN 0 4096 169.254.254.1:18080 0.0.0.0:* users:(("haco-controller",pid=125,fd=3))'

    def test_fixed_listener_belongs_to_same_controller(self):
        gate.assert_proxy_listener("125", self.listener)

    def test_missing_wildcard_duplicate_and_foreign_listeners_fail(self):
        for pid, lines in (("0", self.listener), ("", self.listener), ("125", ""),
                           ("125", self.listener.replace("169.254.254.1", "0.0.0.0")),
                           ("125", self.listener + "\n" + self.listener),
                           ("126", self.listener), ("125", self.listener.replace("pid=125,", "pid=1250,"))):
            with self.subTest(pid=pid, lines=lines):
                with self.assertRaises(RuntimeError):
                    gate.assert_proxy_listener(pid, lines)


class InstallerFailureTest(unittest.TestCase):
    def test_reported_failure_ends_owned_terminal_without_a_timeout_or_retry(self):
        terminal = Mock()
        terminal.proc.isalive.return_value = True
        def run(*, responders, on_output):
            on_output("C:\\package> ", terminal)
            on_output("C:\\package> install-windows.bat\nHacocoon installation failed with exit code 1. \n", terminal)
            self.fail("reported failure was ignored")
        terminal.run.side_effect = run
        with patch.object(gate, "TerminalProcess", return_value=terminal):
            with self.assertRaisesRegex(RuntimeError, "BAT reported installation failure"):
                gate.run_bat(Path("fixture"))
        terminal.write.assert_called_once_with("install-windows.bat\r\n")
        terminal.proc.terminate.assert_called_once_with(force=True)


class TerminalNormalizationTest(unittest.TestCase):
    def test_conpty_carriage_return_does_not_hide_a_real_output_line(self):
        # Reduced from a real pywinpty 3.0.2 -> ordinary WSL -> haco-host
        # session: ConPTY emits CRLF followed by another CR before OSC 3008.
        raw = (
            "root@haco-host:~# cat ~/.hacocoon-installer-acceptance\r\n\r"
            "\x1b[?2004l\x1b]3008;type=command;cwd=/root\x1b\\"
            "kept-through-restart-and-rerun\r\n"
            "\x1b]3008;exit=success\x07"
        )
        output = gate.normalize_terminal(raw)
        gate.require_output(output, r"^kept-through-restart-and-rerun\s*$", phase="haco-host data")
        self.assertNotIn("\r", output)
        self.assertNotIn("3008;", output)

    def test_echoed_command_cannot_satisfy_the_retained_data_assertion(self):
        raw = (
            "root@haco-host:~# printf '%s\\n' kept-through-restart-and-rerun"
            " > ~/.hacocoon-installer-acceptance\r\n\r"
        )
        with self.assertRaises(RuntimeError):
            gate.require_output(gate.normalize_terminal(raw),
                                r"^kept-through-restart-and-rerun\s*$", phase="haco-host data")


if __name__ == "__main__":
    unittest.main()

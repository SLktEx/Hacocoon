#!/usr/bin/env python3
"""Component regressions for read-only Windows user-path assertions."""
import importlib.util
import json
from pathlib import Path
import sys
import unittest

spec = importlib.util.spec_from_file_location("windows_user_path", Path(__file__).with_name("windows-installer-user-path-e2e.py"))
gate = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = gate
spec.loader.exec_module(gate)


class DoctorAssertionTest(unittest.TestCase):
    build = {"checkpoint": "v0.26", "version": "0.26.1-candidate", "commit": "candidate", "build_date": "2026-09-06T00:00:00Z"}

    def report(self):
        return {"protocol_version": 1, "controller": dict(self.build), "checks": [
            {"name": name, "status": "ok"} for name in
            ["runtime", "storage", "trusted_host", "trusted_network", "trusted_connectivity"]
        ]}

    def test_accepts_complete_report(self):
        gate.assert_doctor_report(json.dumps(self.report()), self.build)

    def test_rejects_failure_skips_missing_and_duplicates(self):
        for mutation in ("failed", "skipped", "missing", "duplicate", "protocol", "identity"):
            with self.subTest(mutation=mutation):
                report = self.report()
                if mutation in ("failed", "skipped"):
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

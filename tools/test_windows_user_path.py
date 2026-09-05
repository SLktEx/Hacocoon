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


if __name__ == "__main__":
    unittest.main()

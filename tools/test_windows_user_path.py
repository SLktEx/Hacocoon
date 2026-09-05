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
    def report(self):
        return {"protocol_version": 1, "controller": {"commit": "candidate"}, "checks": [
            {"name": name, "status": "ok"} for name in
            ["runtime", "storage", "trusted_host", "trusted_network", "trusted_connectivity"]
        ]}

    def test_accepts_complete_report(self):
        gate.assert_doctor_report(json.dumps(self.report()))

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
                    gate.assert_doctor_report(json.dumps(report))

    def test_rejects_non_json(self):
        with self.assertRaises(json.JSONDecodeError):
            gate.assert_doctor_report("not a report")


if __name__ == "__main__":
    unittest.main()

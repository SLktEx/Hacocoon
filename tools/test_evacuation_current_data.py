"""Current catalog data must remain visible without becoming cleanup authority."""
import copy
import json
from pathlib import Path
import re
import unittest

import evacuation_inventory as inventory
from evacuation_associations import compare_associations


class CurrentDataTests(unittest.TestCase):
    def fixture(self):
        ref = {"id": "env-data:" + "a" * 32, "owner": "b" * 32}
        generation = {"name": "build", "kind": "build-cache", "compatibility": "c" * 64,
                      "epoch": "d" * 32, "number": 0}
        attachment = {"key": "build", "target": "/opt/build cache", "resource": ref,
                      "origin": generation, "credential": "never-copy"}
        return {"version": 16, "environments": {"dev": {
            "name": "dev", "workspace": {"id": "work", "path": "managed:app"},
            "runtime_ref": "dev", "dns_mode": "backend", "attachments": [attachment]}},
            "workspace_leases": {"work": {"instance_id": "e" * 32, "state": "cleanup-required",
                                           "runtime_absent": True, "attachments": [attachment]}},
            "persistent_resources": {ref["id"]: {**ref, "kind": "build-cache", "native_ref": "pool/data",
                "state": "creating", "environment_instance": "e" * 32, "import_pending": True,
                "copy_completed": False, "source_only": False, "credential": "never-copy"}},
            "resource_generations": {"build": generation}}

    def test_current_schema_stays_aligned_with_canonical_store(self):
        source = Path(__file__).resolve().parents[1] / "internal/state/environment_jsonstore.go"
        version = int(re.search(r"const environmentStateVersion = (\d+)", source.read_text()).group(1))
        self.assertTrue(inventory.catalog_references({"version": version})["projection_complete"])

    def test_current_data_and_recovery_flags_survive_read_only_projection(self):
        data = self.fixture()
        original = copy.deepcopy(data)
        report = inventory.catalog_references(data)
        self.assertTrue(report["projection_complete"])
        self.assertFalse(report["authority"])
        self.assertFalse(report["state_validated"])
        rows = {r["section"]: r for r in report["records"]}
        self.assertEqual(rows["environments"]["attachments"][0]["target"], "/opt/build cache")
        self.assertTrue(rows["persistent_resources"]["import_pending"])
        self.assertTrue(rows["workspace_leases"]["runtime_absent"])
        self.assertEqual(rows["resource_generations"]["generation"]["number"], 0)
        self.assertNotIn("never-copy", json.dumps(report))
        self.assertEqual(data, original)
        associations = compare_associations({"projects": [], "native_queries_complete": True}, report)
        self.assertEqual(len(associations["catalog_links"]), 2)
        self.assertTrue(all(x["status"] == "catalog-reference-observed" for x in associations["catalog_links"]))
        self.assertTrue(any(x["status"] == "not-observed" for x in associations["rows"]))
        self.assertFalse(associations["authority"])

    def test_generation_history_and_missing_producer_remain_distinct(self):
        data = self.fixture()
        source = {"id": "generation:" + "f" * 32, "owner": "a" * 32}
        data["resource_generations"]["build"].update(number=1, current=source)
        child = next(iter(data["persistent_resources"].values()))
        child.update(copy_source=source, producer={"id": "env-data:removed", "owner": "b" * 32},
                     publication_origin=data["resource_generations"]["build"])
        report = inventory.catalog_references(data)
        self.assertTrue(report["projection_complete"])
        links = compare_associations({"projects": []}, report)["catalog_links"]
        self.assertTrue(any(x["relation"] == "producer" and x["status"] == "catalog-reference-not-observed" for x in links))
        self.assertTrue(any(x["relation"] == "generation" for x in links))

    def test_malformed_data_does_not_hide_other_resources_or_echo_values(self):
        for invalid in (None, [None], [{"key": "never-copy"}]):
            data = self.fixture()
            data["environments"]["dev"]["attachments"] = invalid
            report = inventory.catalog_references(data)
            self.assertFalse(report["projection_complete"])
            self.assertTrue(any(r["section"] == "persistent_resources" for r in report["records"]))
            self.assertNotIn("never-copy", json.dumps(report))
        for bad in (True, -1, 2**64, "never-copy"):
            data = self.fixture()
            data["resource_generations"]["build"]["number"] = bad
            self.assertFalse(inventory.catalog_references(data)["projection_complete"])

    def test_reference_owner_mismatch_and_budget_are_observations(self):
        report = inventory.catalog_references(self.fixture())
        resource = next(r for r in report["records"] if r["section"] == "persistent_resources")
        resource["owner"] = "f" * 32
        links = compare_associations({"projects": []}, report)["catalog_links"]
        self.assertTrue(all(x["status"] == "catalog-owner-mismatch" for x in links))


if __name__ == "__main__":
    unittest.main()

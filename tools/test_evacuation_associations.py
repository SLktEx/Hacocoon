import copy
import unittest
from unittest.mock import patch
import evacuation_associations as subject

OWNER = "a" * 32

class AssociationTests(unittest.TestCase):
    def native(self):
        return {"native_queries_complete": True, "projects": [{"name": "hacocoon", "instances": [{"name": "saved-root", "owner_marker": OWNER}], "volumes": [{"name": "work", "pool": "pool", "type": "custom", "owner_marker": OWNER}]}]}

    def catalog(self, ref="pool/work"):
        return {"projection_complete": True, "records": [{"section": "persistent_resources", "key": "oci:work", "native_ref": ref, "owner": OWNER}]}

    def test_matching_reference_is_not_authority_and_inputs_are_unchanged(self):
        native, catalog = self.native(), self.catalog()
        before = copy.deepcopy((native, catalog))
        result = subject.compare_associations(native, catalog)
        self.assertEqual(result["rows"][0]["status"], "reference-and-marker-observed")
        self.assertFalse(result["authority"])
        self.assertTrue(result["review_required"])
        self.assertEqual((native, catalog), before)

    def test_ambiguous_projects_are_not_resolved_by_matching_owner(self):
        native = self.native()
        other = copy.deepcopy(native["projects"][0]); other["name"] = "other"
        other["volumes"][0]["owner_marker"] = "b" * 32
        native["projects"].append(other)
        row = subject.compare_associations(native, self.catalog())["rows"][0]
        self.assertEqual(row["status"], "ambiguous")
        self.assertEqual(len(row["candidates"]), 2)

    def test_missing_mismatched_and_unavailable_owners_remain_distinct(self):
        for marker, expected in [(None, "native-owner-unavailable"), ("b" * 32, "owner-mismatch")]:
            native = self.native(); native["projects"][0]["volumes"][0]["owner_marker"] = marker
            self.assertEqual(subject.compare_associations(native, self.catalog())["rows"][0]["status"], expected)
        self.assertEqual(subject.compare_associations(self.native(), self.catalog("pool/missing"))["rows"][0]["status"], "not-observed")
        catalog = self.catalog(); catalog["records"][0]["owner"] = ""
        self.assertEqual(subject.compare_associations(self.native(), catalog)["rows"][0]["status"], "catalog-owner-unavailable")

    def test_incomplete_native_inventory_cannot_prove_unique_match_or_absence(self):
        native = self.native(); native["native_queries_complete"] = False
        for ref in ["pool/work", "pool/missing"]:
            self.assertEqual(subject.compare_associations(native, self.catalog(ref))["rows"][0]["status"], "incomplete-native-inventory")

    def test_snapshot_components_and_repository_members_use_current_native_forms(self):
        catalog = {"projection_complete": True, "records": [{"section": "snapshots", "key": "saved", "components": [
            {"role": "rootfs", "native_ref": "instance/saved-root", "owner": OWNER},
            {"role": "workspace:app", "native_ref": "volume/pool/work", "owner": OWNER}]}]}
        repos = {"projection_complete": True, "files": [{"file": "work-app.json", "records": [{"id": "app", "native_ref": "pool/work", "owner": OWNER}]}]}
        result = subject.compare_associations(self.native(), catalog, repos)
        self.assertEqual(len(result["rows"]), 3)
        self.assertTrue(all(row["status"] == "reference-and-marker-observed" for row in result["rows"]))

    def test_empty_snapshot_and_incomplete_projection_remain_visible(self):
        catalog = {"projection_complete": False, "records": [
            {"section": "snapshots", "key": "empty", "components": []}]}
        result = subject.compare_associations(self.native(), catalog)
        self.assertEqual(result["errors"], ["catalog-projection-incomplete"])
        self.assertEqual(result["rows"][0]["source"]["key"], "empty")
        self.assertEqual(result["rows"][0]["status"], "unsupported-reference")

    def test_unsupported_references_and_budget_are_explicit(self):
        catalog = self.catalog("legacy-format")
        self.assertEqual(subject.compare_associations(self.native(), catalog)["rows"][0]["status"], "unsupported-reference")
        catalog["records"] *= 3
        with patch.object(subject, "LIMIT", 2):
            result = subject.compare_associations(self.native(), catalog)
        self.assertEqual(len(result["rows"]), 2)
        self.assertEqual(result["errors"], ["comparison-budget-exhausted"])

if __name__ == "__main__":
    unittest.main()

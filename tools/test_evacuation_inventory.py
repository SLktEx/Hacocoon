import json
import subprocess
import unittest
from unittest.mock import patch

import evacuation_inventory as subject


class InventoryTests(unittest.TestCase):
    def fixture(self, url):
        if url == "/1.0/storage-pools?recursion=1":
            return [{"name": "data", "driver": "btrfs", "config": {"secret": "never-copy"}}]
        if url == "/1.0/projects?recursion=1":
            return [{"name": "default"}, {"name": "hacocoon"}]
        if url.startswith("/1.0/instances?"):
            return [{"name": "saved-env", "type": "container", "status": "Stopped",
                     "config": {"environment.TOKEN": "never-copy"}, "devices": {"credential": {"type": "proxy", "connect": "never-copy"}}}]
        if "/instances/saved-env/snapshots?" in url:
            return [{"name": "saved"}]
        if "/volumes/custom/work/snapshots?" in url:
            return [{"name": "work/saved"}]
        if "/volumes?" in url:
            return [{"name": "work", "type": "custom"}, {"name": "cached", "type": "image"}]
        self.fail(url)

    def test_inventory_preserves_native_resources_without_credentials_or_backup_claim(self):
        result = subject.inventory(self.fixture)
        self.assertTrue(result["native_queries_complete"])
        self.assertFalse(result["backup_complete"])
        self.assertTrue(result["unreviewed"])
        self.assertNotIn("never-copy", json.dumps(result))
        self.assertEqual(len(result["projects"]), 2)
        self.assertEqual(result["projects"][0]["instances"][0]["snapshots"], ["saved"])
        self.assertEqual(result["projects"][0]["volumes"][0]["snapshots"], ["work/saved"])

    def test_failed_query_retains_other_resources_and_marks_incomplete(self):
        def fetch(url):
            if url.startswith("/1.0/instances?") and "project=default" in url:
                raise subprocess.TimeoutExpired("private-secret", 30)
            return self.fixture(url)
        result = subject.inventory(fetch)
        self.assertFalse(result["native_queries_complete"])
        self.assertEqual(result["errors"], ["instances:default"])
        self.assertEqual(len(result["projects"][1]["instances"]), 1)
        self.assertEqual(len(result["projects"][0]["volumes"]), 2)

    def test_duplicate_or_malformed_rows_are_not_empty_success(self):
        for invalid in [None, {}, [{"name": "x"}, {"name": "x"}], [{"name": []}]]:
            result = subject.inventory(lambda _: invalid)
            self.assertFalse(result["native_queries_complete"])
            self.assertFalse(result["backup_complete"])

    def test_backend_names_are_url_encoded_not_query_or_option_injection(self):
        urls = []
        def fetch(url):
            urls.append(url)
            if url.startswith("/1.0/projects?"):
                return [{"name": "x&project=other"}]
            if url.startswith("/1.0/instances?"):
                return [{"name": "../x?project=other"}]
            return []
        subject.inventory(fetch)
        self.assertTrue(any("project=x%26project%3Dother" in url for url in urls))
        self.assertTrue(any("..%2Fx%3Fproject%3Dother/snapshots" in url for url in urls))

    def test_query_budget_keeps_partial_inventory(self):
        calls = []
        def fetch(url):
            calls.append(url)
            if url.startswith("/1.0/projects?"):
                return [{"name": "hacocoon"}]
            if url.startswith("/1.0/instances?"):
                return [{"name": "env-" + str(i)} for i in range(400)]
            return []
        result = subject.inventory(fetch)
        self.assertEqual(len(calls), 256)
        self.assertFalse(result["native_queries_complete"])
        self.assertIn("native-query-budget-exhausted", result["errors"])
        self.assertGreater(len(result["projects"][0]["instances"]), 0)
    def test_disk_references_are_inventory_not_authority_or_file_access(self):
        value = {"expanded_devices": {
            "root": {"type": "disk", "path": "/", "pool": "data"},
            "work": {"type": "disk", "path": "/work", "pool": "data", "source": "work-volume"},
            "windows": {"type": "disk", "path": "/windows", "source": "/mnt/c/Users/shared"},
            "uri": {"type": "disk", "source": "https://user:secret@example.invalid/data"},
            "proxy": {"type": "proxy", "connect": "secret"}}}
        with patch("builtins.open", side_effect=AssertionError("must not open sources")):
            bindings = subject.disk_bindings(value)
        self.assertEqual(len(bindings), 4)
        self.assertEqual(bindings[1]["source"], "work-volume")
        self.assertEqual(bindings[2]["source"], "/mnt/c/Users/shared")
        self.assertEqual(bindings[3]["source_kind"], "unreported-reference-review-in-incus")
        self.assertNotIn("secret", json.dumps(bindings))
        self.assertTrue(all(x["source_review"] == "required" for x in bindings))
    def test_invalid_attachment_retains_inventory_with_explicit_error(self):
        def fetch(url):
            data = self.fixture(url)
            if url.startswith("/1.0/instances?"):
                data[0]["devices"] = {"broken": "not-a-device"}
            return data
        result = subject.inventory(fetch)
        self.assertFalse(result["native_queries_complete"])
        self.assertTrue(result["projects"][0]["volumes"])
        self.assertIsNone(result["projects"][0]["instances"][0]["disks"])
        self.assertIn("disks:default/saved-env", result["errors"])
    def test_pool_backing_and_block_content_are_references_not_opened_files(self):
        def fetch(url):
            data = self.fixture(url)
            if url.startswith("/1.0/storage-pools?"):
                data[0]["config"]["source"] = "/dev/disk/by-id/external-data"
            if "/volumes?" in url:
                data[0]["content_type"] = "block"
            return data
        with patch("builtins.open", side_effect=AssertionError("must not open sources")):
            result = subject.inventory(fetch)
        self.assertTrue(result["native_queries_complete"])
        self.assertEqual(result["pools"][0]["source"], "/dev/disk/by-id/external-data")
        self.assertEqual(result["pools"][0]["source_review"], "required")
        self.assertEqual(result["projects"][0]["volumes"][0]["content_type"], "block")
        self.assertNotIn("never-copy", json.dumps(result))

    def test_invalid_pool_reference_keeps_other_data_and_requires_review(self):
        def fetch(url):
            data = self.fixture(url)
            if url.startswith("/1.0/storage-pools?"):
                data[0]["config"] = []
            return data
        result = subject.inventory(fetch)
        self.assertFalse(result["native_queries_complete"])
        self.assertIn("pool-source:data", result["errors"])
        self.assertEqual(result["pools"][0]["source_kind"], "unavailable")
        self.assertTrue(result["projects"][0]["volumes"])

    def test_uri_pool_reference_does_not_publish_credentials(self):
        def fetch(url):
            data = self.fixture(url)
            if url.startswith("/1.0/storage-pools?"):
                data[0]["config"]["source"] = "https://user:never-copy@example.invalid/data"
            return data
        result = subject.inventory(fetch)
        self.assertNotIn("never-copy", json.dumps(result))
        self.assertEqual(result["pools"][0]["source_kind"], "unreported-reference-review-in-incus")

    @patch("evacuation_inventory.subprocess.run")
    def test_command_is_read_only_and_errors_are_not_exposed(self, run):
        run.return_value = subprocess.CompletedProcess([], 0, b"[]", b"secret")
        self.assertEqual(subject.query("/1.0/projects?recursion=1"), [])
        self.assertEqual(run.call_args.args[0], ["incus", "query", "/1.0/projects?recursion=1"])
        self.assertNotIn("shell", run.call_args.kwargs)
        run.return_value.returncode = 1
        with self.assertRaisesRegex(ValueError, "query unavailable"):
            subject.query("/1.0/projects?recursion=1")


if __name__ == "__main__":
    unittest.main()
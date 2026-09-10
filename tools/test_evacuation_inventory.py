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
                     "config": {"environment.TOKEN": "never-copy"}, "devices": {"credential": "never-copy"}}]
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
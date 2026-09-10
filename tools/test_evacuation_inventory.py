import json
import os
import tempfile
from pathlib import Path
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

    def test_catalog_projection_preserves_saved_components_without_config(self):
        data = {"version": 13, "snapshots": {"saved": {"id": "saved", "state": "ready", "source": {"secret": "never-copy"}, "components": [{"role": "workspace", "native_ref": "pool/saved-work", "owner": "abc", "state": "ready", "binding": "never-copy"}]}}, "persistent_resources": {"oci:work": {"id": "oci:work", "owner": "abc", "kind": "oci", "native_ref": "pool/work", "state": "ready"}}, "workspace_leases": {"dev": {"workspace_id": "work", "instance_id": "new-generation", "persistent_resource": {"id": "oci:work", "owner": "abc"}}}}
        original = json.dumps(data, sort_keys=True)
        result = subject.catalog_references(data)
        self.assertTrue(result["projection_complete"])
        self.assertFalse(result["authority"])
        self.assertEqual(len(result["records"]), 3)
        self.assertNotIn("never-copy", json.dumps(result))
        self.assertEqual(json.dumps(data, sort_keys=True), original)
        self.assertTrue(result["unreviewed"])

    def test_catalog_unknown_schema_and_bad_rows_remain_incomplete(self):
        for version in (12, 14, True, None):
            self.assertFalse(subject.catalog_references({"version": version})["projection_complete"])
        data = {"version": 13, "persistent_resources": {"bad": {"native_ref": "https://user:secret@example.invalid"}, "good": {"native_ref": "pool/volume"}}}
        result = subject.catalog_references(data)
        self.assertFalse(result["projection_complete"])
        self.assertEqual(len(result["records"]), 1)
        self.assertNotIn("secret", json.dumps(result))

    @unittest.skipUnless(hasattr(os, "O_NOFOLLOW"), "Linux no-follow catalog reader required")
    def test_catalog_file_observation_does_not_rewrite_or_follow_symlink(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "catalog.json"
            raw = b'{"version":13,"persistent_resources":{}}'
            path.write_bytes(raw)
            before = path.stat()
            self.assertTrue(subject.catalog_inventory(path)["projection_complete"])
            self.assertEqual(path.read_bytes(), raw)
            self.assertEqual(path.stat().st_mtime_ns, before.st_mtime_ns)
            alias = Path(directory) / "alias"
            alias.symlink_to(path)
            with self.assertRaises(OSError):
                subject.catalog_inventory(alias)

    @unittest.skipUnless(hasattr(os, "O_NOFOLLOW"), "Linux no-follow catalog reader required")
    def test_catalog_special_file_is_refused_without_blocking(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "fifo"
            os.mkfifo(path)
            with self.assertRaises(ValueError):
                subject.catalog_inventory(path)

    def test_repository_collection_includes_members_without_remote_credentials(self):
        member = {"kind": "work", "id": "group-app", "owner": "abc", "state": "ready", "native_ref": "pool/work", "remote": "https://user:secret@example.invalid/repo"}
        result = subject.repository_references({"kind": "work", "id": "group", "owner": "abc", "state": "ready", "members": [member]})
        self.assertTrue(result["projection_complete"])
        self.assertEqual(len(result["records"]), 2)
        self.assertEqual(result["records"][1]["native_ref"], "pool/work")
        self.assertNotIn("secret", json.dumps(result))
        member["members"] = [member.copy()]
        self.assertFalse(subject.repository_references({"kind": "work", "id": "group", "owner": "abc", "state": "ready", "members": [member]})["projection_complete"])

    @unittest.skipUnless(hasattr(os, "O_NOFOLLOW"), "Linux directory observation required")
    def test_repository_directory_preserves_good_files_and_refuses_links_mismatch(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            good = root / "work-app.json"
            raw = json.dumps({"kind": "work", "id": "app", "owner": "abc", "state": "ready", "native_ref": "pool/work"})
            good.write_text(raw)
            (root / "work-link.json").symlink_to(good)
            (root / "work-mismatch.json").write_text(raw)
            (root / "manual.txt").write_text("secret")
            result = subject.repository_inventory(root)
            self.assertFalse(result["projection_complete"])
            self.assertEqual(len(result["files"]), 1)
            self.assertEqual(len(result["errors"]), 3)
            self.assertEqual({entry["name"] for entry in result["unreviewed_entries"]}, {"manual.txt", "work-link.json", "work-mismatch.json"})
            self.assertEqual(len({entry["index"] for entry in result["unreviewed_entries"]}), 3)
            self.assertNotIn("secret", json.dumps(result))
            self.assertEqual(good.read_text(), raw)

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
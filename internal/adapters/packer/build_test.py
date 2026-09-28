import base64
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location("worker", Path(__file__).with_name("build.py"))
worker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(worker)


class WorkerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        patch = mock.patch.object(worker, "ROOT", self.root)
        patch.start()
        self.addCleanup(patch.stop)
        patch = mock.patch.object(worker, "trusted_host")
        patch.start()
        self.addCleanup(patch.stop)
        self.build = worker.Build("a" * 32, "tools", 1 << 40)
        self.calls = []
        self.timeouts = []
        self.images = [{"fingerprint": "b" * 64, "type": "container", "public": False,
                        "architecture": "amd64", "properties": {worker.OWNER: "a" * 32}}]
        self.project_exists = True
        self.instances = []
        self.fail = None

        def run(*args, capture=False, timeout=120):
            if args[:3] == ("incus", "profile", "delete"):
                raise RuntimeError("Incus default profile cannot be deleted")
            self.calls.append(args)
            self.timeouts.append((args, timeout))
            if self.fail == self.build.value["stage"]:
                raise RuntimeError("private error")
            if args[:3] == ("incus", "image", "export"):
                (self.build.directory / "export" / "image.tar").write_bytes(b"native archive fixture")
            if args[:3] == ("incus", "image", "delete"):
                self.images = []
            if args[:3] == ("incus", "project", "delete"):
                self.project_exists = False

        def query(path):
            if path.startswith("/1.0/projects/"):
                return {"name": self.build.value["project"], "config": {worker.OWNER: "a" * 32}}
            if path.startswith("/1.0/images"):
                return self.images
            if path.startswith("/1.0/instances"):
                return self.instances
            if path.startswith("/1.0/projects?"):
                return [{"name": self.build.value["project"]}] if self.project_exists else []
            raise AssertionError(path)

        self.build.run = run
        self.build.query = query
        self.template = {"files": [{"path": "base.pkr.hcl", "data": base64.b64encode(b"source").decode()}]}

    def test_export_limit_retains_exact_identity(self):
        self.build.max_bytes = 4
        with self.assertRaises(ValueError):
            self.build.prepare(self.template)
        self.assertEqual(self.build.value["stage"], "export")
        self.assertEqual(self.build.value["fingerprint"], "b" * 64)
        self.assertTrue(self.project_exists)
        self.assertFalse(any(c[:3] == ("incus", "image", "delete") for c in self.calls))

    def test_export_and_exact_cleanup_before_import(self):
        self.build.prepare(self.template)
        self.assertIn(("incus", "profile", "set", "default", "security.idmap.size", "65536"), self.calls)
        for args, timeout in self.timeouts:
            if args[0] == "/usr/local/bin/packer" or args[:3] == ("incus", "image", "export"):
                self.assertIsNone(timeout, args)
        receipt = json.loads((self.build.directory / "receipt.json").read_text())
        self.assertEqual(receipt["state"], "artifact-ready")
        self.assertEqual(receipt["fingerprint"], "b" * 64)
        self.assertEqual(receipt["cleanup_images"], ["b" * 64])
        self.assertFalse((self.build.directory / "source").exists())
        self.assertEqual(sorted(p.name for p in self.build.directory.iterdir()), ["export", "receipt.json"])
        self.assertIn(("incus", "image", "delete", "b" * 64), self.calls)
        self.assertFalse(any("--force" in call for call in self.calls))

    def test_packer_stages_abort_before_publication(self):
        for stage in ("fmt", "init", "validate", "build"):
            with self.subTest(stage=stage):
                self.setUp()
                self.fail = stage
                with self.assertRaises(RuntimeError):
                    self.build.prepare(self.template)
                self.assertFalse(any(call[:3] == ("incus", "image", "export") for call in self.calls))
                self.assertFalse(any(call[:3] == ("incus", "image", "delete") for call in self.calls))

    def test_invalid_context_never_runs_packer(self):
        self.template["files"][0]["path"] = "../escape"
        with self.assertRaises(ValueError):
            self.build.prepare(self.template)
        self.assertEqual(self.calls, [])

    def test_foreign_or_missing_output_never_exports(self):
        self.images[0]["properties"] = {}
        with self.assertRaises(ValueError):
            self.build.prepare(self.template)
        self.assertFalse(any(call[:3] == ("incus", "image", "export") for call in self.calls))
        self.assertTrue(self.project_exists)

    def test_plugin_cleanup_failure_retains_images(self):
        self.instances = [{"name": "exact-owned-instance"}]
        with self.assertRaises(ValueError):
            self.build.prepare(self.template)
        self.assertTrue(self.images)
        self.assertTrue(self.project_exists)
        self.assertEqual(self.build.value["stage"], "cleanup")

    def test_export_failure_retains_identity(self):
        self.fail = "export"
        with self.assertRaises(RuntimeError):
            self.build.prepare(self.template)
        self.assertEqual(self.build.value["fingerprint"], "b" * 64)
        self.assertTrue(self.images)


if __name__ == "__main__":
    unittest.main()

"""Exercise setup refusal/reuse without granting the test host Incus authority."""
import importlib.util
from pathlib import Path
import subprocess
import unittest
from unittest import mock

root = Path(__file__).resolve().parent.parent
spec = importlib.util.spec_from_file_location("tooling", root / "host_tooling.py")
tooling = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tooling)
exec(compile((root / "host_packer.py").read_text(), "host_packer.py", "exec"), tooling.__dict__)
tooling.INCUS_LTS_SCRIPT = "fixture installer"


class QueryBoundsTests(unittest.TestCase):
    def test_query_output_limit_is_enforced_in_child(self):
        actual_run = subprocess.run
        def child(argv, **kwargs):
            return actual_run(["/usr/bin/python3", "-c", "import os; os.write(1,b'x'*(2<<20)); os.write(1,b'y')"], **kwargs)
        with mock.patch.object(tooling.subprocess, "run", side_effect=child):
            with self.assertRaises(subprocess.CalledProcessError):
                tooling.nested_query("/1.0")


class NestedSetupTests(unittest.TestCase):
    def setUp(self):
        self.calls = []
        self.responses = {
            "/1.0": {"config": {}},
            "/1.0/instances": [], "/1.0/images": [], "/1.0/storage-pools": [],
            "/1.0/projects": ["/1.0/projects/default"],
            "/1.0/profiles?recursion=1": [{"name":"default", "config":{}, "devices":{}}],
            "/1.0/storage-pools?recursion=1": [], "/1.0/networks?recursion=1": [],
        }
        for name, replacement in (
                ("run", lambda args: self.calls.append(args)),
                ("nested_query", lambda path: self.responses[path]),
                ("directory", lambda path: None)):
            patch = mock.patch.object(tooling, name, replacement)
            patch.start()
            self.addCleanup(patch.stop)
        for owner, name, options in (
                (tooling.os, "chmod", {}),
                (tooling.os.path, "exists", {"return_value": True}),
                (tooling.subprocess, "run", {})):
            patch = mock.patch.object(owner, name, **options)
            patch.start()
            self.addCleanup(patch.stop)

    def test_fresh_daemon_and_owned_reuse(self):
        tooling.nested_incus()
        self.assertIn(["/usr/bin/incus", "config", "set", "user.hacocoon.packer", tooling.NESTED_OWNER], self.calls)
        self.assertTrue(any("storage" in c and "create" in c for c in self.calls))
        self.responses["/1.0"]["config"] = {"user.hacocoon.packer": tooling.NESTED_OWNER}
        self.responses["/1.0/storage-pools?recursion=1"] = [{"name": "haco-packer", "driver": "dir", "config": {"user.hacocoon.owner": tooling.NESTED_OWNER}}]
        self.responses["/1.0/networks?recursion=1"] = [{"name": "haco-packer0", "type": "bridge", "managed": True, "config": {"user.hacocoon.owner": tooling.NESTED_OWNER, "ipv4.nat": "true", "ipv6.address": "none"}}]
        self.calls.clear()
        tooling.nested_incus()
        self.assertFalse(any("create" in c or "delete" in c for c in self.calls))

    def test_foreign_workloads_are_never_adopted(self):
        for path in ("/1.0/instances", "/1.0/images", "/1.0/storage-pools"):
            with self.subTest(path=path):
                self.responses[path] = ["foreign"]
                with self.assertRaises(ValueError):
                    tooling.nested_incus()
                self.responses[path] = []
        self.assertFalse(any("config" in c and "set" in c for c in self.calls))

    def test_foreign_profile_is_never_adopted(self):
        self.responses["/1.0/profiles?recursion=1"][0]["config"] = {"limits.cpu":"2"}
        with self.assertRaises(ValueError):
            tooling.nested_incus()
        self.assertFalse(any("config" in c and "set" in c for c in self.calls))

    def test_foreign_listener_owner_or_compression_is_refused(self):
        for config in ({"core.https_address": ":8443"}, {"user.hacocoon.packer": "foreign"}, {"images.compression_algorithm": "gzip"}):
            with self.subTest(config=config):
                self.responses["/1.0"]["config"] = config
                with self.assertRaises(ValueError):
                    tooling.nested_incus()
        self.assertFalse(any("config" in c and "set" in c for c in self.calls))

    def test_owned_network_configuration_drift_is_refused(self):
        self.responses["/1.0"]["config"] = {"user.hacocoon.packer": tooling.NESTED_OWNER}
        self.responses["/1.0/networks?recursion=1"] = [{"name": "haco-packer0", "type": "bridge", "managed": True, "config": {"user.hacocoon.owner": tooling.NESTED_OWNER, "ipv4.nat": "false", "ipv6.address": "none"}}]
        with self.assertRaises(ValueError):
            tooling.nested_incus()
        self.assertFalse(any("delete" in c for c in self.calls))

    def test_wrong_packer_cache_never_installs_binary(self):
        with mock.patch.object(tooling, "read_file", return_value=b"corrupt download"), mock.patch.object(tooling, "publish") as publish:
            with self.assertRaises(ValueError):
                tooling.packer_binary()
            publish.assert_not_called()

    def test_incompatible_incus_version_stops_before_adoption(self):
        tooling.subprocess.run.side_effect = subprocess.CalledProcessError(1, "verify-server")
        with self.assertRaises(subprocess.CalledProcessError):
            tooling.nested_incus()
        self.assertFalse(any("config" in c and "set" in c for c in self.calls))


if __name__ == "__main__":
    unittest.main()
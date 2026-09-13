import importlib.util
import io
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location("tooling", sys.argv.pop(1))
tooling = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tooling)


class HostToolingTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = self.temp.name
        # The real helper only uses fixed root-owned system directories. Stop
        # recursion at this test-owned root, never change /tmp permissions.
        real_directory = tooling.directory

        def directory(path):
            if path == self.root:
                return
            self.assertTrue(path.startswith(self.root + "/"))
            real_directory(path)

        patch = mock.patch.object(tooling, "directory", directory)
        patch.start()
        self.addCleanup(patch.stop)
        # CI normally runs non-root. Model owned files using this exact UID.
        original_stat = tooling.os.lstat
        original_fstat = tooling.os.fstat

        def owner(info):
            self.assertEqual(info.st_uid, os.geteuid())
            values = list(info)
            values[4] = 0
            return os.stat_result(values)

        for name, original in [("lstat", original_stat), ("fstat", original_fstat)]:
            patch = mock.patch.object(tooling.os, name, lambda arg, fn=original: owner(fn(arg)))
            patch.start()
            self.addCleanup(patch.stop)

    def test_publish_repeat_partial_retry_and_custom_file(self):
        one, two = self.root + "/bin/one", self.root + "/bin/two"
        self.assertTrue(tooling.publish(one, b"expected", 0o755))
        inode = os.stat(one).st_ino
        self.assertFalse(tooling.publish(one, b"expected", 0o755))
        self.assertEqual(os.stat(one).st_ino, inode)
        self.assertTrue(tooling.publish(two, b"next", 0o755))
        with self.assertRaises(ValueError):
            tooling.publish(one, b"different", 0o755)
        self.assertEqual(Path(one).read_bytes(), b"expected")

    def test_links_writable_files_and_parents_are_rejected(self):
        outside = self.root + "/keep"
        Path(outside).write_bytes(b"keep")
        for kind in ("symlink", "hardlink", "writable", "parent-link", "parent-writable"):
            with self.subTest(kind=kind):
                parent = self.root + "/" + kind
                target = parent + "/tool"
                if kind == "parent-link":
                    os.symlink(self.root, parent)
                else:
                    os.mkdir(parent)
                if kind == "symlink":
                    os.symlink(outside, target)
                elif kind == "hardlink":
                    os.link(outside, target)
                elif kind == "writable":
                    Path(target).write_bytes(b"keep")
                    os.chmod(target, 0o666)
                elif kind == "parent-writable":
                    os.chmod(parent, 0o777)
                with self.assertRaises((ValueError, OSError)):
                    tooling.publish(target, b"keep", 0o755)
                self.assertEqual(Path(outside).read_bytes(), b"keep")

    def archive(self, mutation):
        result = io.BytesIO()
        names = ["bin/" + name for name in tooling.BINARIES]
        names += ["libexec/cni/" + name for name in tooling.CNI]
        with tarfile.open(fileobj=result, mode="w") as archive:
            for index, name in enumerate(names):
                entry = tarfile.TarInfo(name)
                entry.size, entry.mode = 4, 0o755
                if index == 0:
                    if mutation == "missing":
                        continue
                    if mutation in ("symlink", "hardlink"):
                        entry.type = tarfile.SYMTYPE if mutation == "symlink" else tarfile.LNKTYPE
                        entry.linkname, entry.size = "/etc/passwd", 0
                    if mutation == "setuid":
                        entry.mode = 0o4755
                archive.addfile(entry, io.BytesIO(b"test") if entry.isfile() else None)
                if index == 0 and mutation == "duplicate":
                    archive.addfile(entry, io.BytesIO(b"test"))
            for name in ("../../escape", "/etc/passwd", "bin/docker"):
                entry = tarfile.TarInfo(name)
                entry.size = 4
                archive.addfile(entry, io.BytesIO(b"evil"))
        result.seek(0)
        return tarfile.open(fileobj=result, mode="r")

    def test_only_complete_regular_allowlisted_payloads(self):
        for kind in ("ok", "missing", "symlink", "hardlink", "duplicate", "setuid"):
            with self.subTest(kind=kind), self.archive(kind) as archive:
                if kind != "ok":
                    with self.assertRaises(ValueError):
                        tooling.selected_files(archive)
                else:
                    selected = tooling.selected_files(archive)
                    self.assertEqual(len(selected), len(tooling.BINARIES) + len(tooling.CNI))
                    self.assertTrue(all(path.startswith("/usr/local/") for path, _ in selected))
                    self.assertFalse(any(path.endswith("/docker") for path, _ in selected))

    def test_download_rejects_bad_digest_before_publication(self):
        response = mock.MagicMock()
        response.__enter__.return_value = response
        response.url = "https://release-assets.githubusercontent.com/fixture"
        response.read.return_value = b"tampered"
        opener = mock.Mock()
        opener.open.return_value = response
        with mock.patch.object(tooling, "directory"), \
                mock.patch.object(tooling, "read_file", side_effect=FileNotFoundError), \
                mock.patch.object(tooling.urllib.request, "build_opener", return_value=opener), \
                mock.patch.object(tooling, "publish") as publish:
            for arch in tooling.DIGESTS:
                with self.assertRaises(ValueError):
                    tooling.download(arch)
            publish.assert_not_called()

    def test_packages_do_not_upgrade_on_repeat_and_failure_stops(self):
        installed = subprocess.CompletedProcess([], 0, b"install ok installed")
        with mock.patch.object(tooling.subprocess, "run", return_value=installed), \
                mock.patch.object(tooling, "run") as run:
            tooling.packages()
            self.assertEqual([call.args[0] for call in run.call_args_list],
                             [["/usr/bin/git", "--version"], ["/usr/bin/gh", "--version"]])
        absent = subprocess.CompletedProcess([], 1, b"")
        with mock.patch.object(tooling.subprocess, "run", return_value=absent), \
                mock.patch.object(tooling, "run", side_effect=RuntimeError) as run:
            with self.assertRaises(RuntimeError):
                tooling.packages()
            self.assertEqual(run.call_count, 1)

    def test_native_config_both_supported_architectures(self):
        import tomllib
        for machine, arch in (("x86_64", "amd64"), ("aarch64", "arm64")):
            with mock.patch.object(tooling.platform, "machine", return_value=machine):
                self.assertEqual(tooling.architecture(), arch)
                config = tomllib.loads(tooling.native_config(arch))
                self.assertEqual(config["root"], "/var/lib/hacocoon-oci/containerd")
                self.assertEqual(config["state"], "/run/containerd")
                self.assertEqual(config["plugins"]["io.containerd.transfer.v1.local"]["unpack_config"],
                                 [{"platform": "linux/" + arch, "snapshotter": "native"}])
        with mock.patch.object(tooling.platform, "machine", return_value="unknown"):
            with self.assertRaises(KeyError):
                tooling.architecture()

    def test_services_notify_readiness_and_repeat_preserves_data(self):
        # Use the real paired Environment service definition as the initial
        # configuration supplied by managed Store attachment.
        source = Path(spec.origin).with_name("persistent_resource.go").read_text()
        buildkit = source.split("<<'HACO_BUILDKIT'\n", 1)[1].split("HACO_BUILDKIT\n", 1)[0]
        for relative, data in {
            "/etc/containerd/config.toml": tooling.CONTAINERD.encode(),
            "/etc/systemd/system/buildkit.service": buildkit.encode(),
            "/var/lib/hacocoon-oci/containerd/image": b"retained-image",
            "/var/lib/hacocoon-oci/buildkit/cache": b"retained-cache",
            "/var/lib/hacocoon-oci/docker/image": b"retained-docker",
        }.items():
            path = Path(self.root + relative)
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
        real_read, real_publish = tooling.read_file, tooling.publish
        # Both wrappers map the helper's fixed absolute paths into this fixture.
        def read(path, limit):
            return real_read(path if path.startswith(self.root) else self.root + path, limit)

        def publish(path, data, mode, previous=None):
            return real_publish(self.root + path, data, mode, previous)

        with mock.patch.object(tooling, "read_file", read), \
                mock.patch.object(tooling, "publish", publish), \
                mock.patch.object(tooling, "run") as run:
            tooling.services()
            unit = Path(self.root + "/etc/systemd/system/containerd.service")
            inode = unit.stat().st_ino
            tooling.services()
            self.assertEqual(unit.stat().st_ino, inode)
            self.assertIn("Type=notify\n", unit.read_text())
            self.assertIn("Type=notify\n", Path(self.root + "/etc/systemd/system/buildkit.service.d/10-hacocoon-readiness.conf").read_text())
            self.assertFalse(any("restart" in call.args[0] for call in run.call_args_list))
            run.side_effect = RuntimeError("service start failure")
            with self.assertRaises(RuntimeError):
                tooling.services()
        for runtime, name, data in (("containerd", "image", b"retained-image"),
                                    ("buildkit", "cache", b"retained-cache"),
                                    ("docker", "image", b"retained-docker")):
            self.assertEqual(Path(self.root + "/var/lib/hacocoon-oci/" + runtime + "/" + name).read_bytes(), data)


if __name__ == "__main__":
    unittest.main()

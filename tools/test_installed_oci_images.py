"""Pure acceptance-harness regressions; no installed or native runtime claim."""
import contextlib
import copy
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import types
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location(
    "installed_oci_images", Path(__file__).resolve().parents[1] / "test/e2e/installed/oci_images.py")
acceptance = importlib.util.module_from_spec(spec)
spec.loader.exec_module(acceptance)


def store(identifier="oci-source:host", owner="a" * 32, workspace=""):
    return dict(id=identifier, owner=owner, kind="oci-containerd", native_ref="pool/" + owner,
                state="ready", created_at="2026-10-10T00:00:00Z", workspace_id=workspace,
                source_only=identifier == "oci-source:host")


class Product:
    """Models CLI JSON shapes and independent inventories, not runtime behavior."""
    def __init__(self, fixture):
        self.fixture = fixture
        self.stores = {"oci-source:host": store()}
        self.envs, self.images = {}, {"oci-source:host": {}}
        self.calls = []
        self.recipe = None
        self.sequence = 0
        self.fail_delete = False
        self.overwrite_retained = False
        self.copy_is_alias = False
        self.ignore_no_oci = False

    def observe_recipe(self, expected):
        if expected == "absent":
            acceptance.require(self.recipe is None, "existing Host recipe")
            return {"absent": True, "parents": [[1, 1]]}
        acceptance.require(self.recipe == expected, "recipe changed")
        return {"parents": [[1, 1]], "file": [1, 2], "sha256": expected}

    def run(self, *args, **kwargs):
        self.calls.append(args)
        result = {}
        if args == ("env", "list", "--json"):
            result = list(self.envs.values())
        elif args[:3] == ("env", "status", "--json"):
            result = {"environment": self.envs[args[3]], "state": "running"}
        elif args[:2] == ("env", "create"):
            name, path = args[-1], args[args.index("--workspace") + 1]
            if "--no-oci" in args and "--resource" in args:
                return subprocess.CompletedProcess(args, 1, "", "")
            self.sequence += 1
            work = hashlib.sha256(path.encode()).hexdigest()[:32]
            env = dict(name=name, created_at=f"2026-10-10T00:00:{self.sequence:02d}Z",
                       runtime_ref="haco-" + name, workspace={"id": "workspace:" + work, "path": path},
                       persistent_resource={})
            if "--no-oci" not in args or self.ignore_no_oci:
                identifier = "oci:auto-" + work
                if identifier not in self.stores:
                    self.stores[identifier] = store(identifier, format(self.sequence, "032x"), "workspace:" + work)
                    source = self.images["oci-source:host"]
                    self.images[identifier] = source if self.copy_is_alias else copy.deepcopy(source)
                elif self.overwrite_retained:
                    self.images[identifier] = copy.deepcopy(self.images["oci-source:host"])
                env["persistent_resource"] = acceptance.ref(self.stores[identifier])
            self.envs[name] = env
            result = env
        elif args[:2] == ("env", "delete"):
            if self.fail_delete:
                raise RuntimeError("uncertain delete")
            del self.envs[args[-1]]
        elif args[:4] == ("plugin", "oci", "store", "list"):
            result = {"resources": list(self.stores.values()), "uses": [
                {"resource": acceptance.ref(row), "role": "host-source" if row["source_only"] else "independent-store",
                 "environments": [env["name"] for env in self.envs.values()
                                  if env["persistent_resource"] == acceptance.ref(row)],
                 "pending_copies": [], "independent_snapshots": []}
                for row in self.stores.values()]}
        elif args[:4] == ("plugin", "oci", "store", "delete"):
            del self.stores["oci:" + args[-1]]
        elif args[:4] == ("plugin", "oci", "image", "list"):
            identifier = "oci-source:host" if "--host" in args else args[-1]
            if identifier in self.envs and not self.envs[identifier]["persistent_resource"]:
                return subprocess.CompletedProcess(args, 1, "", "")
            source = self.stores[identifier]
            result = {"target": {"host": source["source_only"], "detached": not source["source_only"],
                                 "environment": "", "instance": "", "runtime": "nerdctl", "store": acceptance.ref(source)},
                      "images": [{"id": key, "tags": value, "digests": [], "containers": []}
                                 for key, value in self.images[identifier].items()]}
        elif args[:4] == ("plugin", "oci", "image", "delete"):
            identifier = "oci-source:host" if "--host" in args else args[-2]
            del self.images[identifier][args[-1]]
        elif args[:2] == ("setup", "--script"):
            self.recipe = hashlib.sha256(Path(args[2]).read_bytes()).hexdigest()
            self.images["oci-source:host"] = {
                "sha256:" + digit * 64: [f"example.invalid/{self.fixture.prefix}:{label}"]
                for digit, label in (("1", "remove"), ("2", "keep"))}
        elif args == ("setup", "--clear-script"):
            self.recipe = None
        else:
            raise AssertionError("unexpected command: " + repr(args))
        return subprocess.CompletedProcess(args, 0, json.dumps(result), "")


class FixtureTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.fixture = acceptance.Acceptance(self.root)
        self.product = Product(self.fixture)
        self.fixture.haco = self.product.run
        patcher = mock.patch.object(acceptance, "recipe_observation", side_effect=self.product.observe_recipe)
        patcher.start()
        self.addCleanup(patcher.stop)

    def test_complete_cli_flow_and_exact_cleanup(self):
        acceptance.execute(self.fixture)
        creates = [args for args in self.product.calls if args[:2] == ("env", "create")]
        self.assertEqual(len(creates), 5)
        self.assertTrue(all("--resource" not in args and "--no-oci" not in args for args in creates[:3]))
        self.assertIn("--no-oci", creates[3])
        self.assertIn("--resource", creates[4])
        self.assertEqual(creates[0][4], creates[1][4])
        self.assertNotEqual(creates[0][4], creates[2][4])
        self.assertEqual(list(self.product.stores), ["oci-source:host"])
        self.assertEqual(self.product.images["oci-source:host"], {})
        self.assertEqual(self.product.envs, {})
        self.assertIsNone(self.product.recipe)
        self.assertIsNone(self.fixture.pending)
        self.assertEqual((self.root / "owned.json").stat().st_mode & 0o777, 0o600)
        self.assertFalse(any(args[0] in ("exec", "config") for args in self.product.calls))

    def test_oracle_rejects_host_alias_and_retained_overwrite_and_ignored_optout(self):
        for broken, message in (("copy_is_alias", "copy deletion changed Host"),
                                ("overwrite_retained", "recreation replaced guest changes"),
                                ("ignore_no_oci", "--no-oci attached")):
            with self.subTest(broken=broken), tempfile.TemporaryDirectory() as directory:
                fixture = acceptance.Acceptance(Path(directory))
                product = Product(fixture)
                setattr(product, broken, True)
                fixture.haco = product.run
                with mock.patch.object(acceptance, "recipe_observation", side_effect=product.observe_recipe), \
                        self.assertRaisesRegex(RuntimeError, message):
                    fixture.exercise()

    def test_unexpected_or_mistagged_images_never_become_cleanup_authority(self):
        for case in ("extra", "wrong_tag", "both_tags_and_untagged"):
            with self.subTest(case=case), tempfile.TemporaryDirectory() as directory:
                fixture = acceptance.Acceptance(Path(directory))
                product = Product(fixture)
                original = product.run
                def run(*args, **kwargs):
                    result = original(*args, **kwargs)
                    if args[:2] == ("setup", "--script"):
                        images = product.images["oci-source:host"]
                        if case == "extra":
                            images["sha256:" + "3" * 64] = ["example.invalid/unrelated:keep"]
                        elif case == "wrong_tag":
                            images["sha256:" + "1" * 64] = ["example.invalid/unrelated:keep"]
                        else:
                            images["sha256:" + "1" * 64] += images["sha256:" + "2" * 64]
                            images["sha256:" + "2" * 64] = []
                    return result
                fixture.haco = run
                with mock.patch.object(acceptance, "recipe_observation", side_effect=product.observe_recipe), \
                        self.assertRaisesRegex(RuntimeError, "two distinct fixture images"):
                    acceptance.execute(fixture)
                self.assertEqual(fixture.images, {})
                self.assertFalse(any(args[:4] == ("plugin", "oci", "image", "delete") for args in product.calls))
                self.assertGreaterEqual(len(product.images["oci-source:host"]), 2)

    def test_existing_recipe_refuses_before_setup_or_other_mutation(self):
        self.product.recipe = "existing"
        with self.assertRaisesRegex(RuntimeError, "existing Host recipe"):
            self.fixture.exercise()
        self.assertFalse(any(args[0] == "setup" or args[:2] == ("env", "create") for args in self.product.calls))

    def test_replaced_recipe_cannot_be_cleared(self):
        self.fixture.recipe = {"parents": [[1, 1]], "file": [1, 2], "sha256": "a" * 64}
        self.product.recipe = "b" * 64
        with self.assertRaisesRegex(RuntimeError, "recipe changed"):
            self.fixture.clear_recipe()
        self.assertEqual(self.product.calls, [])

    def test_same_hash_replaced_recipe_inode_cannot_be_cleared(self):
        self.fixture.recipe = {"parents": [[1, 1]], "file": [1, 3], "sha256": "a" * 64}
        self.product.recipe = "a" * 64
        with self.assertRaisesRegex(RuntimeError, "identity changed"):
            self.fixture.clear_recipe()
        self.assertEqual(self.product.calls, [])

    def test_uncertain_create_is_never_adopted_or_cleaned(self):
        self.fixture.source = store()
        original = self.product.run
        def fail(*args, **kwargs):
            result = original(*args, **kwargs)
            if args[:2] == ("env", "create"):
                raise RuntimeError("response lost")
            return result
        self.fixture.haco = fail
        workspace = self.fixture.workspace("lost")
        with self.assertRaisesRegex(RuntimeError, "response lost"):
            self.fixture.create("lost", workspace)
        with self.assertRaisesRegex(RuntimeError, "unconfirmed creation"):
            self.fixture.cleanup()
        self.assertFalse(any(args[:2] == ("env", "delete") for args in self.product.calls))
        self.assertTrue(json.loads((self.root / "owned.json").read_text())["pending"])

    def test_uncertain_delete_is_never_retried(self):
        self.fixture.source = store()
        env = self.fixture.create("delete", self.fixture.workspace("delete"))
        self.product.fail_delete = True
        with self.assertRaisesRegex(RuntimeError, "uncertain delete"):
            self.fixture.remove_environment(env)
        with self.assertRaisesRegex(RuntimeError, "unconfirmed creation"):
            self.fixture.cleanup()
        self.assertEqual(sum(args[:2] == ("env", "delete") for args in self.product.calls), 1)

    def test_replaced_environment_and_store_refuse_cleanup(self):
        self.fixture.source = store()
        env = self.fixture.create("replace", self.fixture.workspace("replace"))
        self.product.envs[env["name"]]["created_at"] = "replacement"
        with self.assertRaisesRegex(RuntimeError, "Environment ownership changed"):
            self.fixture.cleanup()
        self.assertFalse(any(args[:2] == ("env", "delete") for args in self.product.calls))
        self.fixture.environments = []
        self.product.stores[self.fixture.stores[0]["id"]]["owner"] = "b" * 32
        with self.assertRaisesRegex(RuntimeError, "Store ownership changed"):
            self.fixture.cleanup()
        self.assertFalse(any(args[:4] == ("plugin", "oci", "store", "delete") for args in self.product.calls))

    def test_workspace_cleanup_never_follows_link_or_deletes_guest_data(self):
        path = self.fixture.workspace("changed")
        (path / "guest-file").write_text("retain")
        with self.assertRaises(OSError):
            self.fixture.cleanup()
        self.assertTrue((path / "guest-file").exists())
        (path / "guest-file").unlink()
        path.rmdir()
        path.symlink_to(self.root, target_is_directory=True)
        with self.assertRaisesRegex(RuntimeError, "directory changed"):
            self.fixture.cleanup()
        self.assertTrue(path.is_symlink())

    def test_primary_error_survives_cleanup_error(self):
        primary = RuntimeError("primary operation")
        self.fixture.exercise = mock.Mock(side_effect=primary)
        self.fixture.cleanup = mock.Mock(side_effect=RuntimeError("cleanup"))
        with contextlib.redirect_stdout(io.StringIO()), self.assertRaises(RuntimeError) as raised:
            acceptance.execute(self.fixture)
        self.assertIs(raised.exception, primary)

    def test_recipe_contains_only_local_scratch_builds_and_exact_context_cleanup(self):
        script = acceptance.host_recipe(self.fixture.prefix)
        self.assertEqual(script.count("--network none"), 2)
        self.assertEqual(script.count("FROM scratch"), 1)
        for forbidden in ("curl", "wget", "apt-get", " pull ", " load ", " save ", "incus", "systemctl", "rm -rf"):
            self.assertNotIn(forbidden, script)
        subprocess.run(["bash", "-n"], input=script, text=True, check=True)
        with self.assertRaises(RuntimeError):
            acceptance.host_recipe("oci-; echo bad")


class ObservationTests(unittest.TestCase):
    def test_inventory_rejects_wrong_owner_target_id_duplicates_and_container_users(self):
        resource = store("oci:test", workspace="workspace:test")
        value = {"target": {"detached": True, "environment": "", "instance": "", "runtime": "nerdctl",
                            "store": acceptance.ref(resource)},
                 "images": [{"id": "sha256:" + "1" * 64, "tags": ["example.invalid/test:local"], "containers": []}]}
        self.assertEqual(len(acceptance.inventory(value, resource)), 1)
        for case in ("owner", "host", "environment", "runtime", "id", "duplicate", "containers"):
            with self.subTest(case=case):
                changed = copy.deepcopy(value)
                if case == "owner": changed["target"]["store"]["owner"] = "b" * 32
                if case == "host": changed["target"]["host"] = True
                if case == "environment": changed["target"]["environment"] = "replacement"
                if case == "runtime": changed["target"]["runtime"] = "docker"
                if case == "id": changed["images"][0]["id"] = "sha256:short"
                if case == "duplicate": changed["images"] *= 2
                if case == "containers": changed["images"][0]["containers"] = ["stopped-container"]
                with self.assertRaises(RuntimeError):
                    acceptance.inventory(changed, resource)

    def test_store_rejects_unfinished_and_source_confusion(self):
        for field, value in (("owner", "unknown"), ("native_ref", ""), ("state", "creating"),
                             ("copy_source", {"id": "oci:other"}), ("copy_operation", "pending"),
                             ("import_pending", True), ("source_only", True)):
            with self.subTest(field=field):
                row = store("oci:test")
                row[field] = value
                with self.assertRaises(RuntimeError):
                    acceptance.ready_store(row)

    def test_runner_guard_precedes_every_filesystem_and_product_action(self):
        for env, uid in (({}, 1000), ({"GITHUB_ACTIONS": "true"}, 1000),
                         ({"GITHUB_ACTIONS": "true", "HACO_CI_RUNNER_ENVIRONMENT": "self-hosted"}, 1000),
                         ({"GITHUB_ACTIONS": "true", "HACO_CI_RUNNER_ENVIRONMENT": "github-hosted"}, 0)):
            with mock.patch.dict(os.environ, env, clear=True), mock.patch.object(os, "geteuid", return_value=uid), \
                    mock.patch.object(acceptance.tempfile, "mkdtemp") as create, \
                    mock.patch.object(acceptance.subprocess, "run") as run, self.assertRaises(RuntimeError):
                acceptance.main()
            create.assert_not_called()
            run.assert_not_called()

    def test_runner_refuses_controller_overrides_before_privileged_observation(self):
        for key in ("HACO_ROOT", "HACO_CONTROL_SOCKET", "HACO_CLIENT_MODE"):
            env = {"GITHUB_ACTIONS": "true", "HACO_CI_RUNNER_ENVIRONMENT": "github-hosted", key: "other"}
            with mock.patch.dict(os.environ, env, clear=True), mock.patch.object(os, "geteuid", return_value=1000), \
                    mock.patch.object(acceptance.subprocess, "run") as run, \
                    mock.patch.object(acceptance.tempfile, "mkdtemp") as create, \
                    self.assertRaisesRegex(RuntimeError, "default Physical Host controller"):
                acceptance.main()
            run.assert_not_called()
            create.assert_not_called()

    def test_command_does_not_log_raw_output_or_inherit_stdin(self):
        with mock.patch.object(acceptance.subprocess, "run", return_value=subprocess.CompletedProcess(
                [], 1, "private output", "private stderr")) as run, self.assertRaisesRegex(RuntimeError, "exit 1") as raised:
            acceptance.command("haco", "setup")
        self.assertNotIn("private", str(raised.exception))
        self.assertEqual(run.call_args.kwargs["stdin"], subprocess.DEVNULL)
        self.assertTrue(run.call_args.kwargs["capture_output"])


class RecipeObserverTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.parent = self.root / "var/lib/hacocoon/host-customization"
        self.parent.mkdir(parents=True, mode=0o700)
        self.parent.parent.chmod(0o700)
        self.file = self.parent / "recipe.sh"

    def observe(self, expected):
        original_open, original_fstat, original_stat = os.open, os.fstat, os.stat
        def open_at(path, flags, *args, **kwargs):
            self.assertTrue(flags & os.O_NOFOLLOW)
            if path == "/": path = str(self.root)
            return original_open(path, flags, *args, **kwargs)
        def root_owned(info):
            values = {key: getattr(info, key) for key in dir(info) if key.startswith("st_")}
            values["st_uid"] = 0  # Pure local fixture; no privilege required.
            return types.SimpleNamespace(**values)
        output = io.StringIO()
        with mock.patch.object(os, "open", side_effect=open_at), \
                mock.patch.object(os, "fstat", side_effect=lambda fd: root_owned(original_fstat(fd))), \
                mock.patch.object(os, "stat", side_effect=lambda *a, **k: root_owned(original_stat(*a, **k))), \
                mock.patch.object(sys, "argv", ["observer", expected]), contextlib.redirect_stdout(output):
            try:
                exec(acceptance.RECIPE_OBSERVER, {})
            except SystemExit as result:
                if result.code != 0:
                    raise
        return json.loads(output.getvalue())

    def seed(self):
        self.file.write_text("fixture-only\n")
        self.file.chmod(0o600)
        return hashlib.sha256(self.file.read_bytes()).hexdigest()

    def test_absence_does_not_read_existing_file(self):
        self.assertTrue(self.observe("absent")["absent"])
        self.seed()
        with mock.patch.object(os, "read") as read, self.assertRaisesRegex(SystemExit, "existing Host recipe"):
            self.observe("absent")
        read.assert_not_called()

    def test_hash_identity_only_and_replaced_identical_bytes_differ(self):
        digest = self.seed()
        before = self.observe(digest)
        self.assertEqual(before["sha256"], digest)
        self.assertNotIn("fixture-only", json.dumps(before))
        replacement = self.parent / "replacement"
        replacement.write_bytes(self.file.read_bytes())
        replacement.chmod(0o600)
        replacement.replace(self.file)
        self.assertNotEqual(self.observe(digest), before)
        with self.assertRaisesRegex(SystemExit, "recipe changed"):
            self.observe("0" * 64)

    def test_refuses_symlink_hardlink_public_file_and_linked_parent(self):
        digest = self.seed()
        other = self.parent / "other"
        os.link(self.file, other)
        with self.assertRaisesRegex(SystemExit, "unsafe recipe file"):
            self.observe(digest)
        other.unlink()
        self.file.chmod(0o644)
        with self.assertRaisesRegex(SystemExit, "unsafe recipe file"):
            self.observe(digest)
        self.file.unlink()
        self.file.symlink_to("/missing")
        with self.assertRaises(OSError):
            self.observe("absent")
        self.file.unlink()
        self.parent.rmdir()
        self.parent.symlink_to(self.root)
        with self.assertRaises(OSError):
            self.observe("absent")

    def test_refuses_nonregular_and_oversize_without_reading(self):
        os.mkfifo(self.file, 0o600)
        with mock.patch.object(os, "read") as read, self.assertRaisesRegex(SystemExit, "unsafe recipe file"):
            self.observe("0" * 64)
        read.assert_not_called()
        self.file.unlink()
        self.file.write_bytes(b"x" * 1048577)
        self.file.chmod(0o600)
        with mock.patch.object(os, "read") as read, self.assertRaisesRegex(SystemExit, "unsafe recipe file"):
            self.observe("0" * 64)
        read.assert_not_called()


if __name__ == "__main__":
    unittest.main()

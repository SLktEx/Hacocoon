"""Fixture/cleanup regressions; these do not claim installed Incus acceptance."""
import importlib.util
import base64
import contextlib
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock


spec = importlib.util.spec_from_file_location(
    "installed_workspace_input",
    Path(__file__).resolve().parents[1] / "test/e2e/installed/workspace_input.py")
acceptance = importlib.util.module_from_spec(spec)
spec.loader.exec_module(acceptance)


class FixtureTests(unittest.TestCase):
    def test_runner_guard_requires_both_github_actions_and_hosted_markers(self):
        for environ in ({}, {"GITHUB_ACTIONS": "true"},
                        {"HACO_CI_RUNNER_ENVIRONMENT": "github-hosted"},
                        {"GITHUB_ACTIONS": "false", "HACO_CI_RUNNER_ENVIRONMENT": "github-hosted"},
                        {"GITHUB_ACTIONS": "true", "HACO_CI_RUNNER_ENVIRONMENT": "self-hosted"}):
            with self.subTest(environ=environ), self.assertRaisesRegex(RuntimeError, "disposable GitHub-hosted"):
                acceptance.require_disposable_runner(environ)
        acceptance.require_disposable_runner({"GITHUB_ACTIONS": "true",
                                              "HACO_CI_RUNNER_ENVIRONMENT": "github-hosted"})

    def test_main_refuses_outside_runner_before_any_fixture_or_product_mutation(self):
        with mock.patch.dict(acceptance.os.environ, {}, clear=True), \
                mock.patch.object(acceptance.tempfile, "mkdtemp") as create_fixture, \
                mock.patch.object(acceptance.subprocess, "run") as product_command:
            with self.assertRaisesRegex(RuntimeError, "disposable GitHub-hosted"):
                acceptance.main()
            create_fixture.assert_not_called()
            product_command.assert_not_called()

    def test_real_linked_worktree_distinguishes_head_index_and_working_files(self):
        with tempfile.TemporaryDirectory() as directory:
            main, linked, head = acceptance.fixture(Path(directory))
            before = acceptance.tree_digest(main), acceptance.tree_digest(linked)
            self.assertTrue((linked / ".git").is_file())
            self.assertEqual(acceptance.git(linked, "rev-parse", "HEAD"), head)
            self.assertNotEqual(acceptance.git(main, "rev-parse", "HEAD"), head)
            self.assertEqual(acceptance.git(linked, "show", ":tracked"), "staged")
            self.assertEqual((linked / "tracked").read_text(), "dirty\n")
            self.assertEqual((main / "tracked").read_text(), "initial\n")
            self.assertFalse((main / "selected").exists())
            self.assertFalse((linked / "other-only").exists())
            self.assertEqual((acceptance.tree_digest(main), acceptance.tree_digest(linked)), before)
            (main / ".git/config").write_text("changed")
            self.assertNotEqual(acceptance.tree_digest(main), before[0])

    def test_source_snapshot_refuses_symlinks(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "outside").symlink_to("/etc/passwd")
            with self.assertRaisesRegex(RuntimeError, "unexpected source"):
                acceptance.tree_digest(root)


class CleanupTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.fixture = acceptance.Acceptance(Path(self.directory.name))
        self.commands = []
        self.fixture.haco = lambda *args: self.commands.append(args)

    def test_uncertain_creation_is_never_adopted_by_name(self):
        self.fixture.query = lambda *args: self.fail("no confirmed ownership to inspect")
        self.fixture.cleanup()
        self.assertEqual(self.commands, [])

    def test_any_existing_source_is_refused_before_registration(self):
        # These are possible public rows, including deliberately unexpected
        # flags. The actual public API currently hides excluded sources; only
        # the fresh-installed CI context establishes their absence.
        for remote in (acceptance.UPSTREAM, acceptance.UPSTREAM.upper() + "/",
                       "git@github.com:SLktEx/Hacocoon.git",
                       "ssh://git@github.com/SLktEx/Hacocoon.git",
                       "https://github.com/example/unrelated.git"):
            for flags in ({"state": "ready"}, {"state": "creating"},
                          {"state": "ready", "excluded": True}):
                with self.subTest(remote=remote, flags=flags), tempfile.TemporaryDirectory() as directory:
                    fixture = acceptance.Acceptance(Path(directory))
                    fixture.haco = lambda *args: self.fail("pre-existing source must not be mutated")

                    def query(*args):
                        self.assertEqual(args, ("repo", "list", "--json"))
                        return {"sources": [{"source": {"id": "unrelated", "remote": remote, **flags}}]}

                    fixture.query = query
                    with self.assertRaisesRegex(RuntimeError, "fresh installation"):
                        fixture.exercise()
                    self.assertIsNone(fixture.source)

    def test_stale_reference_is_restored_after_refusal_or_unexpected_success(self):
        path = Path(self.directory.name)
        reference = path / ".haco-workspace.json"
        original = b'{"name":"owned","workspace":"original"}\n'
        for refused in (True, False):
            with self.subTest(refused=refused):
                reference.write_bytes(original)

                def open_path(*args):
                    self.assertEqual(json.loads(reference.read_bytes())["workspace"], "other")
                    if refused:
                        raise subprocess.CalledProcessError(1, args)
                    return "{}"

                self.fixture.haco = open_path
                if refused:
                    self.fixture.check_stale_reference(path, "other")
                else:
                    with self.assertRaisesRegex(RuntimeError, "was accepted"):
                        self.fixture.check_stale_reference(path, "other")
                self.assertEqual(reference.read_bytes(), original)

    def test_unexpected_registration_receipt_is_retained_but_never_owned_or_unregistered(self):
        receipt = {"id": "pre-existing-source", "state": "ready", "owner": "a" * 32}
        answers = iter([{"sources": []}, receipt])
        self.fixture.query = lambda *args: next(answers)
        with self.assertRaisesRegex(RuntimeError, "source receipt mismatch"):
            self.fixture.exercise()
        self.assertIsNone(self.fixture.source)
        saved = json.loads((self.fixture.root / "owned.json").read_text())
        self.assertEqual(saved["registration_receipt"], receipt)
        self.assertEqual(saved["requested_source"], self.fixture.prefix)
        self.assertIsNone(saved["source"])
        self.fixture.query = lambda *args: self.fail("unexpected receipt grants no cleanup ownership")
        self.fixture.cleanup()
        self.assertEqual(self.commands, [])

    def test_changed_environment_generation_is_not_deleted(self):
        original = {"name": "owned", "created_at": "original", "runtime_ref": "haco-owned",
                    "workspace": {"id": "workspace:managed:original"}}
        self.fixture.environments = [original]
        for key, value in (("created_at", "replacement"), ("runtime_ref", "other"),
                           ("workspace", {"id": "workspace:managed:replacement"})):
            with self.subTest(key=key):
                changed = dict(original, **{key: value})
                self.fixture.query = lambda *args: {"environment": changed}
                with self.assertRaisesRegex(RuntimeError, "cleanup incomplete"):
                    self.fixture.cleanup()
                self.assertEqual(self.commands, [])

    def test_changed_workspace_owner_is_not_deleted(self):
        self.fixture.workspaces = [{"name": "owned", "workspace": "workspace:managed:original"}]
        self.fixture.query = lambda *args: [{"name": "owned", "workspace": {"id": "replacement"}}]
        with self.assertRaisesRegex(RuntimeError, "cleanup incomplete"):
            self.fixture.cleanup()
        self.assertEqual(self.commands, [])

    def test_changed_registered_source_owner_is_not_unregistered(self):
        self.fixture.source = {"id": "owned", "owner": "original"}
        self.fixture.query = lambda *args: {"sources": [{"source": {"id": "owned", "owner": "replacement"}}]}
        with self.assertRaisesRegex(RuntimeError, "cleanup incomplete"):
            self.fixture.cleanup()
        self.assertEqual(self.commands, [])

    def test_owned_env_and_workspaces_are_deleted_then_source_is_unregistered(self):
        env = {"name": "owned", "created_at": "original", "runtime_ref": "haco-owned",
               "workspace": {"id": "workspace:managed:original"}}
        self.fixture.environments = [env]
        self.fixture.workspaces = [{"name": "owned", "workspace": env["workspace"]["id"]}]
        self.fixture.source = {"id": "owned", "owner": "original"}
        answers = iter([
            {"environment": env}, [{"name": "unrelated"}],
            [{"name": "owned", "workspace": env["workspace"]}, {"name": "unrelated"}],
            [{"name": "unrelated"}],
            {"sources": [{"source": self.fixture.source}, {"source": {"id": "unrelated"}}]},
            {"sources": [{"source": {"id": "unrelated"}}]},
        ])
        self.fixture.query = lambda *args: next(answers)
        self.fixture.cleanup()
        self.assertEqual(self.commands, [
            ("env", "delete", "--force", "owned"),
            ("workspace", "delete", "--yes", "owned"),
            # Product syntax says delete; its current contract is unregistration.
            ("repo", "delete", "owned"),
        ])

    def test_source_unregistration_accepts_hidden_registration_with_native_data_retained(self):
        # Model the current public API contract: UnregisterSource sets Excluded,
        # and ListSources hides it. No physical source-deletion proof is implied.
        source = {"id": "owned", "owner": "original", "excluded": False,
                  "native_ref": "pool/retained-source"}
        self.fixture.source = dict(source)

        def query(*args):
            self.assertEqual(args, ("repo", "list", "--json"))
            return {"sources": [] if source["excluded"] else [{"source": source}]}

        def unregister(*args):
            self.assertEqual(args, ("repo", "delete", "owned"))
            source["excluded"] = True

        self.fixture.query = query
        self.fixture.haco = unregister
        self.fixture.cleanup()
        self.assertTrue(source["excluded"])
        self.assertEqual(source["native_ref"], "pool/retained-source")

    def test_failed_environment_cleanup_still_attempts_independent_owned_resources(self):
        env = {"name": "owned", "created_at": "original", "runtime_ref": "haco-owned",
               "workspace": {"id": "workspace:managed:original"}}
        self.fixture.environments = [env]
        self.fixture.workspaces = [{"name": "other", "workspace": "workspace:managed:other"}]
        answers = iter([{"environment": dict(env, created_at="replacement")},
                        [{"name": "other", "workspace": {"id": "workspace:managed:other"}}], []])
        self.fixture.query = lambda *args: next(answers)
        with self.assertRaisesRegex(RuntimeError, "cleanup incomplete"):
            self.fixture.cleanup()
        self.assertEqual(self.commands, [("workspace", "delete", "--yes", "other")])


class ReceiptTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.fixture = acceptance.Acceptance(Path(self.directory.name))
        self.fixture.source = {"id": self.fixture.prefix, "owner": "a" * 32, "state": "ready"}
        self.name = self.fixture.prefix + "-selected"
        self.workspace = "workspace:managed:" + "b" * 32
        self.ref = {"name": self.name, "state": "ready", "workspace": self.workspace}
        # Production composition wraps Incus with internal/env.Router; its
        # encodeRouteRef uses the provider ID and unpadded URL-safe base64.
        native = "haco-work-" + self.name
        routed = "haco-runtime-v1:runtime.incus:" + base64.urlsafe_b64encode(native.encode()).decode().rstrip("=")
        self.env = {"name": "work-" + self.name, "runtime_ref": routed,
                    "created_at": "2026-10-09T12:00:00Z", "workspace": {"id": self.workspace}}

    def saved(self):
        return json.loads((self.fixture.root / "owned.json").read_text())

    def test_successful_import_and_reopen_preserve_installed_routed_runtime_identity(self):
        opened = {"name": self.name, "workspace": self.workspace, "created": True, "environment": self.env}
        responses = iter([self.ref, opened, dict(opened, created=False)])
        self.fixture.query = lambda *args: next(responses)
        path, env = self.fixture.import_open("selected", self.fixture.root / "input")
        self.assertEqual(env, self.env)
        self.fixture.check_reopen(path, env)
        saved = self.saved()
        self.assertEqual(saved["environments"], [self.env])
        self.assertEqual(saved["open_receipts"][0]["receipt"]["environment"]["runtime_ref"], self.env["runtime_ref"])

    def test_cleanup_refuses_changed_routed_runtime_identity(self):
        self.fixture.source = None
        self.fixture.environments = [self.env]
        other = "haco-runtime-v1:runtime.incus:" + base64.urlsafe_b64encode(b"haco-another").decode().rstrip("=")
        self.fixture.query = lambda *args: {"environment": dict(self.env, runtime_ref=other)}
        self.fixture.haco = lambda *args: self.fail("changed routed identity must not be deleted")
        with self.assertRaisesRegex(RuntimeError, "cleanup incomplete"):
            self.fixture.cleanup()

    def test_mismatched_import_is_saved_before_validation_and_never_owned(self):
        observed = dict(self.ref, name="unexpected-work", private_key="must-not-retain")
        self.fixture.query = lambda *args: observed
        with self.assertRaisesRegex(RuntimeError, "import receipt mismatch"):
            self.fixture.import_open("selected", self.fixture.root / "input")
        saved = self.saved()
        self.assertEqual(saved["import_receipts"][0]["receipt"], acceptance.public_receipt(observed))
        self.assertEqual(saved["workspaces"], [])
        self.assertEqual(saved["environments"], [])
        self.assertEqual(saved["open_receipts"], [])
        self.assertNotIn("must-not-retain", json.dumps(saved))

    def test_mismatched_open_is_saved_without_owning_the_unconfirmed_environment(self):
        observed = dict(self.env, workspace={"id": "workspace:managed:" + "c" * 32})
        answer = {"name": self.name, "workspace": self.workspace, "created": True,
                  "environment": observed, "credentials": "must-not-retain"}
        responses = iter([self.ref, answer])
        self.fixture.query = lambda *args: next(responses)
        with self.assertRaisesRegex(RuntimeError, "Workspace/Environment mismatch"):
            self.fixture.import_open("selected", self.fixture.root / "input")
        saved = self.saved()
        self.assertEqual(saved["workspaces"], [self.ref])
        self.assertEqual(saved["environments"], [])
        self.assertEqual(saved["open_receipts"][0]["receipt"]["environment"], observed)
        self.assertNotIn("must-not-retain", self.fixture.failure_diagnostic())

    def test_nonzero_import_retains_only_public_recovery_identity(self):
        receipt = dict(self.ref, state="recovery-required", config={"token": "must-not-retain"})

        def failed(*args):
            raise subprocess.CalledProcessError(1, ["haco", "workspace", "import"], output=json.dumps(receipt))

        self.fixture.query = failed
        with self.assertRaises(subprocess.CalledProcessError):
            self.fixture.import_open("selected", self.fixture.root / "input")
        saved = self.saved()
        self.assertEqual(saved["import_receipts"][0], {
            "requested_name": self.name, "result": "failed", "exit_code": 1,
            "receipt": acceptance.public_receipt(receipt)})
        self.assertEqual(saved["workspaces"], [])
        self.assertEqual(saved["environments"], [])
        self.assertNotIn("must-not-retain", self.fixture.failure_diagnostic())

    def test_failed_open_without_json_stays_unconfirmed(self):
        def query(*args):
            if args[0] == "workspace":
                return self.ref
            raise subprocess.CalledProcessError(1, ["haco", "open"], output="")

        self.fixture.query = query
        with self.assertRaises(subprocess.CalledProcessError):
            self.fixture.import_open("selected", self.fixture.root / "input")
        saved = self.saved()
        self.assertEqual(saved["open_receipts"][0]["result"], "failed")
        self.assertEqual(saved["open_receipts"][0]["receipt"], {})
        self.assertEqual(saved["environments"], [])
        self.assertEqual(saved["workspaces"], [self.ref])

    def test_failure_stdout_and_diagnostic_are_bounded(self):
        for output in ("not-json", "x" * (acceptance.REPORT_LIMIT + 1),
                       json.dumps({"name": "secret\nvalue", "state": "secret", "authorization": "secret"})):
            self.assertEqual(acceptance.failed_receipt(output), {})
        self.fixture.open_receipts = [{"receipt": {"name": "example"}}] * 1000
        report = self.fixture.failure_diagnostic()
        self.assertLessEqual(len(report), acceptance.REPORT_LIMIT)
        self.assertTrue(json.loads(report)["report_truncated"])

    def test_main_emits_public_failure_report_into_existing_ci_logs(self):
        def failed(fixture):
            fixture.registration_receipt = acceptance.public_receipt({
                "id": "unexpected", "owner": "a" * 32, "password": "must-not-retain"})
            raise RuntimeError("fixture failure")

        output = io.StringIO()
        with mock.patch.dict(acceptance.os.environ, {
                "GITHUB_ACTIONS": "true", "HACO_CI_RUNNER_ENVIRONMENT": "github-hosted"}), \
                mock.patch.object(acceptance.os, "geteuid", return_value=1000), \
                mock.patch.object(acceptance.tempfile, "mkdtemp", return_value=self.directory.name), \
                mock.patch.object(acceptance.Acceptance, "exercise", autospec=True, side_effect=failed), \
                contextlib.redirect_stdout(output):
            with self.assertRaisesRegex(RuntimeError, "fixture failure"):
                acceptance.main()
        report = next(line.removeprefix("INSTALLED_INPUT_FAILURE ") for line in output.getvalue().splitlines()
                      if line.startswith("INSTALLED_INPUT_FAILURE "))
        self.assertEqual(json.loads(report)["status"], "failed")
        self.assertNotIn("must-not-retain", output.getvalue())
        self.assertLessEqual(len(report), acceptance.REPORT_LIMIT)


if __name__ == "__main__":
    unittest.main()

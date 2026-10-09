"""Fixture/cleanup regressions; these do not claim installed Incus acceptance."""
import importlib.util
import base64
import contextlib
import copy
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


class PackagePrerequisiteTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.fixture = acceptance.Acceptance(Path(self.directory.name))
        self.environment = self.env("selected")
        self.original = {"default": "deny", "rules": [
            {"capability": "network.egress", "action": "connect", "resource": "existing.example",
             "environment": "unrelated", "decision": "deny", "attributes": {"protocol": "https", "port": "443"}}],
            "saved_decisions": [], "network_services": []}
        self.policy = copy.deepcopy(self.original)
        self.revision = 1
        self.writes = []
        self.commands = []
        self.installed = set()
        self.fail_save_after_write = False
        self.fail_package = False
        self.fixture.query = self.query
        self.fixture.haco = self.haco

    def env(self, label):
        return {"name": "work-" + self.fixture.prefix + "-" + label,
                "created_at": "2026-10-09T00:00:00Z", "runtime_ref": "haco-exact-" + label,
                "workspace": {"id": "workspace:managed:" + ("a" if label == "selected" else "b") * 32}}

    def snapshot(self):
        return {"revision": "sha256:" + format(self.revision, "064x"), "policy": copy.deepcopy(self.policy)}

    def query(self, *args):
        if args[:3] == ("env", "status", "--json"):
            label = args[3].rsplit("-", 1)[1]
            return {"environment": self.env(label), "state": "stopped"}
        if args == ("config", "--json"):
            return self.snapshot()
        if args[:2] == ("config", "--file"):
            path = Path(args[2])
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            edit = json.loads(path.read_text())
            self.assertEqual(edit["revision"], self.snapshot()["revision"])
            self.policy = edit["policy"]
            self.writes.append(copy.deepcopy(self.policy))
            self.revision += 1
            if self.fail_save_after_write:
                self.fail_save_after_write = False
                raise subprocess.CalledProcessError(1, args)
            return self.snapshot()
        if args == ("repo", "list", "--json"):
            return {"sources": []}
        if args[:2] == ("repo", "add"):
            return {"id": self.fixture.prefix, "owner": "c" * 32, "state": "ready"}
        self.fail("unexpected product query: " + repr(args))

    def haco(self, *args):
        self.commands.append(args)
        if args[0] == "exec":
            environment = args[1]
            if "apt-get install -y --no-install-recommends git" in args[-1]:
                grants = self.policy["rules"][1:]
                self.assertEqual(len(grants), 4)
                self.assertTrue(all(rule["environment"] == environment for rule in grants))
                if self.fail_package:
                    raise subprocess.CalledProcessError(100, args)
                self.installed.add(environment)
            elif environment not in self.installed:
                raise subprocess.CalledProcessError(127, args, stderr="git: not found")
        return "new-local-commit"

    def test_fixture_prepares_both_gitless_environments_before_git_assertions(self):
        self.fixture.import_open = lambda label, source: (self.fixture.root / label, self.env(label))
        self.fixture.check_stale_reference = mock.Mock()
        self.fixture.check_reopen = mock.Mock()
        self.fixture.check_inputs = mock.Mock()
        self.fixture.exercise()
        self.assertEqual(self.installed, {self.env(label)["name"] for label in ("other", "selected")})
        self.assertEqual(self.policy, self.original)
        self.assertEqual(self.fixture.package_rules, [])
        self.assertEqual(len(self.writes), 4)
        self.assertTrue(any("git commit" in repr(command) or "commit -m imported-index" in repr(command)
                            for command in self.commands))

    def test_package_grants_are_exact_temporary_and_preserve_existing_policy(self):
        self.fixture.prepare_git(self.environment)
        granted = self.writes[0]["rules"][1:]
        self.assertEqual({(rule["resource"], rule["attributes"]["protocol"], rule["attributes"]["port"])
                          for rule in granted},
                         {(host, protocol, port) for host in ("archive.ubuntu.com", "security.ubuntu.com")
                          for protocol, port in (("http", "80"), ("https", "443"))})
        for rule in granted:
            self.assertEqual((rule["capability"], rule["action"], rule["decision"]),
                             ("network.egress", "connect", "allow"))
            self.assertEqual(rule["environment"], self.environment["name"])
            self.assertNotIn("*", json.dumps(rule))
            expires = acceptance.datetime.fromisoformat(rule["expires_at"])
            remaining = (expires - acceptance.datetime.now(acceptance.timezone.utc)).total_seconds()
            self.assertGreater(remaining, 890)
            self.assertLessEqual(remaining, 900)
        self.assertEqual(self.writes[-1], self.original)
        self.assertEqual(self.policy, self.original)

    def test_package_failure_removes_grants_without_retrying_guest_command(self):
        self.fail_package = True
        with self.assertRaises(subprocess.CalledProcessError):
            self.fixture.prepare_git(self.environment)
        self.assertEqual(self.policy, self.original)
        self.assertEqual(len(self.commands), 1)
        self.assertEqual(self.fixture.package_rules, [])

    def test_guest_package_script_installs_only_missing_git(self):
        for present in (False, True):
            with self.subTest(present=present), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                binary = root / "git"
                log = root / "packages.log"
                apt = root / "apt-get"
                apt.write_text('''#!/bin/sh
set -eu
printf '%s\\n' "$*" >> "$PACKAGE_LOG"
case "$*" in
  update) ;;
  'install -y --no-install-recommends git')
    printf '#!/bin/sh\\nprintf "git version fixture\\\\n"\\n' > "$GIT_BINARY"
    /bin/chmod 0700 "$GIT_BINARY"
    ;;
  *) exit 93 ;;
esac
''')
                apt.chmod(0o700)
                if present:
                    binary.write_text('#!/bin/sh\nprintf "git version retained\\n"\n')
                    binary.chmod(0o700)
                result = subprocess.run(["/bin/sh", "-ceu", acceptance.GIT_PREREQUISITE],
                                        env={"PATH": directory, "PACKAGE_LOG": str(log), "GIT_BINARY": str(binary)},
                                        text=True, capture_output=True, check=True)
                if present:
                    self.assertFalse(log.exists())
                    self.assertIn("retained", result.stdout)
                else:
                    self.assertEqual(log.read_text().splitlines(), ["update", "install -y --no-install-recommends git"])
                    self.assertIn("fixture", result.stdout)

    def test_uncertain_saved_grant_is_inspected_and_removed_without_package_execution(self):
        self.fail_save_after_write = True
        with self.assertRaises(subprocess.CalledProcessError):
            self.fixture.prepare_git(self.environment)
        self.assertEqual(self.policy, self.original)
        self.assertEqual(self.commands, [])
        self.assertEqual(len(self.writes), 2)

    def test_stale_policy_save_is_not_replayed_over_a_concurrent_change(self):
        query = self.fixture.query
        attempts = []
        concurrent = {"capability": "local.echo", "action": "echo", "resource": "new", "decision": "deny"}

        def conflict(*args):
            if args[:2] == ("config", "--file"):
                attempts.append(args)
                self.policy["rules"].append(concurrent)
                self.revision += 1
                raise subprocess.CalledProcessError(1, args)
            return query(*args)

        self.fixture.query = conflict
        with self.assertRaisesRegex(RuntimeError, "add is still unconfirmed"):
            self.fixture.prepare_git(self.environment)
        self.assertEqual(len(attempts), 1)
        self.assertEqual(self.policy["rules"], self.original["rules"] + [concurrent])
        self.assertEqual(self.commands, [])
        self.assertTrue(self.fixture.package_rules)
        self.assertFalse(self.fixture.package_add_observed)

    def test_timed_out_add_is_not_completed_by_an_early_absent_snapshot(self):
        query = self.fixture.query
        delayed = []

        def delayed_save(*args):
            if args[:2] == ("config", "--file"):
                delayed.append(json.loads(Path(args[2]).read_text()))
                raise subprocess.TimeoutExpired(args, 300)
            return query(*args)

        self.fixture.environments = [self.environment]
        self.fixture.query = delayed_save
        with self.assertRaisesRegex(RuntimeError, "add is still unconfirmed"):
            self.fixture.prepare_git(self.environment)
        self.assertEqual(len(delayed), 1)
        self.assertEqual(self.policy, self.original)
        self.assertTrue(self.fixture.package_rules)
        self.assertFalse(self.fixture.package_add_observed)
        with self.assertRaisesRegex(RuntimeError, "retained package Policy"):
            self.fixture.cleanup()
        self.assertEqual(self.commands, [])

        # Complete the original request only after cleanup observed absence.
        # Positive observation now permits an exact removal, with no add retry.
        self.policy = delayed[0]["policy"]
        self.revision += 1
        self.fixture.query = query
        self.fixture.remove_package_policy()
        self.assertEqual(self.policy, self.original)
        self.assertEqual(self.fixture.package_rules, [])
        self.assertEqual(self.commands, [])
        self.assertEqual(len(self.writes), 1)

    def test_uncertain_removal_preserves_add_observation_and_is_not_replayed(self):
        for committed in (False, True):
            with self.subTest(committed=committed):
                self.policy = copy.deepcopy(self.original)
                self.commands = []
                self.writes = []
                query = self.query
                removals = []

                def lose_removal_response(*args):
                    if args[:2] == ("config", "--file") and self.writes:
                        removals.append(json.loads(Path(args[2]).read_text()))
                        if committed:
                            query(*args)
                        raise subprocess.TimeoutExpired(args, 300)
                    return query(*args)

                self.fixture.query = lose_removal_response
                with self.assertRaises(subprocess.TimeoutExpired):
                    self.fixture.prepare_git(self.environment)
                self.assertTrue(self.fixture.package_add_observed)
                self.assertTrue(self.fixture.package_remove_pending)
                if not committed:
                    with self.assertRaisesRegex(RuntimeError, "removal is still unconfirmed"):
                        self.fixture.remove_package_policy()
                    self.assertEqual(len(removals), 1)
                    self.policy = removals[0]["policy"]
                    self.revision += 1
                self.fixture.remove_package_policy()
                self.assertEqual(self.policy, self.original)
                self.assertEqual(self.fixture.package_rules, [])
                self.assertFalse(self.fixture.package_remove_pending)
                self.assertEqual(len(removals), 1)

    def test_replaced_environment_receipt_never_receives_package_permission(self):
        replaced = dict(self.environment, created_at="different creation")
        with self.assertRaisesRegex(RuntimeError, "replaced Environment"):
            self.fixture.prepare_git(replaced)
        self.assertEqual(self.writes, [])
        self.assertEqual(self.commands, [])

    def test_replacement_during_policy_save_is_refused_before_package_execution(self):
        query = self.fixture.query

        def replace_after_save(*args):
            result = query(*args)
            if args[:3] == ("env", "status", "--json") and self.writes:
                result["environment"]["created_at"] = "replacement"
            return result

        self.fixture.query = replace_after_save
        with self.assertRaisesRegex(RuntimeError, "replaced Environment"):
            self.fixture.prepare_git(self.environment)
        self.assertEqual(self.policy, self.original)
        self.assertEqual(self.commands, [])

    def test_uncertain_package_policy_cleanup_does_not_free_environment_names(self):
        self.fixture.environments = [self.environment]
        self.fixture.remove_package_policy = mock.Mock(side_effect=RuntimeError("ambiguous grants"))
        with self.assertRaisesRegex(RuntimeError, "retained package Policy"):
            self.fixture.cleanup()
        self.assertEqual(self.commands, [])

    def test_cleanup_preserves_concurrent_rules_and_refuses_changed_or_duplicate_grants(self):
        self.fixture.prepare_git(self.environment)
        exact = self.writes[0]["rules"][1:]
        unrelated = {"capability": "local.echo", "action": "echo", "resource": "new", "decision": "deny"}
        self.policy["rules"] += exact + [unrelated]
        self.fixture.package_rules = copy.deepcopy(exact)
        self.fixture.remove_package_policy()
        self.assertEqual(self.policy["rules"], self.original["rules"] + [unrelated])
        for altered in ([dict(exact[0], resource="changed.example")], [exact[0], exact[0]]):
            with self.subTest(altered=altered):
                self.policy["rules"] = self.original["rules"] + altered
                self.fixture.package_rules = copy.deepcopy(exact)
                before = copy.deepcopy(self.policy)
                writes = len(self.writes)
                with self.assertRaises(RuntimeError):
                    self.fixture.remove_package_policy()
                self.assertEqual(self.policy, before)
                self.assertEqual(len(self.writes), writes)


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

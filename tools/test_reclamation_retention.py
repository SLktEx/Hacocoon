import json
import contextlib
import io
import subprocess
import sys
import tempfile
from pathlib import Path
from unittest.mock import patch
import unittest
from unittest.mock import Mock
import reclamation_retention as subject
import reclamation_diagnostics

RECORD = {"version": 2, "nonce": "1" * 16, "workspace": "import-" + "2" * 16,
          "oci": "oci:import-" + "3" * 16, "snapshot": "snap-" + "4" * 32, "commit": "5" * 40,
          "base": "win-base-" + "6" * 16, "base_revision": "sha256:" + "7" * 64}
BASE = {"name": RECORD["base"], "revision": RECORD["base_revision"]}
SAVED = {"id": RECORD["snapshot"], "state": "ready", "oci": True, "workspaces": 1}

class RetentionTests(unittest.TestCase):
    def host(self, *args):
        self.calls.append(args)
        if args[:2] == ("base", "inspect"):
            return json.dumps(BASE)
        if args[:2] == ("snapshot", "list"):
            return json.dumps([SAVED])
        if args[:2] == ("snapshot", "restore"):
            return json.dumps({"environment": args[-1], "state": "running", "workspace": "independent", "oci": "independent-oci"})
        if args[:2] == ("env", "create"):
            return json.dumps({"name": args[-1], "base": BASE})
        return ""

    def setUp(self):
        self.calls = []

    def test_reattachment_and_saved_data_are_separate_and_never_deleted(self):
        guest = Mock()
        subject.verify(RECORD, self.host, guest)
        self.assertEqual(guest.call_count, 2)
        self.assertIn(RECORD["commit"], guest.call_args_list[0].args[1])
        self.assertIn('rootfs-kept', guest.call_args_list[0].args[1])
        self.assertIn('test ! -e /root/transfer-marker', guest.call_args_list[1].args[1])
        self.assertTrue(any(args[:2] == ("env", "create") and RECORD["oci"] in args for args in self.calls))
        create = next(args for args in self.calls if args[:2] == ("env", "create"))
        self.assertEqual(create[create.index("--base") + 1], RECORD["base"])
        self.assertFalse(any("delete" in args for args in self.calls))
        self.assertEqual(sum(args[:2] == ("snapshot", "restore") for args in self.calls), 1)

    def test_explicit_manifest_does_not_require_product_environment_overrides(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory, "fixture.json")
            path.write_text(json.dumps(RECORD), encoding="utf-8")
            with patch.dict("os.environ", {}, clear=True):
                self.assertEqual(subject.load_manifest(path), RECORD)
            path.write_bytes(b" " * 4097)
            with self.assertRaisesRegex(RuntimeError, "oversized"):
                subject.load_manifest(path)

    def test_untrusted_manifest_refused_before_commands(self):
        for key, value in [("nonce", "../foreign"), ("oci", "oci:existing"), ("commit", "-option"), ("version", True), ("version", 1), ("base", "-option"), ("base_revision", "sha256:latest")]:
            record = {**RECORD, key: value}
            with self.assertRaises(RuntimeError):
                subject.verify(record, self.host, Mock())
        self.assertEqual(self.calls, [])

    def test_missing_snapshot_prevents_restore_or_reattachment(self):
        host = Mock(side_effect=[json.dumps(BASE), "[]"])
        with self.assertRaises(RuntimeError):
            subject.verify(RECORD, host, Mock())
        self.assertEqual(host.call_count, 2)
        self.assertEqual(host.call_args.args, ("snapshot", "list", "--json"))

    def test_failed_content_check_retains_resources_without_retry(self):
        guest = Mock(side_effect=RuntimeError("content mismatch"))
        with self.assertRaises(RuntimeError):
            subject.verify(RECORD, self.host, guest)
        self.assertEqual(guest.call_count, 1)
        self.assertEqual(len(self.calls), 3)
        self.assertFalse(any("delete" in args for args in self.calls))

    def test_restore_cannot_reuse_current_data(self):
        def host(*args):
            if args[:2] == ("snapshot", "restore"):
                return json.dumps({"environment": args[-1], "state": "running", "workspace": RECORD["workspace"], "oci": RECORD["oci"]})
            return self.host(*args)
        guest = Mock()
        with self.assertRaises(RuntimeError):
            subject.verify(RECORD, host, guest)
        guest.assert_not_called()

    def test_missing_or_changed_base_refuses_before_retention_mutations(self):
        for base in ({}, [], {**BASE, "revision": "sha256:" + "8" * 64}):
            host, guest = Mock(return_value=json.dumps(base)), Mock()
            with self.assertRaisesRegex(RuntimeError, 'Base identity'):
                subject.verify(RECORD, host, guest)
            self.assertEqual(host.call_count, 1)
            guest.assert_not_called()

    def test_created_environment_must_use_recorded_base_without_retry(self):
        for result in ({}, {"name": "reclaim-current-" + RECORD["nonce"], "base": {**BASE, "revision": "sha256:" + "8" * 64}}):
            self.calls = []
            def host(*args):
                original = self.host(*args)
                return json.dumps(result) if args[:2] == ("env", "create") else original
            guest = Mock()
            with self.assertRaisesRegex(RuntimeError, 'Base identity unproven'):
                subject.verify(RECORD, host, guest)
            self.assertEqual(sum(args[:2] == ("env", "create") for args in self.calls), 1)
            self.assertEqual(guest.call_count, 1)

    def test_native_failure_preserves_exit_and_fixed_reason_without_output(self):
        script = "import sys; print('private stdout'); print('[failed] operation=environment_create stage=controller reason=busy', file=sys.stderr); print('Authorization: Bearer private-token', file=sys.stderr); sys.exit(23)"
        with self.assertRaisesRegex(RuntimeError, 'exit_code=23 reason=busy') as error:
            subject.run([sys.executable, '-c', script])
        self.assertNotIn('private', str(error.exception))
        self.assertEqual(subject.failure_reason(b'[failed] operation=environment_create stage=controller reason=private-token'), 'unclassified')

    def test_controller_projection_returns_only_compiled_labels(self):
        raw = json.dumps({'MESSAGE': 'resolve Base private-token from https://user:password@example.invalid: runtime unavailable', 'SECRET': 'private-token'}).encode()
        self.assertEqual(reclamation_diagnostics.project(raw), {'state': 'observed', 'observations': ['base_resolution', 'runtime_unavailable']})
        for raw in (b'not JSON private-token', b'[]', b'null'):
            self.assertEqual(reclamation_diagnostics.project(raw), {'state': 'invalid'})
        self.assertEqual(reclamation_diagnostics.project(b' ' * ((1 << 20) + 1)), {'state': 'oversized'})

    def test_failure_observation_is_read_only_bounded_and_preserves_original(self):
        primary = RuntimeError('primary operation failure')
        for diagnostic in (RuntimeError('diagnostic failure'), '{"observations": ["private-token"]}', '{"observations": ["base_resolution"]}'):
            with self.subTest(diagnostic=diagnostic), patch.object(subject, 'run', side_effect=[primary, diagnostic]) as run, contextlib.redirect_stdout(io.StringIO()) as output:
                with self.assertRaises(RuntimeError) as error:
                    subject.verify_installed(RECORD, 'registration')
                self.assertIs(error.exception, primary)
                self.assertEqual(run.call_count, 2)
                command = run.call_args.args[0]
                self.assertEqual(command[:7], ['wsl.exe', '--distribution-id', 'registration', '--user', 'root', '--exec', 'python3'])
                self.assertEqual(run.call_args.kwargs, {'timeout': 30})
                self.assertNotIn('private-token', output.getvalue())

    def test_timeout_never_repeats_mutation(self):
        with patch.object(subject.subprocess, 'run', side_effect=subprocess.TimeoutExpired('private-command', 900)) as run:
            with self.assertRaisesRegex(RuntimeError, 'timed out') as error:
                subject.run(['private-command'])
            self.assertEqual(run.call_count, 1)
            self.assertNotIn('private-command', str(error.exception))

if __name__ == "__main__":
    unittest.main()

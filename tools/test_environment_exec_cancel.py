"""Cancellation observer regressions, not native Incus acceptance."""

import contextlib
import io
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock

import environment_exec_cancel as cancel


@unittest.skipUnless(sys.platform == "linux", "guest fixture observes Linux procfs")
class ProcObservationTests(unittest.TestCase):
    def start(self, command=cancel.START_COMMAND):
        process = subprocess.Popen(["sh", "-ec", command], stdout=subprocess.PIPE,
                                   stderr=subprocess.DEVNULL, bufsize=0)
        self.addCleanup(self.stop, process)
        return process

    @staticmethod
    def stop(process):
        if process.poll() is None:
            process.kill()
        process.wait(timeout=5)
        process.stdout.close()

    def observe(self, identity, command=cancel.OBSERVE_COMMAND):
        return subprocess.run(["sh", "-ec", command, "sh", *identity],
                              capture_output=True, text=True, timeout=5)

    def test_exact_shell_to_sleep_identity_alive_reused_restart_and_absent(self):
        process = self.start()
        identity = cancel.read_identity(process, time.monotonic() + 5)
        self.assertEqual(int(identity[0]), process.pid)
        self.assertEqual(self.observe(identity).stdout, "present\n")
        self.assertEqual(self.observe((identity[0], str(int(identity[1]) + 1), identity[2])).stdout,
                         "reused\n")
        self.assertEqual(self.observe((*identity[:2], str(int(identity[2]) + 1))).stdout,
                         "environment_changed\n")
        process.terminate()
        process.wait(timeout=5)
        self.assertEqual(self.observe(identity).stdout, "absent\n")

    def test_unreaped_zombie_is_not_absent(self):
        pid = os.fork()
        if pid == 0:
            os._exit(0)
        self.addCleanup(os.waitpid, pid, 0)
        until = time.monotonic() + 5
        while True:
            fields = Path(f"/proc/{pid}/stat").read_text().rsplit(") ", 1)[1].split()
            if fields[0] == "Z":
                break
            self.assertLess(time.monotonic(), until)
            time.sleep(0.01)
        init = Path("/proc/1/stat").read_text().rsplit(") ", 1)[1].split()[19]
        result = self.observe((str(pid), fields[19], init))
        self.assertEqual((result.returncode, result.stdout), (0, "zombie\n"))

    def test_proc_parser_handles_comm_and_refuses_unknown_facts(self):
        valid = "123 (space ) comm) S " + "0 " * 18 + "456\n"
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "stat"
            command = cancel.STAT_READER + '\nread_stat "$1" 123; printf "%s %s\\n" "$state" "$start"'
            for content, expected in ((valid, 0), ("", 1), ("garbage", 1),
                                      (valid.replace("123 (", "124 ("), 1),
                                      (valid.replace(") S ", ") ? "), 1),
                                      (valid.replace("456", "secret-value"), 1),
                                      ("123 (short) S 1\n", 1)):
                with self.subTest(expected=expected):
                    path.write_text(content)
                    result = subprocess.run(["sh", "-ec", command, "sh", str(path)],
                                            capture_output=True, text=True, timeout=5)
                    self.assertEqual(result.returncode, expected)
                    self.assertEqual(result.stdout, "S 456\n" if expected == 0 else "")
            path.unlink()
            result = subprocess.run(["sh", "-ec", command, "sh", str(path)],
                                    capture_output=True, text=True, timeout=5)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(result.stdout, "")

    def test_missing_malformed_or_unreadable_proc_facts_cannot_prove_absence(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "1").mkdir()
            (root / "123").mkdir()
            (root / "1/stat").write_text("1 (init) S " + "0 " * 18 + "789\n")
            target = root / "123/stat"
            command = cancel.OBSERVE_COMMAND.replace("/proc", directory)
            for malformed in ("garbage\n", "123 (broken) S 1\n"):
                target.write_text(malformed)
                self.assertEqual(self.observe(("123", "456", "789"), command).stdout, "unknown\n")
            target.unlink()  # Directory exists but stat cannot be read.
            self.assertEqual(self.observe(("123", "456", "789"), command).stdout, "unknown\n")
            (root / "123").rmdir()
            self.assertEqual(self.observe(("123", "456", "789"), command).stdout, "absent\n")
            self.assertEqual(self.observe(("123", "456", "790"), command).stdout,
                             "environment_changed\n")
            (root / "1/stat").write_text("secret-invalid\n")
            result = self.observe(("123", "456", "789"), command)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(result.stdout, "")

    def test_readiness_eof_invalid_overflow_and_timeout_are_bounded_and_redacted(self):
        for command, reason in (("exit 0", "before"), ("printf 'secret-invalid\\n'", "invalid"),
                                ("printf '%0100d' 1", "bound"), ("exec sleep 600", "timed out")):
            with self.subTest(reason=reason):
                process = self.start(command)
                deadline = time.monotonic() + 0.1
                with self.assertRaisesRegex(RuntimeError, reason) as error:
                    cancel.read_identity(process, deadline)
                self.assertNotIn("secret-invalid", str(error.exception))
                self.stop(process)


class CancellationContractTests(unittest.TestCase):
    def setUp(self):
        self.original = {"name": "owned", "runtime_ref": "exact-runtime", "created_at": "generation"}
        self.process = mock.Mock(returncode=130)
        self.process.poll.return_value = 130
        self.now = 0.0
        self.calls = []
        self.observations = ["absent\n"]
        self.deadlines = []
        self.status_calls = 0
        self.rows_calls = 0
        self.patches = [mock.patch.object(cancel.subprocess, "Popen", return_value=self.process),
                        mock.patch.object(cancel, "read_identity", return_value=("123", "456", "789")),
                        mock.patch.object(cancel.time, "monotonic", side_effect=lambda: self.now),
                        mock.patch.object(cancel.time, "sleep", side_effect=self.sleep)]
        for patch in self.patches:
            patch.start()
            self.addCleanup(patch.stop)

    def sleep(self, duration):
        self.now += duration

    def rows(self, *, timeout=300):
        self.rows_calls += 1
        self.calls.append("rows")
        return {"owned": self.original.copy()}

    def invoke(self, *args, timeout, limit=None):
        self.calls.append(args[0])
        if args[:2] == ("env", "status"):
            self.status_calls += 1
            return json.dumps({"environment": self.original, "state": "running"})
        self.assertEqual(args[:6], ("exec", "owned", "--", "sh", "-ec", cancel.OBSERVE_COMMAND))
        self.assertEqual(args[6:], ("sh", "123", "456", "789"))
        self.deadlines.append(timeout)
        self.now += 1
        return self.observations.pop(0) if len(self.observations) > 1 else self.observations[0]

    def verify(self, invoke=None, rows=None):
        with contextlib.redirect_stdout(io.StringIO()) as output:
            cancel.verify_cancellation("product", "owned", self.original, invoke or self.invoke, rows or self.rows)
        return output.getvalue()

    def test_waits_for_exact_absence_without_mutating_or_replaying_workload(self):
        self.observations = ["present\n", "zombie\n", "reused\n"]
        self.process.wait.side_effect = lambda **kwargs: self.sleep(5)
        self.assertIn("EXACT EXECUTION ABSENT", self.verify())
        self.process.send_signal.assert_called_once_with(signal.SIGINT)
        self.process.terminate.assert_not_called()
        self.process.kill.assert_not_called()
        self.assertEqual(self.status_calls, 2)
        self.assertEqual(self.rows_calls, 2)
        self.assertEqual(self.calls, ["rows", "env", "rows", "exec", "exec", "exec", "env"])
        self.assertEqual(self.deadlines[0], 25)
        self.assertTrue(all(a > b for a, b in zip(self.deadlines, self.deadlines[1:])))

    def test_leak_and_zombie_fail_before_cleanup_or_env_stop(self):
        for result in ("present\n", "zombie\n"):
            with self.subTest(result=result):
                self.now = 0
                self.observations = [result]
                with self.assertRaisesRegex(RuntimeError, "target remained|timed out"):
                    self.verify()
                self.process.terminate.assert_not_called()
                self.process.kill.assert_not_called()
                self.assertNotIn("stop", self.calls)

    def test_unknown_malformed_restart_and_observer_failures_never_pass(self):
        for result in ("unknown\n", "", "secret-value\n", "absent\nextra"):
            with self.subTest(result=result):
                self.observations = [result]
                with self.assertRaisesRegex(RuntimeError, "absence was unknown") as error:
                    self.verify()
                self.assertNotIn("secret-value", str(error.exception))
        def failing(*args, **kwargs):
            if args[0] == "exec":
                raise subprocess.TimeoutExpired("secret-command", 1, output="absent\n")
            return self.invoke(*args, **kwargs)
        with self.assertRaisesRegex(RuntimeError, "observation failed") as error:
            self.verify(invoke=failing)
        self.assertNotIn("secret-command", str(error.exception))

    def test_restart_stopped_environment_and_failed_observer_are_not_success(self):
        self.observations = ["environment_changed\n"]
        with self.assertRaisesRegex(RuntimeError, "guest incarnation changed"):
            self.verify()
        self.observations = ["absent\n"]
        def stopped(*args, **kwargs):
            result = self.invoke(*args, **kwargs)
            if args[0] == "env" and "exec" in self.calls:
                return json.dumps({"environment": self.original, "state": "stopped"})
            return result
        self.calls.clear()
        with self.assertRaisesRegex(RuntimeError, "running Environment identity"):
            self.verify(invoke=stopped)
        def failed(*args, **kwargs):
            if args[0] == "exec":
                # invoke owns checking returncode; plausible stdout is insufficient.
                raise RuntimeError("ordinary product command returned an unexpected status")
            return self.invoke(*args, **kwargs)
        with self.assertRaisesRegex(RuntimeError, "unexpected status"):
            self.verify(invoke=failed)

    def test_final_status_cannot_exceed_shared_deadline(self):
        def late(*args, **kwargs):
            result = self.invoke(*args, **kwargs)
            if args[0] == "env" and "exec" in self.calls:
                self.now = 31
            return result
        with self.assertRaisesRegex(RuntimeError, "timed out"):
            self.verify(invoke=late)

    def test_cleanup_error_preserves_primary_observation_failure(self):
        self.process.poll.return_value = None
        self.process.terminate.side_effect = OSError("secret-cleanup-error")
        with mock.patch.object(cancel, "read_identity", side_effect=RuntimeError("primary-readiness-failure")):
            with self.assertRaisesRegex(RuntimeError, "primary-readiness-failure") as error:
                self.verify()
        self.assertEqual(error.exception.__notes__, ["exec fixture local process cleanup also failed"])
        self.assertNotIn("secret-cleanup-error", str(error.exception))
        self.process.stdout.close.assert_called_once_with()

    def test_owned_client_cleanup_runs_only_after_failure(self):
        self.process.wait.side_effect = [subprocess.TimeoutExpired("private", 30), None]
        self.process.poll.return_value = None
        with self.assertRaisesRegex(RuntimeError, "observation failed"):
            self.verify()
        self.process.terminate.assert_called_once_with()
        self.assertNotIn("exec", self.calls)

    def test_native_fixture_observes_before_stop_and_keeps_normal_cleanup(self):
        root = Path(__file__).resolve().parents[1]
        source = (root / "tools/test_environment_exec.py").read_text()
        self.assertLess(source.index("verify_cancellation(product"), source.index('invoke("stop", name)'))
        self.assertIn("cleanup_environment(name, original", source)
        for path in ("tools/ci-local.sh", ".github/workflows/test.yml"):
            self.assertIn("python3 tools/test_environment_exec_cancel.py", (root / path).read_text())

    def test_client_failure_or_changed_catalog_cannot_reach_absence_probe(self):
        self.process.returncode = 1
        with self.assertRaisesRegex(RuntimeError, "exit 130"):
            self.verify()
        self.assertNotIn("exec", self.calls)
        self.process.returncode = 130
        self.calls.clear()
        count = 0
        def replaced(**kwargs):
            nonlocal count
            count += 1
            return {"owned": self.original if count == 1 else dict(self.original, created_at="replacement")}
        with self.assertRaisesRegex(RuntimeError, "retained Environment"):
            self.verify(rows=replaced)
        self.assertNotIn("exec", self.calls)


class OwnedCleanupTests(unittest.TestCase):
    def test_retains_unknown_changed_and_unrecorded_targets(self):
        original = {"created_at": "generation", "runtime_ref": "exact"}
        for saved, current in ((None, original), (original, dict(original, created_at="replacement"))):
            with self.subTest(saved=saved):
                invoke = mock.Mock()
                with self.assertRaisesRegex(RuntimeError, "ownership was unknown"):
                    cancel.cleanup_environment("owned", saved, invoke, lambda: {"owned": current}, None)
                invoke.assert_not_called()
        invoke = mock.Mock()
        unavailable_rows = mock.Mock(side_effect=OSError("private"))
        with self.assertRaisesRegex(RuntimeError, "ownership was unknown"):
            cancel.cleanup_environment("owned", original, invoke, unavailable_rows, None)
        invoke.assert_not_called()

    def test_only_matching_target_is_removed_and_cleanup_failure_keeps_primary(self):
        original = {"created_at": "generation", "runtime_ref": "exact"}
        invoke = mock.Mock()
        cancel.cleanup_environment("owned", original, invoke, lambda: {"owned": original}, None)
        invoke.assert_called_once_with("rm", "-f", "owned")
        invoke.reset_mock()
        cancel.cleanup_environment("owned", original, invoke, lambda: {}, None)
        invoke.assert_not_called()
        primary = RuntimeError("primary-cancellation-failure")
        invoke.side_effect = RuntimeError("private-output")
        cancel.cleanup_environment("owned", original, invoke, lambda: {"owned": original}, primary)
        self.assertEqual(str(primary), "primary-cancellation-failure")
        self.assertEqual(primary.__notes__, ["exec fixture Environment cleanup also failed or ownership was unknown"])

    def test_disposable_guard_still_precedes_all_commands(self):
        import test_environment_exec as fixture
        with mock.patch.dict(fixture.os.environ, {}, clear=True), mock.patch.object(fixture, "run_product") as run:
            with self.assertRaisesRegex(SystemExit, "disposable GHA"):
                fixture.main()
            run.assert_not_called()


@unittest.skipUnless(sys.platform == "linux", "local fixture process checks")
class BoundedCaptureTests(unittest.TestCase):
    def test_output_bound_and_nonzero_status_cannot_be_success(self):
        cases = (("print('x'*100)", "exceeded"),
                 ("print('absent'); raise SystemExit(9)", "unexpected status"),
                 ("import os; os.write(1, bytes([255]))", "output failed"),
                 ("import time; time.sleep(60)", "timed out"))
        for source, message in cases:
            with self.subTest(message=message), self.assertRaisesRegex(RuntimeError, message):
                cancel.run_product(sys.executable, "-c", source, timeout=0.1, limit=32)
        self.assertEqual(cancel.run_product(sys.executable, "-c", "print('absent')", timeout=5, limit=32),
                         "absent\n")
        self.assertEqual(cancel.run_product(sys.executable, "-c", "import os; os.write(2, b'x'*1000000)",
                                           timeout=5, limit=32), "")


if __name__ == "__main__":
    unittest.main()

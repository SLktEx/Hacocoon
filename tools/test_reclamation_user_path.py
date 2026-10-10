#!/usr/bin/env python3
"""Refusal tests for the native gate's resume decision, without WSL mutation."""
import importlib.util
import base64
import io
from contextlib import redirect_stdout
from pathlib import Path
import json
import os
import re
import subprocess
import sys
import tempfile
import threading
import time
from types import FunctionType, SimpleNamespace
import unittest
from unittest.mock import Mock, call, patch

spec = importlib.util.spec_from_file_location("reclaim_gate", Path(__file__).resolve().parents[1] / "test/e2e/windows/reclamation.py")
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)
OP = "{11111111-1111-4111-8111-111111111111}"
PROCESSES = {"helpers": [], "counts": {"wslhost.exe": 0, "wsl.exe": 0, "vmmemWSL": 1}}


class ReclamationUserPathTests(unittest.TestCase):
    def test_observation_requires_success_and_complete_bounded_json(self):
        valid = b'\xef\xbb\xbf{"state":"none","operation":""}\n'
        exact_limit = json.dumps("x" * 16382).encode()
        self.assertEqual(len(exact_limit), 16384)
        for payload, code, accepted in ((valid, 0, True), (exact_limit, 0, True),
                                        (valid, 17, False), (b'', 0, False),
                                        (b'{"state":', 0, False), (b'{}\n{}', 0, False),
                                        (b'\xff', 0, False), (exact_limit + b' ', 0, False)):
            command = [sys.executable, '-c',
                       'import base64,sys;sys.stdout.buffer.write(base64.b64decode(sys.argv[1]));sys.exit(int(sys.argv[2]))',
                       base64.b64encode(payload).decode(), str(code)]
            with self.subTest(size=len(payload), code=code, accepted=accepted):
                if accepted:
                    self.assertEqual(gate.read_json(command), json.loads(payload.decode('utf-8-sig')))
                else:
                    with self.assertRaisesRegex(RuntimeError, 'Windows read-only observation'):
                        gate.read_json(command)

    def test_observation_timeout_never_accepts_captured_json(self):
        command = ['private-observation-path']
        for failure in ('none', 'wait', 'kill', 'exited'):
            process = Mock()
            process.poll.return_value = 0 if failure == 'exited' else None
            process.wait.side_effect = [subprocess.TimeoutExpired(command, 25),
                                        subprocess.TimeoutExpired(command, 5) if failure == 'wait' else 0]
            if failure == 'kill':
                process.kill.side_effect = OSError('private kill failure')
            output = io.BytesIO()

            def start(*args, **kwargs):
                self.assertIs(kwargs['stdout'], output)
                self.assertEqual(kwargs['stderr'], subprocess.DEVNULL)
                output.write(b'{"state":"complete"}')
                return process

            with self.subTest(failure=failure), \
                 patch.object(gate.tempfile, 'TemporaryFile', return_value=output), \
                 patch.object(gate.subprocess, 'Popen', side_effect=start):
                expected = 'timed out' + ('; termination unconfirmed' if failure in ('wait', 'kill') else '')
                with self.assertRaisesRegex(RuntimeError, expected) as raised:
                    gate.read_json(command)
            self.assertNotIn('private', str(raised.exception))
            self.assertTrue(output.closed)
            self.assertEqual(process.wait.call_args_list,
                             [call(timeout=25)] if failure == 'kill' else [call(timeout=25), call(timeout=5)])
            self.assertEqual(process.kill.call_count, 0 if failure == 'exited' else 1)
            process.communicate.assert_not_called()

    def test_observation_start_and_wait_failures_are_private_and_close_capture(self):
        for failure in ('start', 'wait'):
            process = Mock()
            process.poll.return_value = None
            process.wait.side_effect = [OSError('private wait failure'), 0]
            output = io.BytesIO()
            with self.subTest(failure=failure), \
                 patch.object(gate.tempfile, 'TemporaryFile', return_value=output), \
                 patch.object(gate.subprocess, 'Popen',
                              side_effect=OSError('private start failure') if failure == 'start' else None,
                              return_value=process):
                with self.assertRaisesRegex(RuntimeError, 'observation unavailable') as raised:
                    gate.capture_observation(['private-command'])
            self.assertNotIn('private', str(raised.exception))
            self.assertTrue(output.closed)
            self.assertEqual(process.kill.call_count, 0 if failure == 'start' else 1)

    def test_capture_close_failure_preserves_timeout_and_refuses_success(self):
        class CloseFailure(io.BytesIO):
            def close(self):
                super().close()
                raise OSError('private file failure')

        for timed_out in (False, True):
            output = CloseFailure(b'{}')
            process = Mock()
            process.poll.return_value = 0
            process.wait.side_effect = [subprocess.TimeoutExpired('private', 25), 0] if timed_out else [0]
            with self.subTest(timed_out=timed_out), \
                 patch.object(gate.tempfile, 'TemporaryFile', return_value=output), \
                 patch.object(gate.subprocess, 'Popen', return_value=process):
                with self.assertRaisesRegex(RuntimeError, 'timed out' if timed_out else 'capture cleanup failed') as raised:
                    gate.capture_observation(['private-command'])
            self.assertNotIn('private', str(raised.exception))
            self.assertTrue(output.closed)

    def test_observation_does_not_wait_for_inherited_output_after_emitter_exit(self):
        # The fixed observation process is the only emitter. A descendant holds
        # stdout open but never writes it. Exercise real pipes, not fake output.
        with tempfile.TemporaryDirectory() as directory:
            ready, release, released = [Path(directory) / name for name in ("ready", "release", "released")]
            holder = ("import os,pathlib,sys,time;ready,release,released=map(pathlib.Path,sys.argv[1:]);"
                      "ready.touch();end=time.monotonic()+10\n"
                      "while not release.exists() and time.monotonic()<end: time.sleep(0.01)\n"
                      "os.close(1);os.close(2);released.touch()")
            emitter = ("import subprocess,sys;subprocess.Popen([sys.executable,'-c',sys.argv[1],*sys.argv[2:]],"
                       "close_fds=False);print('{\"fixture\":\"complete\"}',flush=True)")
            command = [sys.executable, "-c", emitter, holder, str(ready), str(release), str(released)]
            processes, waits, result, errors, captures = [], [], [], [], []
            finished = threading.Event()
            original_temporary_file = tempfile.TemporaryFile

            def temporary_file():
                output = original_temporary_file()
                captures.append(output)
                return output

            class TrackedProcess(subprocess.Popen):
                def __init__(self, *args, **kwargs):
                    super().__init__(*args, **kwargs)
                    processes.append(self)
                    end = time.monotonic() + 3
                    while not ready.exists() and time.monotonic() < end:
                        time.sleep(0.01)
                    if self.wait(timeout=3) != 0:
                        raise RuntimeError("fixture emitter failed")

                def communicate(self, input=None, timeout=None):
                    waits.append(timeout)
                    return super().communicate(input, timeout)

            # Keep the real platform Popen and real pipes. Only select CPython's
            # Windows run() cleanup branch on non-Windows hosts. Never alter the
            # subprocess module globals used by the actual Popen implementation.
            namespace = dict(subprocess.run.__globals__, _mswindows=True, Popen=TrackedProcess)
            windows_run = FunctionType(subprocess.run.__code__, namespace,
                                       argdefs=subprocess.run.__defaults__)
            windows_run.__kwdefaults__ = subprocess.run.__kwdefaults__

            def shortened_run(*args, **kwargs):
                self.assertEqual(kwargs.pop("timeout"), 25)
                return windows_run(*args, timeout=0.1, **kwargs)

            def observe():
                try:
                    result.append(gate.read_json(command))
                except BaseException as error:
                    errors.append(error)
                finally:
                    finished.set()

            facade = SimpleNamespace(run=shortened_run, Popen=TrackedProcess,
                                     TimeoutExpired=subprocess.TimeoutExpired, DEVNULL=subprocess.DEVNULL)
            worker = threading.Thread(target=observe, daemon=True)
            try:
                with patch.object(gate, "subprocess", facade), \
                     patch.object(gate, "tempfile", SimpleNamespace(TemporaryFile=temporary_file), create=True):
                    worker.start()
                    end = time.monotonic() + 4
                    while not ready.exists() and time.monotonic() < end:
                        time.sleep(0.01)
                    self.assertTrue(ready.exists(), "fixture holder did not start")
                    self.assertTrue(finished.wait(1),
                                    "observation blocked after emitter exit; cleanup waits=" + str(waits))
                    self.assertEqual(errors, [])
                    self.assertEqual(result, [{"fixture": "complete"}])
                    self.assertFalse(released.exists(), "holder ended before the observation returned")
                    self.assertEqual(len(captures), 1)
                    self.assertTrue(captures[0].closed)
                    # Windows O_TEMPORARY deletion belongs to the last inherited
                    # handle close. Do not infer immediate deletion from ours.
            finally:
                release.touch()
                worker.join(3)
                for process in processes:
                    if process.poll() is None:
                        process.kill()
                    process.wait(timeout=3)
                end = time.monotonic() + 3
                while not released.exists() and time.monotonic() < end:
                    time.sleep(0.01)
            self.assertFalse(worker.is_alive())
            self.assertTrue(released.exists())
            if os.name == 'nt' and captures:
                name = Path(captures[0].name)
                end = time.monotonic() + 3
                while name.exists() and time.monotonic() < end:
                    time.sleep(0.01)
                self.assertFalse(name.exists(), 'capture remained after the fixture released its handle')

    def test_phase_receipt_has_its_own_line_after_a_terminal_prompt(self):
        output = io.StringIO()
        with redirect_stdout(output):
            print("root@haco-host:~# ", end="")
            gate.report_phase("host_entry")
        self.assertEqual(json.loads(output.getvalue().splitlines()[1]),
                         {"component": "ci", "operation": "reclamation_user_path", "phase": "host_entry"})

    def test_failed_receipts_cannot_skip_cleanup_or_replace_primary_failure(self):
        for error in (BrokenPipeError("private output error"), ValueError("closed runner stream")):
            observer = gate.ProcessStartObserver()
            observer.process = Mock()
            observer.process.poll.return_value = None
            observer.thread = Mock()
            observer.thread.is_alive.return_value = True
            with self.subTest(error=error), \
                 patch.object(gate.ProcessStartObserver, "__enter__", return_value=observer), \
                 patch("builtins.print", side_effect=error):
                gate.report_phase("host_entry")
                with self.assertRaisesRegex(RuntimeError, "original worker failure"):
                    with observer:
                        raise RuntimeError("original worker failure")
            observer.process.terminate.assert_called_once_with()
            observer.process.wait.assert_called_once_with(timeout=5)
            observer.thread.join.assert_called_once_with(timeout=5)
            observer.process.stdout.close.assert_not_called()

    def test_phase_receipts_are_fixed_vocabulary_and_flushed(self):
        with patch("builtins.print") as output:
            for phase in sorted(gate.USER_PHASES):
                gate.report_phase(phase)
            self.assertEqual([json.loads(call.args[0]) for call in output.call_args_list],
                             [{"component": "ci", "operation": "reclamation_user_path", "phase": phase}
                              for phase in sorted(gate.USER_PHASES)])
            self.assertTrue(all(call.kwargs == {"flush": True} for call in output.call_args_list))
            with self.assertRaises(ValueError):
                gate.report_phase("private child text")
            self.assertNotIn("private", str(output.call_args_list))

    def test_workflow_identifies_each_regression_before_the_user_journey(self):
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/windows-all-scripts-e2e.yml").read_text()
        section = workflow.split('name: "[product] Verify public reclamation through ordinary Host entry"', 1)[1]
        section = section.split('      - name:', 1)[0]
        receipts = [json.loads(raw) for raw in re.findall(r"Write-Host '(\{[^\n]+\})'", section)]
        operations = ("reclamation_retention_regressions", "reclamation_observer_regressions", "reclamation_journey")
        self.assertEqual(receipts, [{"component": "ci", "operation": op, "state": state}
                                    for op in operations for state in ("started", "passed")])
        commands = ("python ./tools/test_reclamation_retention.py",
                    "python ./tools/test_reclamation_user_path.py",
                    "python ./test/e2e/windows/reclamation.py")
        for operation, command in zip(operations, commands):
            start = section.index('"operation":"' + operation + '","state":"started"')
            invoked = section.index(command)
            checked = section.index('if ($LASTEXITCODE -ne 0)', invoked)
            passed = section.index('"operation":"' + operation + '","state":"passed"')
            self.assertLess(start, invoked)
            self.assertLess(invoked, checked)
            self.assertLess(checked, passed)

    def test_observer_teardown_preserves_primary_failure_and_owned_process(self):
        for waiting in (None, subprocess.TimeoutExpired("private observer command", 5)):
            observer = gate.ProcessStartObserver()
            observer.process = Mock()
            observer.process.poll.return_value = None
            observer.process.wait.side_effect = waiting
            observer.thread = Mock()
            observer.thread.is_alive.return_value = True
            with self.subTest(waiting=waiting), \
                 patch.object(gate.ProcessStartObserver, "__enter__", return_value=observer), \
                 patch("builtins.print") as output:
                with self.assertRaisesRegex(RuntimeError, "original worker failure"):
                    with observer:
                        raise RuntimeError("original worker failure")
            observer.process.terminate.assert_called_once_with()
            observer.process.wait.assert_called_once_with(timeout=5)
            observer.process.stdout.close.assert_not_called()
            self.assertIn({"state": "unavailable"}, observer.rows)
            self.assertNotIn("private", str(output.call_args_list))

    def test_finished_observer_reader_closes_its_stream(self):
        observer = gate.ProcessStartObserver()
        observer.process = Mock()
        observer.process.poll.return_value = 0
        stream = io.BytesIO(b'{"state":"ready"}\n')
        observer.process.stdout = stream
        observer.thread = threading.Thread(target=observer.read)
        observer.thread.start()
        observer.close()
        self.assertFalse(observer.thread.is_alive())
        self.assertTrue(stream.closed)
        observer.process.terminate.assert_not_called()

    def test_observer_close_does_not_close_a_live_readers_buffer(self):
        entered = threading.Event()
        release = threading.Event()
        finished = threading.Event()
        errors = []

        class HeldRead(io.RawIOBase):
            def readable(self):
                return True

            def readinto(self, buffer):
                entered.set()
                if not release.wait(5):
                    raise OSError("bounded fixture release expired")
                return 0

        observer = gate.ProcessStartObserver()
        stream = io.BufferedReader(HeldRead())
        observer.process = SimpleNamespace(stdout=stream, poll=lambda: 0,
                                           wait=lambda timeout: 0)
        reader = threading.Thread(target=observer.read, daemon=True)
        observer.thread = reader
        reader.start()
        original_join = reader.join

        def close():
            try:
                observer.close()
            except BaseException as exc:
                errors.append(exc)
            finally:
                finished.set()

        closer = threading.Thread(target=close, daemon=True)
        try:
            self.assertTrue(entered.wait(1), "fixture reader did not begin")
            # Exercise the production join-timeout branch without a five-second
            # unit-test delay. The real BufferedReader still owns its read lock.
            with patch.object(reader, "join", side_effect=lambda timeout: original_join(0.01)) as join:
                closer.start()
                self.assertTrue(finished.wait(1), "observer close blocked on a live reader")
                join.assert_called_once_with(timeout=5)
            self.assertEqual(errors, [])
            self.assertTrue(reader.is_alive())
            self.assertFalse(stream.closed)
            self.assertIn({"state": "unavailable"}, observer.rows)
        finally:
            release.set()
            original_join(1)
            if closer.ident is not None:
                closer.join(1)
        self.assertFalse(reader.is_alive())
        self.assertFalse(closer.is_alive())
        self.assertTrue(stream.closed)

    def test_start_event_projection_rejects_private_fields_and_unbounded_values(self):
        valid = {"state": "observed", "kind": "wsl", "chain": "notification/powershell/other", "duration_ms": 12}
        self.assertEqual(gate.observed_start_event(valid), valid)
        for key, value in (("kind", "private.exe"), ("chain", "private.exe"), ("chain", []),
                           ("duration_ms", True), ("duration_ms", -1), ("duration_ms", 725001),
                           ("private", "secret")):
            with self.subTest(key=key, value=value):
                self.assertEqual(gate.observed_start_event(dict(valid, **{key: value})), {"state": "unavailable"})
        self.assertEqual(gate.observed_start_event({"state": "unavailable", "error": "private"}), {"state": "unavailable"})

    def test_event_reader_is_bounded_and_records_missing_readiness(self):
        import io
        from types import SimpleNamespace
        for content, state in ((b'', 'unavailable'), (b'x' * 2049, 'unavailable'),
                               (b'{"state":"ready"}\n' * 200, 'truncated')):
            observer = gate.ProcessStartObserver()
            observer.process = SimpleNamespace(stdout=io.BytesIO(content))
            observer.read()
            self.assertTrue(observer.ready.is_set())
            self.assertEqual(observer.rows[-1], {"state": state})
            self.assertLessEqual(len(observer.rows), 132)

    def test_unavailable_observer_cannot_replace_product_failure(self):
        with patch.dict(gate.os.environ, {"SystemRoot": r"C:\Windows"}), \
             patch.object(gate.subprocess, 'CREATE_NO_WINDOW', 0, create=True), \
             patch.object(gate.subprocess, 'Popen', side_effect=OSError('private error')), \
             patch('builtins.print') as output:
            with self.assertRaisesRegex(RuntimeError, 'original worker failure'):
                with gate.ProcessStartObserver():
                    raise RuntimeError('original worker failure')
            self.assertNotIn('private', str(output.call_args_list))
            self.assertIn('unavailable', str(output.call_args_list))

    @unittest.skipUnless(os.name == "nt", "Windows PowerShell event projection contract")
    def test_event_projection_keeps_exited_launch_and_rejects_reused_parent(self):
        fixture = r"""
function Register-CimIndicationEvent { }
function Unregister-Event { }
function Remove-Event { }
function Get-Event { }
$script:events=0;
function Wait-Event {
  $script:events++;if($script:events -gt 2) { throw 'end fixture' };
  [pscustomobject]@{EventIdentifier=1;SourceEventArgs=[pscustomobject]@{NewEvent=[pscustomobject]@{
    ProcessName='wsl.exe';ProcessID=10;ParentProcessID=$script:events;
    TIME_CREATED=([datetime]'2026-01-01T00:00:10Z').ToFileTimeUtc() }}}
}
function Get-CimInstance {
  # The child has exited before this snapshot. Event parent identity remains.
  [pscustomobject]@{ProcessId=1;ParentProcessId=99;Name='haco-review.exe';CreationDate=[datetime]'2026-01-01T00:00:00Z'};
  [pscustomobject]@{ProcessId=2;ParentProcessId=99;Name='private.exe';CreationDate=[datetime]'2026-01-01T00:01:00Z'};
}
"""
        powershell = Path(os.environ["SystemRoot"]) / "System32/WindowsPowerShell/v1.0/powershell.exe"
        output = gate.capture_observation([str(powershell), '-NoProfile', '-NonInteractive', '-Command',
                                           fixture + gate.process_event_query()])
        rows = [json.loads(line) for line in output.decode('utf-8-sig').splitlines()]
        events = [row for row in rows if row['state'] == 'observed']
        self.assertEqual([row['chain'] for row in events], ['notification/unavailable', 'unavailable'])
        self.assertTrue(all(gate.observed_start_event(row) == row for row in rows))
        self.assertNotIn('private', output.decode('utf-8-sig'))

    def test_host_origins_remain_visible_without_a_live_launcher(self):
        snapshot = {"counts": {"wslhost.exe": 2, "wsl.exe": 0, "vmmemWSL": 1},
                    "origins": {}, "host_origins": {"service/other": 1, "unavailable": 1}}
        self.assertEqual(gate.observed_process_origins(snapshot),
                         {"state": "observed", "chains": {}})
        self.assertEqual(gate.observed_process_origins(snapshot, "wslhost.exe"),
                         {"state": "observed", "chains": snapshot["host_origins"]})
        for origins in ({"private.exe": 2}, {"service": 1}, {"service": True}, None):
            self.assertEqual(gate.observed_process_origins(dict(snapshot, host_origins=origins), "wslhost.exe"),
                             {"state": "unavailable"})

    def test_parent_categories_are_bounded_and_never_emit_arbitrary_names(self):
        snapshot = {"counts": {"wslhost.exe": 2, "wsl.exe": 2, "vmmemWSL": 1},
                    "origins": {"wsl/ssh/editor/other": 1, "ssh/editor/other": 1}}
        self.assertEqual(gate.observed_process_origins(snapshot),
                         {"state": "observed", "chains": snapshot["origins"]})
        for origins in ({"secret.exe": 2}, {"ssh": True}, {"ssh": -1}, {"ssh": 4097},
                        {"ssh": 1}, {"ssh/" * 8 + "editor": 2}, {"ssh//editor": 2}, []):
            with self.subTest(origins=origins):
                self.assertEqual(gate.observed_process_origins(dict(snapshot, origins=origins)),
                                 {"state": "unavailable"})
        self.assertEqual(gate.observed_process_origins(dict(PROCESSES, origins={})),
                         {"state": "observed", "chains": {}})

    @unittest.skipUnless(os.name == "nt", "Windows PowerShell 5.1 query contract")
    def test_native_query_classifies_parents_and_rejects_pid_reuse(self):
        # Supply OS-shaped metadata to the actual PowerShell query. No processes
        # are started/stopped and no raw fixture fields leave its projection.
        fixture = r"""
function Get-CimInstance {
  $start=[datetime]'2026-01-01T00:00:00Z';
  foreach($v in @(
    @(1,0,'private-name.exe',0), @(2,1,'Code.exe',1), @(3,2,'ssh.exe',2),
    @(4,3,'wsl.exe',3), @(5,4,'wsl.exe',4),
    @(6,99,'wsl.exe',1), @(7,8,'wsl.exe',1), @(8,0,'ssh.exe',2),
    @(9,9,'wsl.exe',2), @(10,0,'haco-wsl.exe',1),
    @(11,1,'haco-review.exe',1), @(12,11,'wsl.exe',2),
    @(13,1,'wslrelay.exe',1), @(14,13,'wsl.exe',2),
    @(15,1,'wslservice.exe',1), @(16,15,'wslhost.exe',2),
    @(17,6,'wslhost.exe',5), @(18,1999,'wslhost.exe',5))) {
      [pscustomobject]@{ ProcessId=$v[0];ParentProcessId=$v[1];Name=$v[2];
        CreationDate=$start.AddSeconds($v[3]);ExecutablePath='C:\private\'+$v[2] }
  }
}
"""
        powershell = Path(os.environ["SystemRoot"]) / "System32/WindowsPowerShell/v1.0/powershell.exe"
        snapshot = gate.read_json([str(powershell), "-NoProfile", "-NonInteractive", "-Command",
                                   fixture + gate.process_query()])
        self.assertEqual(snapshot["origins"], {"ssh/editor/other": 1,
                         "wsl/ssh/editor/other": 1, "unavailable": 2, "wsl/unavailable": 1,
                         "notification/other": 1, "wsl-relay/other": 1})
        observed = gate.observed_process_origins(snapshot)
        self.assertEqual(observed["state"], "observed")
        self.assertNotIn("private", json.dumps(observed))
        self.assertEqual(gate.observed_process_origins(snapshot, "wslhost.exe"),
                         {"state": "observed", "chains": {"service/other": 1,
                          "wsl/unavailable": 1, "unavailable": 1}})
        self.assertTrue(gate.helper_is_running(snapshot["helpers"], r"C:\private\haco-wsl.exe"))

    def test_japanese_status_still_requires_the_successful_completion_marker(self):
        message = "容量回収の保存結果はありません。この確認で新しい操作は開始していません。\n"
        self.assertFalse(gate.absent_status_completed(message))
        self.assertTrue(gate.absent_status_completed(message + "HACO_ABSENT_STATUS_EXIT:0\n"))
        with self.assertRaises(RuntimeError):
            gate.absent_status_completed(message + "HACO_ABSENT_STATUS_EXIT:1\n")

    def test_status_waits_for_completion_despite_echo_and_prompt_repaint(self):
        output = ""
        chunks = [
            "root@haco-host:~# \n",
            'haco reclaim --status; printf \'\\nHACO_ABSENT_STATUS_EXIT:%s\\n\' "$?"\n',
            "root@haco-host:~# \n",
            "No saved reclamation result. No operation was started by this status check.\n",
            "HACO_ABSENT_STATUS_EX",
            "IT:",
        ]
        for chunk in chunks:
            output += chunk
            self.assertFalse(gate.absent_status_completed(output))
        self.assertTrue(gate.absent_status_completed(output + "0\n"))

    def test_status_completion_requires_success_and_response(self):
        message = "No saved reclamation result. No operation was started by this status check.\n"
        for output in ("HACO_ABSENT_STATUS_EXIT:0\n",
                       message + "HACO_ABSENT_STATUS_EXIT:1\n",
                       "HACO_ABSENT_STATUS_EXIT:0\n" + message):
            with self.assertRaises(RuntimeError):
                gate.absent_status_completed(output)

    def test_absent_history_refuses_existing_or_partial_evidence(self):
        gate.require_absent_history({"operation": "", "state": "none"})
        for result in (None, {}, {"state": "none"},
                       {"operation": OP, "state": "pending"},
                       {"operation": OP, "state": "failed"},
                       {"operation": "", "state": "none", "linux_started": True}):
            with self.assertRaises(RuntimeError):
                gate.require_absent_history(result)

    def test_process_identity_must_be_observed(self):
        helper = r"C:\fixture\haco-wsl.exe"
        self.assertFalse(gate.helper_is_running([], helper))
        self.assertTrue(gate.helper_is_running({"ExecutablePath": helper.upper()}, helper))
        self.assertFalse(gate.helper_is_running([{"ExecutablePath": r"C:\other\haco-wsl.exe"}], helper))
        for rows in (None, {}, [None], [{"ExecutablePath": None}], [{"ExecutablePath": ""}]):
            with self.assertRaises(RuntimeError):
                gate.helper_is_running(rows, helper)

    def test_pending_and_failed_never_authorize_resume(self):
        self.assertFalse(gate.require_complete({"operation": OP, "state": "pending"}, OP))
        for result in ({"operation": OP, "state": "failed"},
                       {"operation": OP, "state": "complete"},
                       {"operation": "foreign", "state": "complete"},
                       {"operation": OP, "state": "unknown"}):
            with self.assertRaises(RuntimeError):
                gate.require_complete(result, OP)

    def test_failed_status_stops_observation_without_reentry(self):
        with patch.object(gate, "read_json", side_effect=[{"operation": OP, "state": "failed"}, PROCESSES]) as read, patch.dict(gate.os.environ, {"SystemRoot": r"C:\Windows"}), patch("builtins.print"):
            with self.assertRaises(RuntimeError):
                gate.wait_for_worker(Path("fixture-haco-wsl.exe"), OP, OP)
            self.assertEqual(read.call_count, 2)
            self.assertEqual(read.call_args_list[0].args[0][1:], ["_status", OP, OP])
            self.assertEqual(read.call_args_list[1].args[0][1:4], ["-NoProfile", "-NonInteractive", "-Command"])

    def test_process_counts_are_bounded_and_do_not_expose_paths_or_child_fields(self):
        observed = gate.observed_process_counts(dict(PROCESSES, helpers=[{"ExecutablePath": "secret-token"}], command="secret-token"))
        self.assertEqual(observed, {"state": "observed", **PROCESSES["counts"]})
        self.assertNotIn("secret-token", json.dumps(observed))
        for bad in (None, {}, {"counts": []}, {"counts": dict(PROCESSES["counts"], extra="secret-token")},
                    {"counts": dict(PROCESSES["counts"], **{"wsl.exe": True})},
                    {"counts": dict(PROCESSES["counts"], **{"wsl.exe": -1})},
                    {"counts": dict(PROCESSES["counts"], **{"wsl.exe": 4097})}):
            self.assertEqual(gate.observed_process_counts(bad), {"state": "unavailable"})

    def test_failed_diagnostic_preserves_original_worker_failure(self):
        with patch.object(gate, "read_json", side_effect=[{"operation": OP, "state": "failed"}, RuntimeError("private diagnostic detail")]), patch.dict(gate.os.environ, {"SystemRoot": r"C:\Windows"}), patch("builtins.print") as output:
            with self.assertRaisesRegex(RuntimeError, "Reclamation failed; retain recorded failure"):
                gate.wait_for_worker(Path("fixture-haco-wsl.exe"), OP, OP)
            self.assertNotIn("private diagnostic detail", str(output.call_args_list))

    def test_worker_completion_between_status_and_process_observation(self):
        complete = {"operation": OP, "state": "complete", "linux_started": True,
                    "linux": {"incus_btrfs_loop": {"status": "complete"}, "wsl_ext4": {"status": "complete"}},
                    "observation": {"StopAttempted": True, "StopRequested": True, "ResumeAttempted": True, "Resumed": True,
                                    "Compaction": {"Attempted": True, "Completed": True, "Virtual": {"Capacity": 1024}}}}
        # The worker publishes completion and exits after the first status read.
        for snapshot in (PROCESSES, {"helpers": [], "counts": {"unknown": "private"}}):
            observations = [{"operation": OP, "state": "pending"}, snapshot, complete]
            with self.subTest(snapshot=snapshot), patch.object(gate, "read_json", side_effect=observations) as read, patch.dict(gate.os.environ, {"SystemRoot": r"C:\Windows"}), patch("builtins.print"):
                gate.wait_for_worker(Path("fixture-haco-wsl.exe"), OP, OP)
                self.assertEqual(read.call_count, 3)
                self.assertEqual(read.call_args.args[0][1:], ["_status", OP, OP])

    def test_absent_worker_requires_fresh_complete_result(self):
        for final in ({"operation": OP, "state": "pending"},
                      {"operation": OP, "state": "failed"},
                      {"operation": OP, "state": "complete"},
                      {"operation": "foreign", "state": "complete"}):
            statuses = iter([{"operation": OP, "state": "pending"}, final])
            def read_response(command):
                return next(statuses) if command[1] == "_status" else PROCESSES
            with self.subTest(final=final), patch.object(gate, "read_json", side_effect=read_response) as read, patch.dict(gate.os.environ, {"SystemRoot": r"C:\Windows"}), patch("builtins.print"):
                with self.assertRaises(RuntimeError):
                    gate.wait_for_worker(Path("fixture-haco-wsl.exe"), OP, OP)
                self.assertEqual(sum(call.args[0][1] == "_status" for call in read.call_args_list), 2)

    def test_failure_summary_does_not_emit_child_secrets(self):
        result = {"state": "failed", "linux_started": True, "linux": {"failure": "secret-token", "incus_btrfs_loop": {"status": "failed"}}, "observation": {"StopAttempted": False, "Resumed": "secret-token", "Failure": "secret-token", "NativeError": "secret-token"}, "credentials": "secret-token"}
        summary = gate.failure_summary(result)
        self.assertNotIn("secret-token", json.dumps(summary))
        self.assertEqual(summary["linux_pool"], "failed")
        self.assertIsNone(summary["windows_resumed"])
        self.assertEqual(gate.failure_summary(None), {"worker_result": "unrecognized"})

    def test_resume_failure_projection_is_bounded_and_private(self):
        self.assertEqual(gate.resume_failure_summary({"Kind": "exit", "Code": 0x8000ffff}), {"kind": "exit", "code": 0x8000ffff})
        for value in ({"Kind": "private-token"}, {"Kind": "exit", "Code": True}, {"Kind": "exit", "Code": -1}, {"Kind": "exit", "Code": 2**32}, {"Kind": "timeout", "Code": 5}, {"Kind": "exit", "Code": 0}):
            self.assertEqual(gate.resume_failure_summary(value), {"kind": "unrecognized"})
        self.assertEqual(gate.resume_failure_summary({"Kind": "timeout", "secret": "private-token"}), {"kind": "timeout"})
        self.assertEqual(gate.resume_failure_summary(None), {"kind": "unrecorded"})

    def test_all_stages_are_required(self):
        result = {"operation": OP, "state": "complete", "linux_started": True,
                  "linux": {"incus_btrfs_loop": {"status": "complete"}, "wsl_ext4": {"status": "complete"}},
                  "observation": {"StopAttempted": True, "StopRequested": True, "ResumeAttempted": True, "Resumed": True,
                    "Compaction": {"Attempted": True, "Completed": True, "Virtual": {"Capacity": 1024}}}}
        self.assertTrue(gate.require_complete(result, OP))
        result["observation"]["Resumed"] = False
        with self.assertRaises(RuntimeError):
            gate.require_complete(result, OP)


if __name__ == "__main__":
    unittest.main()

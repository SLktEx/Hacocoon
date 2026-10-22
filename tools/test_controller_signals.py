#!/usr/bin/env python3
"""Pure harness regressions; these do not establish shipped-process acceptance."""

import os
from pathlib import Path
import signal
import socket
import stat
import struct
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "test" / "e2e"))
import controller_signals as signals


def endpoint_info(inode=123, mode=stat.S_IFSOCK | 0o600, uid=None):
    return os.stat_result((mode, inode, 1, 1, os.geteuid() if uid is None else uid,
                           os.getegid(), 0, 0, 0, 0))


class ReadinessTests(unittest.TestCase):
    def observe(self, response, *, peer=42, before=None, after=None):
        connection = mock.MagicMock()
        connection.recv.side_effect = [response, b""]
        connection.getsockopt.return_value = struct.pack("3i", peer, os.geteuid(), os.getegid())
        path = mock.Mock()
        path.lstat.side_effect = [before or endpoint_info(), after or endpoint_info()]
        with mock.patch.object(signals.socket, "socket") as create, \
                mock.patch.object(signals.time, "monotonic", return_value=0):
            create.return_value.__enter__.return_value = connection
            result = signals.ping(path, 1, 42)
        connection.settimeout.assert_called_with(1)
        connection.sendall.assert_called_once_with(b'{"version":1,"method":"system.ping"}\n')
        return result

    def test_ping_requires_exact_protocol_response_and_owned_peer(self):
        self.assertEqual(self.observe(b'{"version":1,"payload":{"protocol_version":1}}\n').st_ino, 123)
        for response in (b"", b"private malformed data", b'{"version":1}',
                         b'{"version":1,"payload":{"protocol_version":2}}',
                         b'{"version":true,"payload":{"protocol_version":1}}',
                         b'{"version":1,"payload":{"protocol_version":true}}',
                         b'{"version":1,"version":1,"payload":{"protocol_version":1}}',
                         b'{"version":1,"payload":{"protocol_version":1},"error":{}}',
                         b"x" * 4097):
            with self.subTest(response=response[:30]), self.assertRaises(AssertionError) as caught:
                self.observe(response)
            self.assertNotIn("private malformed data", str(caught.exception))
        with self.assertRaisesRegex(AssertionError, "different process"):
            self.observe(b'{"version":1,"payload":{"protocol_version":1}}', peer=99)
        replaced = endpoint_info(inode=456)
        with self.assertRaisesRegex(AssertionError, "changed during"):
            self.observe(b'{"version":1,"payload":{"protocol_version":1}}', after=replaced)

    def test_mode_is_checked_after_readiness_not_during_bind_chmod_window(self):
        self.observe(b'{"version":1,"payload":{"protocol_version":1}}',
                     before=endpoint_info(mode=stat.S_IFSOCK | 0o755))
        shared = endpoint_info(mode=stat.S_IFSOCK | 0o755)
        with self.assertRaisesRegex(AssertionError, "private socket"):
            self.observe(b'{"version":1,"payload":{"protocol_version":1}}',
                         after=shared)

    def test_non_socket_symlink_shared_or_foreign_endpoint_is_refused(self):
        for info in (endpoint_info(mode=stat.S_IFREG | 0o600),
                     endpoint_info(mode=stat.S_IFLNK | 0o777),
                     endpoint_info(mode=stat.S_IFSOCK | 0o666),
                     endpoint_info(uid=os.geteuid() + 1)):
            path = mock.Mock(lstat=mock.Mock(return_value=info))
            with self.subTest(info=info), self.assertRaisesRegex(AssertionError, "private socket"):
                signals.private_endpoint(path)

    def test_trickled_response_cannot_extend_the_ping_deadline(self):
        connection = mock.MagicMock()
        connection.getsockopt.return_value = struct.pack("3i", 42, os.geteuid(), os.getegid())
        connection.recv.return_value = b"x"
        path = mock.Mock(lstat=mock.Mock(return_value=endpoint_info()))
        with mock.patch.object(signals.socket, "socket") as create, \
                mock.patch.object(signals.time, "monotonic", side_effect=[0, 0, 0, 0.5, 2]):
            create.return_value.__enter__.return_value = connection
            with self.assertRaisesRegex(AssertionError, "deadline"):
                signals.ping(path, 1, 42)
        connection.recv.assert_called_once()

    def test_missing_endpoint_retries_but_never_counts_as_readiness(self):
        process = mock.Mock(pid=42, poll=mock.Mock(return_value=None))
        path = Path("unused")
        with mock.patch.object(signals, "ping", side_effect=[FileNotFoundError(), ConnectionRefusedError(), "ready"]) as ping, \
                mock.patch.object(signals.time, "sleep"):
            self.assertEqual(signals.wait_ready(process, path), "ready")
            self.assertEqual(ping.call_count, 3)
        with mock.patch.object(signals, "ping", side_effect=FileNotFoundError()), \
                mock.patch.object(signals.time, "monotonic", side_effect=[0, 0, 6]), \
                mock.patch.object(signals.time, "sleep"):
            with self.assertRaisesRegex(AssertionError, "deadline"):
                signals.wait_ready(process, path)

    def test_exited_process_and_nonretryable_ping_fail_immediately(self):
        process = mock.Mock(pid=42, poll=mock.Mock(return_value=0))
        path = Path("unused")
        with mock.patch.object(signals, "ping") as ping:
            with self.assertRaisesRegex(AssertionError, "before readiness"):
                signals.wait_ready(process, path)
            ping.assert_not_called()
        process.poll.return_value = None
        for error in (AssertionError("protocol refusal"), socket.timeout(), PermissionError()):
            with self.subTest(error=type(error)), mock.patch.object(signals, "ping", side_effect=error) as ping:
                with self.assertRaises(type(error)):
                    signals.wait_ready(process, path)
                ping.assert_called_once()


class ShutdownTests(unittest.TestCase):
    def setUp(self):
        kill = mock.patch.object(signals.os, "kill")
        self.kill = kill.start()
        self.addCleanup(kill.stop)

    def test_only_zero_exit_after_each_requested_signal_passes(self):
        for signum in (signal.SIGINT, signal.SIGTERM):
            self.kill.reset_mock()
            process = mock.Mock(pid=42, poll=mock.Mock(return_value=None), wait=mock.Mock(return_value=0))
            signals.graceful_shutdown(process, signum)
            self.kill.assert_called_once_with(42, signum)
            process.send_signal.assert_not_called()
            process.wait.assert_called_once_with(timeout=5)
            process.kill.assert_not_called()
            for code in (1, -signum, -signal.SIGKILL):
                process.wait.return_value = code
                with self.assertRaisesRegex(AssertionError, "expected 0"):
                    signals.graceful_shutdown(process, signum)

    def test_early_exit_or_timeout_does_not_pass(self):
        process = mock.Mock(pid=42, poll=mock.Mock(return_value=0))
        with self.assertRaisesRegex(AssertionError, "before the requested signal"):
            signals.graceful_shutdown(process, signal.SIGTERM)
        process.send_signal.assert_not_called()
        self.kill.assert_not_called()
        process.poll.return_value = None
        process.wait.side_effect = subprocess.TimeoutExpired("controller", 5)
        with self.assertRaisesRegex(AssertionError, "did not exit"):
            signals.graceful_shutdown(process, signal.SIGTERM)

    def test_signal_cannot_be_silently_skipped_by_a_second_poll(self):
        process = mock.Mock(pid=42, poll=mock.Mock(side_effect=[None, 0]),
                            wait=mock.Mock(return_value=0))
        # Use the real Popen method: it would consume the second poll, discover
        # exit0, and return without calling os.kill. The harness must bypass it.
        process.send_signal = subprocess.Popen.send_signal.__get__(process)
        signals.graceful_shutdown(process, signal.SIGTERM)
        self.kill.assert_called_once_with(42, signal.SIGTERM)
        process.poll.assert_called_once_with()
        process.poll.side_effect = None
        process.poll.return_value = None
        self.kill.side_effect = ProcessLookupError()
        with self.assertRaisesRegex(AssertionError, "before the requested signal"):
            signals.graceful_shutdown(process, signal.SIGTERM)

    def test_cleanup_is_exact_bounded_and_reports_forced_or_unknown(self):
        process = mock.Mock(poll=mock.Mock(return_value=None))
        self.assertTrue(signals.reap_failed_process(process))
        process.kill.assert_called_once_with()
        process.wait.assert_called_once_with(timeout=5)
        process.wait.side_effect = subprocess.TimeoutExpired("controller", 5)
        with self.assertRaisesRegex(AssertionError, "could not be reaped"):
            signals.reap_failed_process(process)
        process.poll.return_value = 1
        process.reset_mock()
        self.assertFalse(signals.reap_failed_process(process))
        process.kill.assert_not_called()


class FixtureTests(unittest.TestCase):
    def prepare(self, root):
        for name in ("home", "data", "data/run", "data/run/git"):
            (root / name).mkdir(mode=0o700)
        contents = b"retained fixture data\n"
        paths = [root / "retain", root / "home/retain", root / "data/retain"]
        for path in paths:
            path.write_bytes(contents)
            path.chmod(0o600)
        retained = {path: signals.metadata(path) for path in [root / "home", *paths]}
        return retained, contents

    def test_retained_data_and_exact_endpoint_absence_are_required(self):
        for mutation in ("none", "endpoint", "dangling-endpoint", "bytes", "replacement", "extra", "permissions"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                retained, contents = self.prepare(root)
                if mutation == "endpoint":
                    (root / "control.sock").write_text("replacement")
                elif mutation == "dangling-endpoint":
                    (root / "control.sock").symlink_to(root / "absent")
                elif mutation == "bytes":
                    (root / "data/retain").write_bytes(b"changed")
                elif mutation == "replacement":
                    old = root / "data/retain"
                    old.rename(root / "data/old")
                    old.write_bytes(contents)
                    old.chmod(0o600)
                    (root / "data/old").unlink()
                elif mutation == "extra":
                    (root / "data/policy.json").write_text("unexpected")
                elif mutation == "permissions":
                    (root / "data/run/git").chmod(0o755)
                if mutation == "none":
                    signals.check_retained(root, retained, contents)
                else:
                    with self.assertRaises(AssertionError):
                        signals.check_retained(root, retained, contents)

    def test_allowlisted_environment_and_primary_failure_survive_cleanup(self):
        binary = Path("/trusted/controller")
        for cleanup_fails in (False, True):
            with self.subTest(cleanup_fails=cleanup_fails), tempfile.TemporaryDirectory() as parent:
                root = Path(parent) / "fixture"
                root.mkdir(mode=0o700)
                process = mock.Mock()
                primary = AssertionError("primary readiness failure")
                cleanup = mock.Mock(side_effect=AssertionError("cleanup failure") if cleanup_fails else None)
                with mock.patch.dict(os.environ, {"HACO_ROOT": "/not-owned", "AWS_SECRET_ACCESS_KEY": "private"}), \
                        mock.patch.object(signals.tempfile, "mkdtemp", return_value=str(root)), \
                        mock.patch.object(signals.subprocess, "Popen", return_value=process) as start, \
                        mock.patch.object(signals, "wait_ready", side_effect=primary), \
                        mock.patch.object(signals, "reap_failed_process", cleanup):
                    with self.assertRaises(AssertionError) as caught:
                        signals.run_case(binary, signal.SIGTERM)
                self.assertIs(caught.exception, primary)
                env = start.call_args.kwargs["env"]
                self.assertEqual(set(env), {"PATH", "HOME", "LC_ALL", "HACO_ROOT", "HACO_CONTROL_SOCKET",
                                            "HACO_UI_LANGUAGE", "HACO_LOG_LEVEL", "HACO_LOG_FORMAT"})
                self.assertEqual(env["PATH"], "")
                self.assertEqual(env["HOME"], str(root / "home"))
                self.assertEqual(env["HACO_ROOT"], str(root / "data"))
                self.assertEqual(env["HACO_CONTROL_SOCKET"], str(root / "control.sock"))
                self.assertEqual(start.call_args.args[0], ["/trusted/controller"])
                self.assertEqual(start.call_args.kwargs["cwd"], root / "home")
                self.assertTrue(start.call_args.kwargs["start_new_session"])
                cleanup.assert_called_once_with(process)
                self.assertEqual(root.exists(), cleanup_fails)
                self.assertEqual(bool(getattr(primary, "__notes__", [])), cleanup_fails)

    def test_forced_or_failed_fixture_cleanup_cannot_turn_into_success(self):
        binary = Path("/trusted/controller")
        for mode in ("forced", "unreaped", "files"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as parent:
                root = Path(parent) / "fixture"
                root.mkdir(mode=0o700)
                process = mock.Mock()
                with mock.patch.object(signals.tempfile, "mkdtemp", return_value=str(root)), \
                        mock.patch.object(signals.subprocess, "Popen", return_value=process), \
                        mock.patch.object(signals, "wait_ready", return_value=endpoint_info()), \
                        mock.patch.object(signals, "private_endpoint", return_value=endpoint_info()), \
                        mock.patch.object(signals, "graceful_shutdown"), \
                        mock.patch.object(signals, "check_retained"), \
                        mock.patch.object(signals, "reap_failed_process", return_value=mode == "forced",
                                          side_effect=AssertionError("unreaped child") if mode == "unreaped" else None), \
                        mock.patch.object(signals.shutil, "rmtree", side_effect=OSError("files not removed") if mode == "files" else None) as remove:
                    with self.assertRaisesRegex(AssertionError, {
                            "forced": "forced cleanup", "unreaped": "unreaped child", "files": "fixture cleanup failed"}[mode]):
                        signals.run_case(binary, signal.SIGTERM)
                    if mode == "unreaped":
                        remove.assert_not_called()

    def test_endpoint_replaced_after_readiness_prevents_signal(self):
        binary = Path("/trusted/controller")
        with tempfile.TemporaryDirectory() as parent:
            root = Path(parent) / "fixture"
            root.mkdir(mode=0o700)
            with mock.patch.object(signals.tempfile, "mkdtemp", return_value=str(root)), \
                    mock.patch.object(signals.subprocess, "Popen"), \
                    mock.patch.object(signals, "wait_ready", return_value=endpoint_info()), \
                    mock.patch.object(signals, "private_endpoint", return_value=endpoint_info(inode=456)), \
                    mock.patch.object(signals, "reap_failed_process", return_value=True), \
                    mock.patch.object(signals, "graceful_shutdown") as stop:
                with self.assertRaisesRegex(AssertionError, "changed before shutdown"):
                    signals.run_case(binary, signal.SIGTERM)
                stop.assert_not_called()

    def test_replaced_fixture_root_is_retained_after_primary_failure(self):
        binary = Path("/trusted/controller")
        with tempfile.TemporaryDirectory() as parent:
            root = Path(parent) / "fixture"
            root.mkdir(mode=0o700)
            primary = AssertionError("primary failure")

            def replace(*_):
                root.rename(root.with_name("original"))
                root.mkdir(mode=0o700)
                (root / "retain").write_text("replacement data")
                raise primary

            with mock.patch.object(signals.tempfile, "mkdtemp", return_value=str(root)), \
                    mock.patch.object(signals.subprocess, "Popen"), \
                    mock.patch.object(signals, "wait_ready", side_effect=replace), \
                    mock.patch.object(signals, "reap_failed_process", return_value=False):
                with self.assertRaises(AssertionError) as caught:
                    signals.run_case(binary, signal.SIGTERM)
            self.assertIs(caught.exception, primary)
            self.assertEqual((root / "retain").read_text(), "replacement data")
            self.assertTrue(root.with_name("original").is_dir())
            self.assertIn("cleanup also failed", primary.__notes__[0])


if __name__ == "__main__":
    unittest.main()

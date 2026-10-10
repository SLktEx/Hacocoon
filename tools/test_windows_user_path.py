#!/usr/bin/env python3
"""Component regressions for read-only Windows user-path assertions."""
import importlib.util
from contextlib import contextmanager, ExitStack, redirect_stdout
import io
import json
import queue
from pathlib import Path
import sys
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

spec = importlib.util.spec_from_file_location("windows_user_path", Path(__file__).resolve().parents[1] / "test/e2e/windows/install.py")
gate = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = gate
spec.loader.exec_module(gate)

tunnel_spec = importlib.util.spec_from_file_location("windows_tunnel_path", Path(__file__).resolve().parents[1] / "test/e2e/windows/tunnel.py")
tunnel = importlib.util.module_from_spec(tunnel_spec)
tunnel_spec.loader.exec_module(tunnel)


class TunnelExitTest(unittest.TestCase):
    @contextmanager
    def journey(self, chunks):
        self.application = Mock(stdin=io.BytesIO(), stdout=io.BytesIO(b"12345\n"))
        self.application.poll.return_value = None
        def wait(*, timeout):
            self.assertEqual(timeout, 15)
            self.application.poll.return_value = 0
            return 0
        self.application.wait.side_effect = wait
        self.terminal = SimpleNamespace(proc=Mock(), write=Mock(), output="", run=Mock(side_effect=self.drive))
        self.terminal.proc.isalive.return_value = True
        self.driver = SimpleNamespace(TerminalProcess=Mock(return_value=self.terminal),
                                      cmd_prompt_count=gate.cmd_prompt_count)
        driver_spec = SimpleNamespace(name="tunnel_terminal", loader=Mock())
        self.output = io.StringIO()
        self.chunks = chunks
        self.observations = []
        self.native_owner = Mock()
        self.exchange = Mock()
        self.cleanup = Mock(side_effect=OSError("closed"))
        replacements = (
            patch.object(tunnel, "os", SimpleNamespace(name="nt")),
            patch.object(tunnel.shutil, "which", return_value="pwsh"),
            patch.object(tunnel.subprocess, "Popen", return_value=self.application),
            patch.object(tunnel.importlib.util, "spec_from_file_location", return_value=driver_spec),
            patch.object(tunnel.importlib.util, "module_from_spec", return_value=self.driver),
            patch.object(tunnel, "assert_native_owner", self.native_owner),
            patch.object(tunnel, "exchange", self.exchange),
            patch.object(tunnel.socket, "create_connection", self.cleanup),
            patch.object(sys, "argv", ["tunnel.py", "--env", "fixture"]),
            patch.dict(sys.modules),
        )
        with ExitStack() as stack:
            for replacement in replacements:
                stack.enter_context(replacement)
            yield

    def drive(self, *, on_output, timeout):
        self.assertEqual(timeout, 180)
        def consume(chunk):
            # Exercise the maintained terminal's accumulation and normalization,
            # including CRLF/OSC and read boundaries, before the real callback.
            gate.TerminalProcess._consume(self.terminal, chunk, [], on_output)
        consume("C:\\runner> ")
        consume("\r\nroot@haco-host:~# ")
        command = self.terminal.write.call_args.args[0].rstrip("\r")
        consume(command + "\r\nListening at 127.0.0.1:45678 (TCP)\r\n")
        self.assertEqual(self.terminal.write.call_args.args, ("\x03",))
        self.native_owner.assert_called_once_with(45678)
        self.assertEqual(self.exchange.call_count, 8)
        self.application.wait.assert_called_once_with(timeout=15)
        for chunk in self.chunks:
            consume(chunk)
            self.observations.append(self.terminal.write.call_count)
        if self.terminal.write.call_args.args != ("exit\r",):
            raise AssertionError("waited past the reported tunnel failure")
        consume("\r\nC:\\runner> ")
        self.terminal.proc.isalive.return_value = False

    def run_tunnel(self):
        with redirect_stdout(self.output):
            tunnel.main()

    def assert_failed_cleanup(self):
        self.terminal.proc.terminate.assert_called_once_with(force=True)
        self.driver.TerminalProcess.assert_called_once_with()
        self.terminal.run.assert_called_once()
        self.assertTrue(self.application.stdin.closed)
        self.assertTrue(self.application.stdout.closed)
        self.assertNotIn("CTRL+C CLEANUP: PASS", self.output.getvalue())

    def test_nonzero_exit_fails_on_complete_marker_without_waiting(self):
        for code in (1, 17, 130, 255):
            with self.subTest(code=code), self.journey([f"\r\nTUNNEL-EXIT:{code}\r\n", "unreachable"]):
                # A failure must interrupt this call, not wait for another read
                # or for the 180-second terminal timeout.
                with self.assertRaisesRegex(RuntimeError, f"exited with code {code} after Ctrl\\+C"):
                    self.run_tunnel()
                self.cleanup.assert_not_called()
                self.assertEqual(self.terminal.write.call_count, 3)
                self.assert_failed_cleanup()

    def test_every_marker_chunk_boundary_preserves_the_complete_status(self):
        for code in (0, 1, 130, 255):
            marker = f"TUNNEL-EXIT:{code}\r\n"
            for split in range(len(marker) + 1):
                chunks = ["\r\n" + marker[:split], marker[split:]]
                with self.subTest(code=code, split=split), self.journey(chunks):
                    if code:
                        with self.assertRaisesRegex(RuntimeError, f"exited with code {code} after Ctrl\\+C"):
                            self.run_tunnel()
                        self.assertEqual(self.observations, [] if split == len(marker) else [3])
                        self.cleanup.assert_not_called()
                        self.assert_failed_cleanup()
                    else:
                        self.run_tunnel()
                        self.assertEqual(self.observations, [4, 4] if split == len(marker) else [3, 4])
                        self.cleanup.assert_called_once_with(("127.0.0.1", 45678), timeout=2)

    def test_split_nonzero_marker_waits_for_all_digits_and_newline(self):
        chunks = ["\r\nTUNNEL-EX", "IT:1", "30", "\r", "\n", "unreachable"]
        with self.journey(chunks):
            with self.assertRaisesRegex(RuntimeError, "exited with code 130 after Ctrl\\+C"):
                self.run_tunnel()
            self.assertEqual(self.observations, [3, 3, 3, 3])
            self.cleanup.assert_not_called()
            self.assert_failed_cleanup()

    def test_only_complete_ascii_zero_output_allows_normal_exit(self):
        chunks = [
            "\r\nroot@haco-host:~# printf 'TUNNEL-EXIT:0\\n'\r\n",
            " TUNNEL-EXIT:0\r\n",
            "TUNNEL-EXIT:0 unexpected\r\n",
            "TUNNEL-EXIT:00\r\n",
            "TUNNEL-EXIT:+0\r\n",
            "TUNNEL-EXIT:-1\r\n",
            "TUNNEL-EXIT:０\r\n",
            "TUNNEL-EXIT:١\r\n",
            "TUNNEL-EXIT:256\r\n",
            "TUNNEL-EXIT:999\r\n",
            "TUNNEL-EXIT:1234\r\n",
            "TUNNEL-EXIT:0\x00\r\n",
            "\x1b]3008;type=command\x07TUNNEL-EX", "IT:0", "\r", "\n",
        ]
        with self.journey(chunks):
            self.run_tunnel()
            self.assertEqual(self.observations, [3] * (len(chunks) - 1) + [4])
            self.cleanup.assert_called_once_with(("127.0.0.1", 45678), timeout=2)
            self.assertEqual([call.args[0] for call in self.terminal.write.call_args_list][-3:],
                             ["\x03", "exit\r", "exit\r\n"])
            self.terminal.proc.terminate.assert_not_called()
            self.assertIn("CTRL+C CLEANUP: PASS", self.output.getvalue())

    def test_delayed_suffix_cannot_turn_a_partial_marker_into_a_receipt(self):
        for prefix in ("0", "1"):
            for suffix in ("23", "234", "unexpected", "\x00", "０"):
                chunks = [f"\r\nTUNNEL-EXIT:{prefix}", suffix, "\r\n", "TUNNEL-EXIT:0\r\n"]
                with self.subTest(prefix=prefix, suffix=suffix), self.journey(chunks):
                    if prefix == "1" and suffix == "23":
                        with self.assertRaisesRegex(RuntimeError, "exited with code 123 after Ctrl\\+C"):
                            self.run_tunnel()
                        self.assertEqual(self.observations, [3, 3])
                        self.cleanup.assert_not_called()
                        self.assert_failed_cleanup()
                    else:
                        self.run_tunnel()
                        self.assertEqual(self.observations, [3, 3, 3, 4])

    def test_zero_still_fails_if_listener_survives(self):
        with self.journey(["\r\nTUNNEL-EXIT:0\r\n"]):
            connection = Mock()
            self.cleanup.side_effect = None
            self.cleanup.return_value = connection
            with self.assertRaisesRegex(RuntimeError, "Windows listener survived Ctrl\\+C"):
                self.run_tunnel()
            connection.close.assert_called_once_with()
            self.assertEqual(self.terminal.write.call_count, 3)
            self.assert_failed_cleanup()


    def receipts(self):
        records = []
        for line in self.output.getvalue().splitlines():
            start = line.find('{"component": "ci", "operation": "windows_tunnel_entry"')
            if start >= 0:
                records.append(json.loads(line[start:]))
        return records

    def test_teardown_receipts_follow_existing_success_sequence(self):
        with self.journey(["\r\nTUNNEL-EXIT:0\r\n"]):
            self.run_tunnel()
            records = self.receipts()
            phases = [record["phase"] for record in records]
            expected = ["exchange_completed", "ctrl_c_sent", "tunnel_exit_received", "listener_connect_failed",
                        "host_exit_sent", "host_returned", "terminal_exit_sent", "terminal_run_finished",
                        "cleanup_start", "terminal_cleanup", "application_observer_join_returned",
                        "cleanup_completed", "cleanup_finished"]
            self.assertEqual(phases[phases.index("exchange_completed"):], expected)
            cleanup = records[phases.index("terminal_cleanup")]
            self.assertIs(cleanup["terminal_alive"], False)
            self.assertIs(cleanup["terminal_exit_known"], False)
            self.assertIsNone(cleanup["terminal_exit_code"])
            self.assertFalse(records[-3]["application_observer_alive"])
            self.terminal.proc.terminate.assert_not_called()

    def test_failed_connect_receipt_does_not_claim_listener_absence(self):
        for failure in (ConnectionRefusedError(), TimeoutError(), PermissionError()):
            with self.subTest(failure=type(failure).__name__), self.journey(["\r\nTUNNEL-EXIT:0\r\n"]):
                self.cleanup.side_effect = failure
                self.run_tunnel()
                phases = [record["phase"] for record in self.receipts()]
                self.assertIn("listener_connect_failed", phases)
                self.assertNotIn("listener_closed", phases)

    def test_force_termination_result_is_observed_without_changing_failure(self):
        for result in (True, False, "PRIVATE"):
            with self.subTest(result=result), self.journey(["\r\nTUNNEL-EXIT:17\r\n"]):
                self.terminal.proc.terminate.return_value = result
                with self.assertRaisesRegex(RuntimeError, "exited with code 17"):
                    self.run_tunnel()
                records = {record["phase"]: record for record in self.receipts()}
                self.assertIn("terminal_terminate_start", records)
                expected = result if type(result) is bool else None
                self.assertIs(records["terminal_terminate_returned"]["termination_returned"], expected)
                self.assertIn("cleanup_completed", records)
                self.assertNotIn("PRIVATE", self.output.getvalue())
                self.assert_failed_cleanup()

    def test_diagnostic_serialization_failure_never_changes_success_or_primary(self):
        for code in (0, 17):
            with self.subTest(code=code), self.journey([f"\r\nTUNNEL-EXIT:{code}\r\n"]):
                with patch.object(tunnel.json, "dumps", side_effect=RuntimeError("PRIVATE")):
                    if code:
                        with self.assertRaisesRegex(RuntimeError, "exited with code 17"):
                            self.run_tunnel()
                    else:
                        self.run_tunnel()
                self.assertTrue(self.application.stdin.closed)
                self.assertTrue(self.application.stdout.closed)
                self.assertNotIn("PRIVATE", self.output.getvalue())

    def test_diagnostic_sink_failure_never_changes_success_or_primary(self):
        def selective_print(message, **kwargs):
            if message.startswith('{"component":'):
                raise OSError("PRIVATE sink failure")
            print(message, **kwargs)
        for code in (0, 17):
            with self.subTest(code=code), self.journey([f"\r\nTUNNEL-EXIT:{code}\r\n"]):
                with patch.object(tunnel, "print", side_effect=selective_print, create=True):
                    if code:
                        with self.assertRaisesRegex(RuntimeError, "exited with code 17"):
                            self.run_tunnel()
                    else:
                        self.run_tunnel()
                self.assertTrue(self.application.stdout.closed)
                self.assertNotIn("PRIVATE", self.output.getvalue())

    def test_actual_cleanup_failure_keeps_existing_propagation_and_order(self):
        with self.journey(["\r\nTUNNEL-EXIT:17\r\n"]):
            cleanup_error = RuntimeError("PRIVATE cleanup failure")
            self.terminal.proc.terminate.side_effect = cleanup_error
            with self.assertRaises(RuntimeError) as caught:
                self.run_tunnel()
            self.assertIs(caught.exception, cleanup_error)
            self.assertIn("exited with code 17", str(caught.exception.__context__))
            # Arming already closed stdin; failed terminal cleanup still skips stdout.
            self.assertTrue(self.application.stdin.closed)
            self.assertFalse(self.application.stdout.closed)
            phases = [record["phase"] for record in self.receipts()]
            self.assertIn("terminal_terminate_start", phases)
            self.assertNotIn("terminal_terminate_returned", phases)
            self.assertNotIn("cleanup_completed", phases)
            self.assertEqual(phases[-1], "cleanup_finished")
            self.assertNotIn("PRIVATE", self.output.getvalue())

    def test_failed_path_poll_observation_cannot_replace_primary_or_skip_cleanup(self):
        with self.journey(["\r\nTUNNEL-EXIT:17\r\n"]):
            calls = []
            def poll():
                calls.append(None)
                if len(calls) == 2:
                    raise RuntimeError("PRIVATE observation failure")
                return None if len(calls) == 1 else 0
            self.application.poll.side_effect = poll
            with self.assertRaisesRegex(RuntimeError, "exited with code 17"):
                self.run_tunnel()
            self.assertEqual(len(calls), 3)
            self.assertTrue(self.application.stdout.closed)
            self.assertNotIn("PRIVATE", self.output.getvalue())


class TunnelReceiptProjectionTest(unittest.TestCase):
    def terminal(self, status=None, reader="eof", stop="reader_done"):
        return SimpleNamespace(observed_exit_status=status, reader_outcome=reader, run_stop=stop,
                               _reader=SimpleNamespace(is_alive=lambda: False))

    def test_known_unknown_and_hostile_terminal_fields(self):
        for status in (0, 23, None, True, "PRIVATE\npath/key"):
            with self.subTest(status=status):
                record = tunnel.terminal_receipt(self.terminal(status))
                known = type(status) is int
                self.assertIs(record["terminal_exit_known"], known)
                self.assertEqual(record["terminal_exit_code"], status if known else None)
                self.assertEqual(record["terminal_reader"], "eof")
                self.assertFalse(record["terminal_reader_alive"])
        record = tunnel.terminal_receipt(self.terminal(reader="PRIVATE", stop="PRIVATE"))
        self.assertEqual(record["terminal_reader"], "unknown")
        self.assertEqual(record["terminal_run_stop"], "unknown")
        self.assertNotIn("PRIVATE", json.dumps(record))

    def test_hostile_accessors_and_sink_are_noninterfering(self):
        class Hostile:
            def __getattribute__(self, name):
                raise RuntimeError("PRIVATE")
        output = io.StringIO()
        with redirect_stdout(output):
            tunnel.write_phase(0, "PRIVATE\nphase", terminal=Hostile(), observer=Hostile(), application=Hostile())
        record = json.loads(output.getvalue())
        self.assertEqual(record["phase"], "unknown")
        self.assertEqual(record["terminal_reader"], "unknown")
        self.assertNotIn("PRIVATE", output.getvalue())
        with patch("builtins.print", side_effect=RuntimeError("PRIVATE")):
            tunnel.write_phase(0, "cleanup_finished", terminal=self.terminal(0))

    def test_flush_failure_is_noninterfering(self):
        class BrokenFlush(io.StringIO):
            def flush(self):
                raise OSError("PRIVATE flush failure")
        with redirect_stdout(BrokenFlush()):
            tunnel.write_phase(0, "cleanup_finished", terminal=self.terminal(0))

    def test_reader_eof_with_live_terminal_keeps_unknown_exit_and_existing_acceptance(self):
        terminal = object.__new__(gate.TerminalProcess)
        terminal.proc = Mock(exitstatus=None)
        terminal.proc.read.side_effect = EOFError()
        terminal._queue = queue.Queue()
        terminal._reader = SimpleNamespace(is_alive=lambda: False)
        terminal.output = ""
        terminal.reader_outcome = "running"
        terminal.observed_exit_status = None
        gate.TerminalProcess._read_loop(terminal)
        self.assertEqual(terminal.run(timeout=1), "")
        record = tunnel.terminal_receipt(terminal)
        self.assertEqual(record["terminal_reader"], "eof")
        self.assertEqual(record["terminal_run_stop"], "reader_done")
        self.assertIs(record["terminal_exit_known"], False)
        terminal.proc.terminate.assert_not_called()
        terminal.proc.isalive.assert_not_called()

    def test_reader_error_is_distinct_from_eof_without_raw_receipt(self):
        terminal = object.__new__(gate.TerminalProcess)
        terminal.proc = Mock()
        terminal.proc.read.side_effect = RuntimeError("PRIVATE read failure")
        terminal._queue = queue.Queue()
        terminal._reader = SimpleNamespace(is_alive=lambda: False)
        terminal.observed_exit_status = None
        terminal.run_stop = "running"
        gate.TerminalProcess._read_loop(terminal)
        record = tunnel.terminal_receipt(terminal)
        self.assertEqual(record["terminal_reader"], "failed")
        self.assertNotIn("PRIVATE", json.dumps(record))
        self.assertIsInstance(terminal._queue.get_nowait(), str)
        self.assertIsNone(terminal._queue.get_nowait())
        self.assertTrue(terminal._queue.empty())

    def test_callback_failure_does_not_reuse_an_earlier_exit_observation(self):
        terminal = object.__new__(gate.TerminalProcess)
        terminal.proc = Mock(exitstatus=0)
        terminal._queue = queue.Queue()
        terminal._queue.put("fixture output")
        terminal._reader = SimpleNamespace(is_alive=lambda: True)
        terminal.output = ""
        terminal.reader_outcome = "running"
        terminal.observed_exit_status = 0
        primary = RuntimeError("fixed primary failure")
        with redirect_stdout(io.StringIO()), self.assertRaises(RuntimeError) as caught:
            terminal.run(timeout=1, on_output=Mock(side_effect=primary))
        self.assertIs(caught.exception, primary)
        record = tunnel.terminal_receipt(terminal)
        self.assertEqual(record["terminal_run_stop"], "callback_failed")
        self.assertFalse(record["terminal_exit_known"])
        self.assertTrue(record["terminal_reader_alive"])
        terminal.proc.terminate.assert_called_once_with(force=True)


class EnvironmentBoundaryTest(unittest.TestCase):
    def test_host_entry_error_is_terminal_without_rerun(self):
        for text in ("haco: enter trusted haco-host: internal: Host setup is busy\n",
                     "haco: enter trusted haco-host: unavailable\n"):
            with self.assertRaisesRegex(RuntimeError, "product: ordinary Host entry failed"):
                gate.reject_failed_host_entry(text)
        gate.reject_failed_host_entry("[HACO-HOST] root@haco-host:~# ")
        gate.reject_failed_host_entry("some unrelated diagnostic mentions Host setup\n")

    def test_phase_preserves_failure_and_calls_action_once(self):
        from unittest.mock import Mock
        action = Mock(side_effect=RuntimeError("test failure"))
        with patch("builtins.print") as output:
            with self.assertRaisesRegex(RuntimeError, "test failure"):
                gate.run_phase("restart", action)
        action.assert_called_once_with()
        records = [json.loads(call.args[0]) for call in output.call_args_list]
        self.assertEqual([r["state"] for r in records], ["started", "failed"])
        self.assertIn("duration_ms", records[1])

    def test_retention_flags_are_not_allowed_in_the_product_environment(self):
        for name in ("HACO_E2E_RECLAIM_RETENTION", "HACO_RECLAIM_RETENTION_MANIFEST"):
            with patch.dict(gate.os.environ, {name: "fixture"}, clear=True):
                with self.assertRaisesRegex(RuntimeError, "refuses Hacocoon environment overrides"):
                    gate.inherited_child_environment()
        with patch.dict(gate.os.environ, {"RUNNER_TEMP": "fixture-directory"}, clear=True):
            self.assertEqual(gate.inherited_child_environment(), {"RUNNER_TEMP": "fixture-directory"})


class JourneyModeTest(unittest.TestCase):
    def test_fresh_only_stops_before_restart_and_reinstall(self):
        phases = []
        def record(name, *args, **kwargs):
            phases.append(name)
        with patch.object(gate, "run_phase", side_effect=record):
            gate.run_user_journey(Path("package"), use_cached_wsl_image=True, fresh_only=True)
        self.assertEqual(phases, ["initial-install", "installed-host-assertions", "initial-host-entry"])


class InstallerCommandTest(unittest.TestCase):
    @contextmanager
    def terminal_chunks(self, chunks):
        # Use the actual read loop, accumulation, normalization and callback.
        # A further read fails immediately: no Windows host or timeout is needed
        # to prove that a live cmd.exe cannot hide an explicit BAT failure.
        terminal = object.__new__(gate.TerminalProcess)
        terminal.output = ""
        terminal.proc = Mock(exitstatus=0)
        terminal.proc.isalive.return_value = True
        terminal._queue = Mock()
        terminal._queue.get.side_effect = ["C:\\package> ", *chunks,
                                          AssertionError("waited beyond final installer result")]
        with patch.object(gate, "TerminalProcess", return_value=terminal), redirect_stdout(io.StringIO()):
            yield terminal

    def test_normal_and_cached_installs_type_the_shipped_command_once(self):
        for cached, expected in ((False, "install-windows.bat\r\n"),
                                 (True, "install-windows.bat -UseCachedWslImage\r\n")):
            writes = []

            class Terminal:
                def __init__(self, *, cwd):
                    self.cwd = cwd

                def write(self, command):
                    writes.append(command)

                def run(self, *, responders, on_output):
                    prompt = "C:\\package> "
                    on_output(prompt, self)
                    on_output(prompt, self)  # A repainted prompt must not rerun BAT.
                    complete = prompt + "\nHacocoon WSL installation complete\nHacocoon Windows installation complete.\n"
                    on_output(complete, self)
                    on_output(complete, self)
                    return complete

            with self.subTest(cached=cached), patch.object(gate, "TerminalProcess", Terminal):
                gate.run_bat(Path("package"), use_cached_wsl_image=cached)
                self.assertEqual(writes, [expected, "exit\r\n"])

    def test_incomplete_install_does_not_retry_or_exit_as_success(self):
        from unittest.mock import Mock
        terminal = Mock()

        def incomplete(*, responders, on_output):
            on_output("C:\\package> ", terminal)
            return "installation failed"

        terminal.run.side_effect = incomplete
        with patch.object(gate, "TerminalProcess", return_value=terminal):
            with self.assertRaisesRegex(RuntimeError, "BAT did not complete"):
                gate.run_bat(Path("package"))
        terminal.write.assert_called_once_with("install-windows.bat\r\n")

    def test_explicit_failure_stops_live_terminal_at_every_chunk_boundary(self):
        # The trailing space and CRLF reproduce the #756 BAT failure receipt.
        # Include signed native statuses and split ANSI/OSC wrappers as well as
        # text/digits. A newline is required before deciding the result.
        for code in (1, 37, -2147024891):
            receipt = ("\r\n\x1b]0;installer\x1b\\\x1b[31m"
                       f"Hacocoon installation failed with exit code {code}.\x1b[0m \r\n")
            for split in range(len(receipt) + 1):
                with self.subTest(code=code, split=split), self.terminal_chunks(
                    [receipt[:split], receipt[split:]]
                ) as terminal:
                    with self.assertRaisesRegex(RuntimeError, f"product: Windows installer failed with exit code {code}"):
                        gate.run_bat(Path("package"), use_cached_wsl_image=True)
                    terminal.proc.write.assert_called_once_with("install-windows.bat -UseCachedWslImage\r\n")
                    terminal.proc.terminate.assert_called_once_with(force=True)
                    self.assertEqual(terminal._queue.get.call_count, 2 if split == len(receipt) else 3)

    def test_restart_required_is_terminal_without_another_bat_or_restart(self):
        receipt = ("\r\nHacocoon installation is paused until Windows restarts. "
                   "Follow the saved continuation instructions. \r\n")
        for split in range(len(receipt) + 1):
            with self.subTest(split=split), self.terminal_chunks([receipt[:split], receipt[split:]]) as terminal:
                with self.assertRaisesRegex(RuntimeError, "requires a Windows restart.*3010"):
                    gate.run_bat(Path("package"))
                terminal.proc.write.assert_called_once_with("install-windows.bat\r\n")
                terminal.proc.terminate.assert_called_once_with(force=True)

    def test_explicit_failure_takes_precedence_over_a_later_completion_banner(self):
        with self.terminal_chunks([
            "\r\nHacocoon installation failed with exit code 1.\r\n"
            "Hacocoon WSL installation complete\r\nHacocoon Windows installation complete.\r\n"
        ]) as terminal:
            with self.assertRaisesRegex(RuntimeError, "installer failed with exit code 1"):
                gate.run_bat(Path("package"))
            terminal.proc.write.assert_called_once_with("install-windows.bat\r\n")
            terminal.proc.terminate.assert_called_once_with(force=True)

    def test_partial_echoed_or_malformed_failure_lines_do_not_reject_success(self):
        chunks = [
            "\r\nC:\\package>echo Hacocoon installation failed with exit code 1.\r\n",
            "Hacocoon installation failed with exit code 1.", " unrelated\r\n",
            "Hacocoon installation failed with exit code 1", "37", ". unexpected\r\n",
            "Hacocoon installation failed with exit code ١.\r\n",
            "Hacocoon installation failed with exit code 12345678901.\r\n",
            "Hacocoon WSL installation complete\r\n",
            "\x1b[32mHacocoon Windows installation complete.\x1b[0m\r\n", None,
        ]
        with self.terminal_chunks(chunks) as terminal:
            gate.run_bat(Path("package"))
            self.assertEqual([call.args[0] for call in terminal.proc.write.call_args_list],
                             ["install-windows.bat\r\n", "exit\r\n"])
            terminal.proc.terminate.assert_not_called()


class LanguageAssertionTest(unittest.TestCase):
    def test_only_one_exact_normalized_observation_passes(self):
        for language in ('en', 'ja'):
            gate.assert_host_language(f'HACO_HOST_UI:{language}\n', language)
            gate.assert_host_language(f'HACO_HOST_UI:{language}\r\n', language)
        for output in ('', 'HACO_HOST_UI:ja_JP\n', 'HACO_HOST_UI:en\n',
                       "root@haco-host:~# printf 'HACO_HOST_UI:ja'\n",
                       'HACO_HOST_UI:ja\nHACO_HOST_UI:ja\n'):
            with self.subTest(output=output), self.assertRaises(RuntimeError):
                gate.assert_host_language(output, 'ja')


class DoctorAssertionTest(unittest.TestCase):
    build = {"checkpoint": "v0.26", "version": "0.26.1-candidate", "commit": "candidate", "build_date": "2026-09-06T00:00:00Z"}

    def report(self):
        return {"protocol_version": 1, "controller": dict(self.build), "checks": [
            {"name": name, "status": "ok"} for name in
            ["runtime", "storage", "storage_mount", "trusted_host", "trusted_network", "trusted_connectivity"]
        ]}

    def test_accepts_complete_report(self):
        gate.assert_doctor_report(json.dumps(self.report()), self.build)

    def test_rejects_failure_skips_missing_and_duplicates(self):
        for mutation in ("failed", "skipped", "pending", "missing", "duplicate", "protocol", "identity"):
            with self.subTest(mutation=mutation):
                report = self.report()
                if mutation in ("failed", "skipped", "pending"):
                    report["checks"][4]["status"] = mutation
                elif mutation == "missing":
                    report["checks"].pop()
                elif mutation == "duplicate":
                    report["checks"][1]["name"] = report["checks"][0]["name"]
                elif mutation == "protocol":
                    report["protocol_version"] = 0
                else:
                    report["controller"]["commit"] = ""
                with self.assertRaises(RuntimeError):
                    gate.assert_doctor_report(json.dumps(report), self.build)

    def test_rejects_unstamped_or_mismatched_controller(self):
        for field, value in (("version", "dev"), ("build_date", "unknown"), ("commit", "stale"), ("checkpoint", "stale")):
            with self.subTest(field=field):
                report = self.report()
                report["controller"][field] = value
                with self.assertRaises(RuntimeError):
                    gate.assert_doctor_report(json.dumps(report), self.build)

    def test_rejects_unstamped_client_even_when_controller_matches(self):
        for field in self.build:
            with self.subTest(field=field):
                report = self.report()
                report["controller"][field] = "unknown"
                with self.assertRaises(RuntimeError):
                    gate.assert_doctor_report(json.dumps(report), report["controller"])

    def test_rejects_non_json(self):
        with self.assertRaises(json.JSONDecodeError):
            gate.assert_doctor_report("not a report", self.build)


class ProxyListenerAssertionTest(unittest.TestCase):
    listener = 'LISTEN 0 4096 169.254.254.1:18080 0.0.0.0:* users:(("haco-controller",pid=125,fd=3))'

    def test_fixed_listener_belongs_to_same_controller(self):
        gate.assert_proxy_listener("125", self.listener)

    def test_missing_wildcard_duplicate_and_foreign_listeners_fail(self):
        for pid, lines in (("0", self.listener), ("", self.listener), ("125", ""),
                           ("125", self.listener.replace("169.254.254.1", "0.0.0.0")),
                           ("125", self.listener + "\n" + self.listener),
                           ("126", self.listener), ("125", self.listener.replace("pid=125,", "pid=1250,"))):
            with self.subTest(pid=pid, lines=lines):
                with self.assertRaises(RuntimeError):
                    gate.assert_proxy_listener(pid, lines)


class TerminalNormalizationTest(unittest.TestCase):
    def test_conpty_carriage_return_does_not_hide_a_real_output_line(self):
        # Reduced from a real pywinpty 3.0.2 -> ordinary WSL -> haco-host
        # session: ConPTY emits CRLF followed by another CR before OSC 3008.
        raw = (
            "root@haco-host:~# cat ~/.hacocoon-installer-acceptance\r\n\r"
            "\x1b[?2004l\x1b]3008;type=command;cwd=/root\x1b\\"
            "kept-through-restart-and-rerun\r\n"
            "\x1b]3008;exit=success\x07"
        )
        output = gate.normalize_terminal(raw)
        gate.require_output(output, r"^kept-through-restart-and-rerun\s*$", phase="haco-host data")
        self.assertNotIn("\r", output)
        self.assertNotIn("3008;", output)

    def test_echoed_command_cannot_satisfy_the_retained_data_assertion(self):
        raw = (
            "root@haco-host:~# printf '%s\\n' kept-through-restart-and-rerun"
            " > ~/.hacocoon-installer-acceptance\r\n\r"
        )
        with self.assertRaises(RuntimeError):
            gate.require_output(gate.normalize_terminal(raw),
                                r"^kept-through-restart-and-rerun\s*$", phase="haco-host data")


if __name__ == "__main__":
    unittest.main()

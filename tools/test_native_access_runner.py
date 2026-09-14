import importlib.util
import os
from pathlib import Path
import signal
import subprocess
import sys
import time
from types import SimpleNamespace
import unittest

spec = importlib.util.spec_from_file_location('native_runner', Path(__file__).with_name('windows-native-access-e2e.py'))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)

driver_spec = importlib.util.spec_from_file_location('native_entry_driver', Path(__file__).with_name('windows-installer-user-path-e2e.py'))
entry_driver = importlib.util.module_from_spec(driver_spec)
sys.modules[driver_spec.name] = entry_driver
driver_spec.loader.exec_module(entry_driver)

stream_spec = importlib.util.spec_from_file_location('native_stream_runner', Path(__file__).with_name('windows-run-stream-e2e.py'))
stream_runner = importlib.util.module_from_spec(stream_spec)
stream_spec.loader.exec_module(stream_runner)

class NativeRunnerTests(unittest.TestCase):
    def test_cold_checks_have_no_retained_host_terminal(self):
        terminals, checks = [], []

        class Terminal:
            def __init__(self):
                self.proc = self
                self.alive = True
                self.writes = []
                terminals.append(self)

            def isalive(self):
                return self.alive

            def terminate(self, force):
                self.alive = False

            def write(self, text):
                if not self.alive:
                    raise EOFError('closed terminal')
                self.writes.append(text)

            def run(self, on_output):
                on_output('CMD>', self)
                on_output('CMD>\nroot@haco-host:~# ', self)
                on_output('CMD>\nroot@haco-host:~# \nCMD>', self)
                if self.writes != ['wsl -d Hacocoon\r\n', 'exit\r\n', 'exit\r\n']:
                    raise AssertionError('ordinary terminal was not closed normally')
                self.alive = False

        driver = SimpleNamespace(TerminalProcess=Terminal,
                                 cmd_prompt_count=lambda output: output.count('CMD>'),
                                 reject_failed_host_entry=entry_driver.reject_failed_host_entry)

        def check(name):
            active = sum(terminal.alive for terminal in terminals)
            self.assertEqual(active, 1 if name == 'test_windows_host_interop.ps1' else 0)
            checks.append(name)

        runner.run_native_checks(driver, check, host_customization=True)
        self.assertEqual(checks, ['test_windows_host_interop.ps1',
            'test_windows_environment_ssh.ps1', 'test_host_customization.ps1',
            'test_windows_host_interop.ps1'])
        self.assertEqual(len(terminals), 2)
        self.assertFalse(any(terminal.alive for terminal in terminals))

    def test_failed_entry_stops_before_checks_or_session_timeout(self):
        class Terminal:
            def __init__(self):
                self.proc = self
                self.alive = True
                self.writes = []
            def isalive(self): return self.alive
            def terminate(self, force): self.alive = False
            def write(self, text): self.writes.append(text)
            def run(self, on_output, **kwargs):
                on_output('CMD>', self)
                on_output('CMD>\nhaco: enter trusted haco-host: internal: Host setup failed: stage=notification_setup reason=failed\nCMD>', self)
                raise AssertionError('waited past the reported product failure')

        def checks(): raise AssertionError('checks started without successful entry')
        for entry in (lambda driver: runner.run_in_host_terminal(driver, checks), stream_runner.run_sequence):
            with self.subTest(entry=entry):
                terminal = Terminal()
                driver = SimpleNamespace(TerminalProcess=lambda: terminal,
                                         cmd_prompt_count=lambda output: output.count('CMD>'),
                                         reject_failed_host_entry=entry_driver.reject_failed_host_entry)
                with self.assertRaisesRegex(RuntimeError, 'product: ordinary Host entry failed'):
                    entry(driver)
                self.assertEqual(terminal.writes, ['wsl -d Hacocoon\r\n'])
                self.assertFalse(terminal.alive)

    def test_result_requires_success_and_all_markers(self):
        name = 'test_windows_environment_ssh.ps1'
        marker = 'WINDOWS DIRECT ENVIRONMENT SSH: PASS'
        for output in ('', marker):
            with self.assertRaisesRegex(RuntimeError, 'failed with exit 17'):
                runner.verify_acceptance_result(name, subprocess.CompletedProcess([], 17, output))
        with self.assertRaisesRegex(RuntimeError, 'acceptance assertions'):
            runner.verify_acceptance_result(name, subprocess.CompletedProcess([], 0, ''))
        with self.assertRaisesRegex(RuntimeError, 'Real VS Code'):
            runner.verify_acceptance_result(name, subprocess.CompletedProcess([], 0, marker), True)
        runner.verify_acceptance_result(name, subprocess.CompletedProcess([], 0, marker))
        runner.verify_acceptance_result(name, subprocess.CompletedProcess([], 0, marker + '\nVS CODE REMOTE ENVIRONMENT: PASS'), True)

    def test_capture_and_exit_status(self):
        result = runner.run_acceptance([sys.executable, '-c', "import sys;print('marker');sys.exit(17)"], timeout=5)
        self.assertEqual(result.returncode, 17)
        self.assertEqual(result.stdout.strip(), 'marker')

    def test_timeout_is_bounded(self):
        started = time.monotonic()
        with self.assertRaisesRegex(RuntimeError, 'timed out'):
            runner.run_acceptance([sys.executable, '-c', 'import time;time.sleep(30)'], timeout=0.3)
        self.assertLess(time.monotonic() - started, 10)

    def test_descendant_inherited_output_does_not_hold_parent(self):
        code = "import subprocess,sys;p=subprocess.Popen([sys.executable,'-c','import time;time.sleep(15)']);print(p.pid,flush=True)"
        started = time.monotonic()
        result = runner.run_acceptance([sys.executable, '-c', code], timeout=5)
        child = int(result.stdout.strip())
        try:
            self.assertLess(time.monotonic() - started, 10)
            self.assertEqual(result.returncode, 0)
        finally:
            if os.name == 'nt':
                subprocess.run(['taskkill', '/PID', str(child), '/T', '/F'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=5, check=False)
            else:
                os.kill(child, signal.SIGTERM)

if __name__ == '__main__':
    unittest.main()

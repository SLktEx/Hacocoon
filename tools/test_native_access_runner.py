import importlib.util
import os
from pathlib import Path
import signal
import subprocess
import sys
import time
import unittest

spec = importlib.util.spec_from_file_location('native_runner', Path(__file__).with_name('windows-native-access-e2e.py'))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)

class NativeRunnerTests(unittest.TestCase):
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

"""Protocol regressions for the ordinary Windows terminal acceptance driver."""
import contextlib
import importlib.util
import io
from pathlib import Path
import re
from types import SimpleNamespace
import unittest

spec = importlib.util.spec_from_file_location('windows_run_stream', Path(__file__).with_name('windows-run-stream-e2e.py'))
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)


class SequenceTest(unittest.TestCase):
    def drive(self, input_value='abD', native_changed=False):
        writes, sizes = [], []

        class Terminal:
            def __init__(self):
                self.proc = self

            def write(self, value):
                writes.append(value)

            def setwinsize(self, rows, columns):
                sizes.append((rows, columns))

            def isalive(self):
                return False

            def run(self, on_output, timeout):
                output = ''
                observations = ['C:\\>', 'root@haco-host:~# ', 'RUN-TTY-READY\n48 160\n',
                                f'RUN-INPUT:{input_value}\nRUN-RESIZED:43 132\nRUN-EXIT:17\nRUN-RESTORED\n', 'C:\\>']
                for item in observations:
                    output += '\n' + item
                    on_output(output, self)

        count = 0

        def inspect(*args):
            nonlocal count
            count += 1
            return 'haco-host\nhaco-retained' + ('\nhaco-leaked' if native_changed and count == 2 else '')

        def require(output, pattern, phase):
            if not re.search(pattern, output, re.MULTILINE):
                raise RuntimeError('missing actual observation')

        driver = SimpleNamespace(TerminalProcess=Terminal, inspect_root=inspect,
                                 cmd_prompt_count=lambda text: text.count('C:\\>'), require_output=require)
        with contextlib.redirect_stdout(io.StringIO()):
            gate.run_sequence(driver)
        return writes, sizes

    def test_command_uses_one_enter_before_guest_input_and_real_resize(self):
        writes, sizes = self.drive()
        self.assertEqual(writes[0], 'wsl -d Hacocoon\r\n')
        self.assertEqual(writes[1].count('\r'), 1)
        self.assertEqual(writes[1].count('\n'), 0)
        self.assertTrue(writes[1].endswith('\r'))
        self.assertIn('haco run -it -- sh -ec', writes[1])
        self.assertEqual(writes[2], 'abc\x7fD\r')
        self.assertEqual(sizes, [(43, 132)])
        self.assertEqual(writes[3:], ['exit\r\n', 'exit\r\n'])

    def test_empty_guest_read_and_changed_native_ownership_cannot_pass(self):
        with self.assertRaises(RuntimeError):
            self.drive(input_value='')
        with self.assertRaises(RuntimeError):
            self.drive(native_changed=True)


if __name__ == '__main__':
    unittest.main()

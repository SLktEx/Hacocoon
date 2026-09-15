"""The Linux application fixture waits for client readiness before accept's budget."""
import concurrent.futures
import importlib.util
import os
from pathlib import Path
import socket
import subprocess
import sys
import time
import unittest

spec = importlib.util.spec_from_file_location(
    "installed_forward", Path(__file__).resolve().parents[1] / "test/e2e/installed/forward.py")
forward = importlib.util.module_from_spec(spec)
spec.loader.exec_module(forward)
arm_application, line, server_source = forward.arm_application, forward.line, forward.server_source


class ApplicationBudgetTests(unittest.TestCase):
    def test_invalid_deadlines_are_rejected_before_starting_application(self):
        for name, maximum in (("startup_seconds", 180), ("accept_seconds", 40)):
            for value in (0, -1, maximum + 1, float("inf"), float("nan")):
                with self.subTest(name=name, value=value), self.assertRaises(ValueError):
                    server_source(**{name: value})


@unittest.skipUnless(os.name == 'posix', 'application fixture runs inside Linux')
class ForwardApplicationTests(unittest.TestCase):
    def start(self, **budgets):
        process = subprocess.Popen([sys.executable, '-u', '-c', server_source(**budgets)],
                                   stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.addCleanup(self.stop, process)
        port = int(line(process, timeout=5))
        return process, port

    @staticmethod
    def stop(process):
        if process.poll() is None:
            process.kill()
            process.wait(timeout=5)
        for stream in (process.stdin, process.stdout, process.stderr):
            if stream is not None and not stream.closed:
                stream.close()

    def test_preparation_longer_than_accept_budget_preserves_eight_binary_exchanges(self):
        process, port = self.start(startup_seconds=5, accept_seconds=1)
        time.sleep(1.2)  # Reproduces preconditions consuming the old accept budget.
        self.assertIsNone(process.poll())
        arm_application(process)

        def exchange(index):
            data = bytes(range(256)) * (1024 + index) + bytes([index])
            with socket.create_connection(('127.0.0.1', port), timeout=2) as client:
                client.sendall(data)
                client.shutdown(socket.SHUT_WR)
                received = bytearray()
                while chunk := client.recv(65536):
                    received.extend(chunk)
                self.assertEqual(received, data)

        with concurrent.futures.ThreadPoolExecutor(max_workers=8) as workers:
            list(workers.map(exchange, range(8)))
        self.assertEqual(process.wait(timeout=5), 0)

    def test_missing_connections_still_fail_after_arming(self):
        process, _ = self.start(startup_seconds=5, accept_seconds=0.1)
        arm_application(process)
        self.assertNotEqual(process.wait(timeout=5), 0)

    def test_unarmed_or_abandoned_application_cannot_pass(self):
        for abandon in (False, True):
            with self.subTest(abandon=abandon):
                process, _ = self.start(startup_seconds=0.1, accept_seconds=1)
                if abandon:
                    process.stdin.close()
                self.assertNotEqual(process.wait(timeout=5), 0)


if __name__ == '__main__':
    unittest.main()

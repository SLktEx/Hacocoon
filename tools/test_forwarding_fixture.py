"""Real socket checks for the application shared by installed tunnel journeys."""
import concurrent.futures
import importlib.util
from pathlib import Path
import socket
import subprocess
import sys
import unittest

spec = importlib.util.spec_from_file_location(
    "installed_forward", Path(__file__).resolve().parents[1] / "test/e2e/installed/forward.py"
)
forward = importlib.util.module_from_spec(spec)
spec.loader.exec_module(forward)


class ApplicationFixtureTests(unittest.TestCase):
    def start(self, timeout):
        process = subprocess.Popen(
            [sys.executable, "-u", "-c", forward.SERVER, "--accept-timeout", str(timeout)],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        )
        self.addCleanup(self.stop, process)
        return process

    @staticmethod
    def stop(process):
        if process.poll() is None:
            process.kill()
        process.communicate(timeout=5)

    def test_configured_accept_timeout_terminates_unused_fixture(self):
        process = self.start(1)
        out, err = process.communicate(timeout=5)
        self.assertTrue(out.strip().isdigit(), out)
        self.assertNotEqual(process.returncode, 0)
        self.assertIn(b"TimeoutError", err)

    def test_all_concurrent_binary_half_closes_complete(self):
        process = self.start(10)
        port = int(process.stdout.readline(4096))

        def exchange(index):
            data = bytes(range(256)) * (1024 + index)
            with socket.create_connection(("127.0.0.1", port), timeout=5) as client:
                client.sendall(data)
                client.shutdown(socket.SHUT_WR)
                answer = bytearray()
                while chunk := client.recv(65536):
                    answer.extend(chunk)
            self.assertEqual(answer, data)

        with concurrent.futures.ThreadPoolExecutor(max_workers=8) as workers:
            list(workers.map(exchange, range(8)))
        _, error = process.communicate(timeout=5)
        self.assertEqual(process.returncode, 0, error)

    def test_invalid_timeout_never_announces_listener(self):
        for timeout in (0, 301, "nan", "-1"):
            with self.subTest(timeout=timeout):
                process = self.start(timeout)
                out, _ = process.communicate(timeout=5)
                self.assertNotEqual(process.returncode, 0)
                self.assertEqual(out, b"")


if __name__ == "__main__":
    unittest.main()

"""Regression for the installed approval fixture's real setup output contract."""
import unittest
from test_pending_approvals import valid_network_output


class NetworkOutputTest(unittest.TestCase):
    def test_setup_completion_is_expected(self):
        self.assertTrue(valid_network_output("PENDING_NETWORK_RESULT_OK\nProject setup completed.\n"))

    def test_missing_or_extra_results_fail(self):
        for output in ("PENDING_NETWORK_RESULT_OK\n", "Project setup completed.\n",
                       "PENDING_NETWORK_RESULT_OK\nunexpected\nProject setup completed.\n"):
            with self.subTest(output=output):
                self.assertFalse(valid_network_output(output))


if __name__ == "__main__":
    unittest.main()

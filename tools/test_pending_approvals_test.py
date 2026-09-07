"""Regression for the installed approval fixture's real setup output contract."""
import unittest
from test_pending_approvals import valid_network_output, command_failure_category


class NetworkOutputTest(unittest.TestCase):
    def test_diagnostics_never_return_arbitrary_output(self):
        self.assertEqual(command_failure_category("SECRET", "SECRET"), "command")
        self.assertEqual(command_failure_category("", "haco: cannot read a regular UTF-8 setup script SECRET"), "script-input")
        self.assertEqual(command_failure_category("", "Project setup request failed SECRET"), "controller-request")
        self.assertEqual(command_failure_category("", "Unit hacocoon-project-setup.service already exists. Project setup failed; correct the script or Environment SECRET"), "unit-busy")
        self.assertEqual(command_failure_category("PENDING_PREREQ_STARTED\nSECRET\n", ""), "package-update")
        self.assertEqual(command_failure_category("PENDING_PREREQ_UPDATED\n", ""), "package-install")
        self.assertEqual(command_failure_category("PENDING_PREREQ_READY\n", ""), "after-prerequisite")

    def test_setup_completion_is_expected(self):
        self.assertTrue(valid_network_output("PENDING_NETWORK_RESULT_OK\nProject setup completed.\n"))

    def test_missing_or_extra_results_fail(self):
        for output in ("PENDING_NETWORK_RESULT_OK\n", "Project setup completed.\n",
                       "PENDING_NETWORK_RESULT_OK\nunexpected\nProject setup completed.\n"):
            with self.subTest(output=output):
                self.assertFalse(valid_network_output(output))


if __name__ == "__main__":
    unittest.main()

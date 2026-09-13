"""Regression for the installed approval fixture's real setup output contract."""
import unittest
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from unittest.mock import patch
import test_pending_approvals as acceptance
import time
from test_pending_approvals import valid_network_output, command_failure_category, terminal_command

@unittest.skipUnless(sys.platform == "linux", "installed approval terminal is Linux")
class TerminalAnswerTest(unittest.TestCase):
    def test_terminal_answers_keep_json_and_exit_separate(self):
        script = "import sys; assert sys.stdin.isatty(); assert sys.stdin.readline() == '5\\n'; assert sys.stdin.readline() == 'n\\n'; print('{\"receipt\":true}'); print('reviewed', file=sys.stderr); sys.exit(37)"
        result = terminal_command([sys.executable, "-c", script], "5\nn\n", 5)
        self.assertEqual(result.stdout, '{"receipt":true}\n')
        self.assertEqual(result.stderr, "reviewed\n")
        self.assertEqual(result.returncode, 37)

    def test_timeout_kills_and_reaps_only_the_owned_child(self):
        started = time.monotonic()
        with self.assertRaises(subprocess.TimeoutExpired):
            terminal_command([sys.executable, "-c", "import time; time.sleep(10)"], "n\n", .1)
        self.assertLess(time.monotonic() - started, 3)


class MachineOutputTest(unittest.TestCase):
    def test_configuration_and_pending_list_request_machine_output(self):
        snapshot = {"revision": "sha256:" + "a" * 64, "policy": {"default": "deny"}}
        def cli(*args, **kwargs):
            if "--json" not in args:
                return "revision: sha256:human-output\npolicy:\n  default: deny\n"
            if args[0] == "approve":
                return '[{"request_id":"pending"}]'
            if "--file" in args:
                return Path(args[args.index("--file") + 1]).read_text()
            return json.dumps(snapshot)
        with tempfile.TemporaryDirectory() as directory, patch.object(acceptance, "command", side_effect=cli):
            acceptance.configuration_update(Path(directory), lambda policy: policy.update(default="allow"))
            self.assertEqual(acceptance.pending_requests(), [{"request_id": "pending"}])

    @unittest.skipUnless(os.name == "posix", "executes the installed Linux shell fixture")
    def test_installed_configuration_shell_handles_human_default_cli(self):
        source = Path(__file__).with_name("test_windows_environment_ssh.ps1").read_text()
        probe = re.search(r"\$configurationProbe = @'\n(.*?)\n'@", source, re.S)
        self.assertIsNotNone(probe)
        # Execute the actual embedded shell, with a CLI double that preserves
        # the shipped human-default / explicit-JSON output distinction.
        with tempfile.TemporaryDirectory() as directory:
            cli = Path(directory) / "haco"
            cli.write_text("#!" + sys.executable + '''
import json, pathlib, sys
args = sys.argv[1:]
if '--json' not in args:
    print('revision: human-output')
elif '--file' in args:
    print(pathlib.Path(args[args.index('--file') + 1]).read_text())
else:
    print(json.dumps({'revision': 'sha256:' + 'a' * 64, 'policy': {'default': 'deny'}}))
''')
            cli.chmod(0o700)
            result = subprocess.run(["bash", "-ec", probe.group(1)], capture_output=True, text=True,
                                    env=dict(os.environ, PATH=directory + os.pathsep + os.environ["PATH"]), timeout=15)
            self.assertEqual(result.returncode, 0, result.stderr)


class NetworkOutputTest(unittest.TestCase):
    def test_diagnostics_never_return_arbitrary_output(self):
        self.assertEqual(command_failure_category("SECRET", "SECRET"), "command")
        self.assertEqual(command_failure_category("", "haco: cannot read a regular UTF-8 setup script SECRET"), "script-input")
        self.assertEqual(command_failure_category("", "Project setup request failed SECRET"), "controller-request")
        self.assertEqual(command_failure_category("", "Unit hacocoon-project-setup.service already exists. Project setup failed; correct the script or Environment SECRET"), "unit-busy")
        self.assertEqual(command_failure_category("PENDING_PREREQ_STARTED\nSECRET\n", ""), "package-update")
        self.assertEqual(command_failure_category("PENDING_PREREQ_UPDATED\n", ""), "package-install")
        self.assertEqual(command_failure_category("PENDING_PREREQ_READY\n", ""), "after-prerequisite")

    def test_safe_setup_stage_survives_without_subprocess_output(self):
        self.assertEqual(command_failure_category("SECRET", "stage=start error_code=unavailable exit_code=0 SECRET"), "setup-start-unavailable")
        self.assertEqual(command_failure_category("", "stage=SECRET error_code=SECRET"), "command")

    def test_setup_completion_is_expected(self):
        self.assertTrue(valid_network_output("PENDING_NETWORK_RESULT_OK\nProject setup completed.\n"))

    def test_missing_or_extra_results_fail(self):
        for output in ("PENDING_NETWORK_RESULT_OK\n", "Project setup completed.\n",
                       "PENDING_NETWORK_RESULT_OK\nunexpected\nProject setup completed.\n"):
            with self.subTest(output=output):
                self.assertFalse(valid_network_output(output))


if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3
"""Exercise common host preparation without changing host packages or networks."""
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
INSTALLER = (ROOT / "scripts/install.sh").read_text(encoding="utf-8")
PREPARE = re.search(r"^prepare_ubuntu_host\(\) \{\n.*?^\}", INSTALLER, re.M | re.S)[0]
CONNECTIVITY = re.search(r"^verify_trusted_host_connectivity\(\) \{\n.*?^\}", INSTALLER, re.M | re.S)[0]


class InstallerNetworkTests(unittest.TestCase):
    def prepare(self, ready):
        with tempfile.TemporaryDirectory() as directory:
            trace = Path(directory) / "commands"
            script = PREPARE + r'''
set -eu
trace="$1"; ready="$2"
SUDO=privileged; SKIP_INCUS=0; GRANT_INCUS_ADMIN=0; INSTALL_UID=1000
die() { printf '%s\n' "$*" >&2; exit 1; }
assert_ubuntu() { :; }
prepare_privilege() { :; }
ensure_gh_attestation_verify() { :; }
configure_workspace_owner_idmap() { :; }
ensure_bridge_netfilter() { :; }
ensure_incus_userns_compatibility() { :; }
ps() { printf 'systemd\n'; }
incus() { :; }
privileged() {
  printf '%s\n' "$*" >> "$trace"
  case "$*" in
    'incus info') [ "$ready" = 1 ] ;;
    'incus storage '*|'incus admin '*) return 2 ;;
    *) return 0 ;;
  esac
}
prepare_ubuntu_host
'''
            result = subprocess.run(["sh", "-c", script, "sh", str(trace), str(int(ready))],
                                    capture_output=True, text=True)
            return result, trace.read_text().splitlines()

    def test_ready_daemon_needs_no_storage_probe_or_minimal_initialization(self):
        result, commands = self.prepare(True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([c for c in commands if c.startswith("incus ")], ["incus info"])
        self.assertIn("apt-get install -y incus iptables", commands)

    def test_unavailable_daemon_fails_without_initialization(self):
        result, commands = self.prepare(False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Incus daemon is not ready", result.stderr)
        self.assertEqual([c for c in commands if c.startswith("incus ")], ["incus info"])

    def test_guest_network_startup_is_bounded_without_repair(self):
        for ready_after, expected_calls, expected_status in ((3, 3, 0), (0, 10, 1)):
            script = CONNECTIVITY + r'''
SUDO=probe; calls=0; ready_after="$1"
probe() {
  calls=$((calls + 1))
  case "$*" in 'incus exec haco-host --project hacocoon -- timeout 8 /bin/sh -ec '*) ;; *) exit 99 ;; esac
  [ "$calls" = "$ready_after" ]
}
sleep() { :; }
verify_trusted_host_connectivity
status="$?"
printf '%s:%s\n' "$calls" "$status"
'''
            result = subprocess.run(["sh", "-c", script, "sh", str(ready_after)], capture_output=True, text=True)
            self.assertEqual(result.stdout.strip(), f"{expected_calls}:{expected_status}", result.stderr)


if __name__ == "__main__":
    unittest.main()

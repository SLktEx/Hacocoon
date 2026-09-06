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
        self.assertIn("apt-get install -y incus iptables nftables", commands)

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
  case "$*" in 'incus exec haco-host --project hacocoon -- env -i PATH=/usr/sbin:/usr/bin:/sbin:/bin timeout 8 /bin/sh -ec '*) ;; *) exit 99 ;; esac
  [ "$calls" = "$ready_after" ]
}
sleep() { :; }
verify_trusted_host_connectivity
status="$?"
printf '%s:%s\n' "$calls" "$status"
'''
            result = subprocess.run(["sh", "-c", script, "sh", str(ready_after)], capture_output=True, text=True)
            self.assertEqual(result.stdout.strip(), f"{expected_calls}:{expected_status}", result.stderr)



    def test_installer_bootstrap_uses_controller_and_stops_at_failure(self):
        bootstrap = INSTALLER[INSTALLER.index('\nhaco_bin='):]
        for failed_stage in ("none", "controller", "setup", "connectivity"):
            with self.subTest(failed_stage=failed_stage):
                script = r'''
set -eu
failed_stage="$1"; SUDO=privileged; GRANT_INCUS_ADMIN=0; INSTALL_UID=1000
HACOCOON_ACCESS_USER=""
die() { printf '%s\n' "$*" >&2; exit 1; }
command() {
  case "$*" in
    '-v haco') printf 'haco\n' ;;
    '-v haco-controller') printf 'haco-controller\n' ;;
    *) exit 99 ;;
  esac
}
readlink() { printf '%s\n' "$2"; }
configure_hacocoon_controller() { printf 'stage:controller\n'; [ "$failed_stage" != controller ]; }
privileged() {
  case "$*" in
    'haco setup') printf 'stage:setup\n'; [ "$failed_stage" != setup ] ;;
    'incus exec haco-host --project hacocoon -- /usr/local/bin/haco-host doctor') printf 'stage:roundtrip\n' ;;
    *) exit 99 ;;
  esac
}
verify_trusted_host_connectivity() { printf 'stage:connectivity\n'; [ "$failed_stage" != connectivity ]; }
''' + bootstrap
                result = subprocess.run(["sh", "-c", script, "sh", failed_stage], capture_output=True, text=True)
                if failed_stage == "none":
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(result.stdout.count("stage:setup"), 1)
                    self.assertIn("Hacocoon common Ubuntu installation complete.", result.stdout)
                else:
                    self.assertNotEqual(result.returncode, 0)
                    self.assertNotIn("Hacocoon common Ubuntu installation complete.", result.stdout)
                if failed_stage == "controller":
                    self.assertNotIn("stage:setup", result.stdout)
                if failed_stage == "setup":
                    self.assertNotIn("stage:connectivity", result.stdout)

    def test_installed_controller_unit_owns_standard_proxy(self):
        configure = re.search(r"^configure_hacocoon_controller\(\) \{\n.*?^\}", INSTALLER, re.M | re.S)[0]
        script = r'''
set -eu
SUDO=privileged; HACOCOON_CONTROLLER_SERVICE=haco-controller.service
HACOCOON_CONTROLLER_SOCKET=/run/hacocoon/control.sock
configure_hacocoon_access_group() { HACOCOON_ACCESS_GID=1000; }
die() { printf '%s\n' "$*" >&2; exit 1; }
privileged() {
  case "$*" in
    'stat -Lc %u /usr/local/bin/haco-controller') printf '0\n' ;;
    'stat -Lc %u:%g:%a /run/hacocoon/control.sock') printf '0:1000:660\n' ;;
    'find /usr/local/bin/haco-controller -perm /022 -print -quit') : ;;
    'install -o root -g root -m 0644 '*'/etc/systemd/system/haco-controller.service') cat "$8" ;;
    'systemctl daemon-reload'|'systemctl enable haco-controller.service'|'systemctl restart haco-controller.service'|'test -S /run/hacocoon/control.sock') : ;;
    *) printf 'Unexpected privileged command\n' >&2; exit 99 ;;
  esac
}
''' + configure + '\nconfigure_hacocoon_controller /usr/local/bin/haco-controller\n'
        result = subprocess.run(["sh", "-c", script], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("ExecStart=/usr/local/bin/haco-controller --standard-egress\n", result.stdout)
        self.assertIn("Requires=incus.service\nAfter=incus.service\n", result.stdout)
        self.assertNotIn("hacoq", result.stdout)

if __name__ == "__main__":
    unittest.main()

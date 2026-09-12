#!/usr/bin/env python3
"""Exercise the real shell identity boundary without host mutations."""

from pathlib import Path
import re
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[1]
INSTALLER = (ROOT / "scripts/install.sh").read_text(encoding="utf-8")


def function(name: str) -> str:
    match = re.search(rf"^{name}\(\) \{{\n.*?^\}}", INSTALLER, re.M | re.S)
    if match is None:
        raise AssertionError(f"missing installer function {name}")
    return match[0]


FUNCTIONS = "\n".join(function(name) for name in (
    "die", "resolve_install_identity", "configure_workspace_owner_idmap",
    "resolve_hacocoon_access_user", "configure_hacocoon_access_group",
))


class InstallerIdentityTests(unittest.TestCase):
    def run_identity(self, *, caller="0", user="alice", uid="1007", gid="1013", sudo_user=""):
        # Positional parameters keep adversarial input out of shell source.
        script = FUNCTIONS + r'''
set -eu
caller="$1"; HACO_INSTALL_USER="$2"; target_uid="$3"; target_gid="$4"; SUDO_USER="$5"
id() {
  case "$*" in
    '-u') printf '%s\n' "$caller" ;;
    '-un') printf 'alice\n' ;;
    '-u -- alice') [ "$target_uid" != missing ] && printf '%s\n' "$target_uid" ;;
    '-g -- alice') printf '%s\n' "$target_gid" ;;
    '-u -- root'|'-g -- root') printf '0\n' ;;
    *) return 1 ;;
  esac
}
allow_root_subid() { printf 'subid:%s:%s\n' "$1" "$2"; }
getent() {
  case "$*" in
    'group hacocoon') printf 'hacocoon:x:998:\n' ;;
    'passwd alice') printf 'alice:x:1007:1013::/home/alice:/bin/bash\n' ;;
    *) return 1 ;;
  esac
}
record_privileged() { printf 'privileged'; printf ':%s' "$@"; printf '\n'; }
SUDO=record_privileged
HACOCOON_ACCESS_GROUP=hacocoon
resolve_install_identity
configure_workspace_owner_idmap
configure_hacocoon_access_group
printf 'identity:%s:%s:%s\n' "$INSTALL_USER" "$INSTALL_UID" "$INSTALL_GID"
'''
        return subprocess.run(["sh", "-c", script, "sh", caller, user, uid, gid, sudo_user],
                              text=True, capture_output=True)

    def test_root_bootstrap_keeps_exact_ordinary_owner(self):
        result = self.run_identity()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.splitlines(), [
            "subid:/etc/subuid:1007", "subid:/etc/subgid:1013",
            "privileged:/usr/sbin/usermod:-aG:hacocoon:alice", "identity:alice:1007:1013",
        ])

    def test_sudo_preserves_original_user(self):
        result = self.run_identity(user="", sudo_user="alice")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("identity:alice:1007:1013", result.stdout)

    def test_nonroot_may_select_only_itself(self):
        self.assertEqual(self.run_identity(caller="1007").returncode, 0)
        result = self.run_identity(caller="1010")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, "")

    def test_invalid_identity_fails_before_privileged_work(self):
        cases = [dict(user="root"), dict(uid="0"), dict(gid="0"), dict(uid="missing"),
                 dict(user="-root"), dict(user="alice\nroot"), dict(user="a;id"),
                 dict(user="../alice"), dict(uid="oops"), dict(gid="")]
        for case in cases:
            with self.subTest(case=case):
                result = self.run_identity(**case)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")


if __name__ == "__main__":
    unittest.main()

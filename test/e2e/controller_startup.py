#!/usr/bin/env python3
"""Exercise real Standard-egress startup refusal without a Host substrate.

Run with: python3 test/e2e/controller_startup.py /path/to/built/haco-controller
The same build's real haco must be beside it. This covers only failure before
listeners/provider startup; successful installed Standard egress is separate.
"""

import json
import os
from pathlib import Path
import signal
import stat
import subprocess
import sys
import tempfile


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def trusted_binary(path):
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_mode & 0o111
            and not info.st_mode & 0o022 and info.st_uid == os.geteuid(),
            "startup fixture requires owned, non-writable executable companions")


def metadata(path):
    info = path.lstat()
    # Reads can change atime. Everything else, including replacement of a file
    # with identical bytes or create/remove in a directory, must be observable.
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid,
            info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def run_controller(binary, env):
    process = subprocess.Popen(
        [str(binary), "--standard-egress"], env=env, cwd=env["HOME"],
        stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        start_new_session=True,
    )
    try:
        try:
            stdout, stderr = process.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            # A descendant may retain the pipes after the leader has exited.
            # Kill our isolated process group even when poll() reports an exit.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            raise AssertionError("Standard-egress startup did not refuse promptly") from None
        return process.returncode, stdout, stderr
    finally:
        if process.poll() is None:
            os.killpg(process.pid, signal.SIGKILL)
        process.wait()
        process.stdout.close()
        process.stderr.close()


def unique_fields(pairs):
    record = {}
    for key, value in pairs:
        require(key not in record, "startup refusal log has duplicate fields")
        record[key] = value
    return record


def check_refusal(result):
    code, stdout, stderr = result
    # A signal, deadline or unrelated configuration failure cannot satisfy this
    # assertion. Do not dump arbitrary subprocess output on assertion failure.
    require(code == 1, f"Standard-egress startup exit status {code}, expected 1")
    require(stdout == b"", "Standard-egress refusal wrote command output")
    require(b"listening" not in stderr, "failed controller announced readiness")
    try:
        record = json.loads(stderr, object_pairs_hook=unique_fields)
    except (ValueError, UnicodeError):
        raise AssertionError("startup refusal did not emit one JSON log record") from None
    require(isinstance(record, dict), "startup refusal log was not an object")
    timestamp = record.pop("time", None)
    require(isinstance(timestamp, str) and timestamp, "startup refusal log has no time")
    require(record == {
        "level": "ERROR", "msg": "controller failed", "component": "control",
        "error": "prepare Standard egress substrate failed",
    }, "startup refusal did not preserve the fixed safe failure category")


def main(binary):
    binary = Path(binary).resolve(strict=True)
    trusted_binary(binary)
    trusted_binary(binary.with_name("haco"))
    with tempfile.TemporaryDirectory(prefix="haco-startup-") as temporary:
        root = Path(temporary)
        home = root / "home"
        home.mkdir(mode=0o700)
        sentinels = [home / "retain", root / "unowned"]
        for path in sentinels:
            path.write_bytes(b"retain existing caller data\n")
            path.chmod(0o600)
        retained = [root, home, *sentinels]
        before = {path: metadata(path) for path in retained}
        state = root / "haco-root"
        socket = root / "control.sock"
        # This is an allowlist, not a copy of the caller's environment. Empty
        # PATH makes the first substrate command (`ip -o -4 address show`) fail
        # lookup before address/firewall changes, Incus or listener startup.
        # ConfigureEnvironmentDNS only validates/hashes the real sibling haco.
        # Never provide fake ip/sudo/incus helpers that could let it go further.
        env = {
            "PATH": "", "HOME": str(home), "LC_ALL": "C",
            "HACO_ROOT": str(state), "HACO_CONTROL_SOCKET": str(socket),
            "HACO_UI_LANGUAGE": "en", "HACO_LOG_LEVEL": "info",
            "HACO_LOG_FORMAT": "json",
        }
        check_refusal(run_controller(binary, env))
        require(not os.path.lexists(socket), "failed startup left a management endpoint")
        require(not os.path.lexists(state), "failed startup created product state")
        require(set(root.iterdir()) == {home, sentinels[1]}
                and set(home.iterdir()) == {sentinels[0]},
                "failed startup changed the private fixture contents")
        require({path: metadata(path) for path in retained} == before,
                "failed startup changed existing caller data or directory metadata")
        require(all(path.read_bytes() == b"retain existing caller data\n" for path in sentinels),
                "failed startup changed existing caller data bytes")
    print("Controller Standard-egress startup / missing substrate refusal: PASS", flush=True)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: controller_startup.py /path/to/built/haco-controller")
    main(sys.argv[1])

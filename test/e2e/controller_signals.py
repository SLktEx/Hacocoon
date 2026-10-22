#!/usr/bin/env python3
"""Linux shipped bare-controller readiness and graceful OS-signal shutdown.

Run with: python3 test/e2e/controller_signals.py /path/to/built/haco-controller
Only system.ping is requested. This is not installed Standard-egress acceptance.
"""

import json
import os
from pathlib import Path
import shutil
import signal
import socket
import stat
import struct
import subprocess
import sys
import tempfile
import time

from controller_startup import metadata, require, trusted_binary


def private_endpoint(path):
    info = path.lstat()
    require(stat.S_ISSOCK(info.st_mode) and stat.S_IMODE(info.st_mode) == 0o600
            and info.st_uid == os.geteuid(),
            "controller endpoint was not an owned private socket")
    return info


def unique_fields(pairs):
    value = {}
    for key, item in pairs:
        require(key not in value, "controller ping response has duplicate fields")
        value[key] = item
    return value


def ping(path, timeout, pid):
    deadline = time.monotonic() + timeout
    before = path.lstat()
    require(stat.S_ISSOCK(before.st_mode) and before.st_uid == os.geteuid(),
            "controller endpoint was not an owned socket")
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as connection:
        def bound():
            remaining = deadline - time.monotonic()
            require(remaining > 0, "controller ping exceeded its deadline")
            connection.settimeout(remaining)

        bound()
        connection.connect(str(path))
        peer_pid, peer_uid, _ = struct.unpack("3i", connection.getsockopt(
            socket.SOL_SOCKET, socket.SO_PEERCRED, struct.calcsize("3i")))
        require(peer_pid == pid and peer_uid == os.geteuid(),
                "controller ping reached a different process")
        bound()
        connection.sendall(b'{"version":1,"method":"system.ping"}\n')
        response = bytearray()
        while True:
            bound()
            chunk = connection.recv(4097 - len(response))
            if not chunk:
                break
            response.extend(chunk)
            require(len(response) <= 4096, "controller ping response exceeded its bound")
        try:
            value = json.loads(response, object_pairs_hook=unique_fields)
        except (ValueError, UnicodeError):
            raise AssertionError("controller ping response was malformed") from None
        require(value == {"version": 1, "payload": {"protocol_version": 1}}
                and type(value["version"]) is int
                and type(value["payload"]["protocol_version"]) is int,
                "controller ping did not establish protocol readiness")
    # ListenUnix chmods after bind and serves only after that completes. Check
    # final mode after the reply so a scheduler pause in that window is not a
    # false startup refusal; the enclosing fixture is private throughout.
    require(os.path.samestat(before, private_endpoint(path)),
            "controller endpoint changed during readiness")
    return before


def wait_ready(process, path, timeout=5):
    deadline = time.monotonic() + timeout
    while True:
        code = process.poll()
        require(code is None, f"controller exited before readiness (status {code})")
        remaining = deadline - time.monotonic()
        require(remaining > 0, "controller did not become ready before the deadline")
        try:
            return ping(path, remaining, process.pid)
        except (FileNotFoundError, ConnectionRefusedError):
            # Only transport unavailability is retryable. A socket pathname,
            # malformed reply, protocol refusal or silent peer is not readiness.
            time.sleep(min(0.01, remaining))


def graceful_shutdown(process, signum, timeout=5):
    require(process.poll() is None, "controller exited before the requested signal")
    # This child is still unreaped, so its PID cannot be reused. Do not use
    # Popen.send_signal: its second poll can silently skip a now-exited child
    # and make a spontaneous zero exit look like a successful signal test.
    try:
        os.kill(process.pid, signum)
    except ProcessLookupError:
        raise AssertionError("controller exited before the requested signal") from None
    try:
        code = process.wait(timeout=timeout)
    except subprocess.TimeoutExpired:
        raise AssertionError("controller did not exit after the requested signal") from None
    require(code == 0, f"controller signal shutdown status {code}, expected 0")


def reap_failed_process(process):
    if process is None or process.poll() is not None:
        return False
    # This exact unreaped child is the only process started by the fixture.
    # The audited empty-catalog bare path does not launch subprocesses. A forced
    # cleanup is only reached after failure and can never satisfy acceptance.
    process.kill()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        raise AssertionError("controller could not be reaped after forced cleanup") from None
    return True


def check_retained(root, retained, contents):
    home, state, endpoint = root / "home", root / "data", root / "control.sock"
    sentinels = [root / "retain", home / "retain", state / "retain"]
    require(not os.path.lexists(endpoint), "controller shutdown retained its endpoint")
    require({path: metadata(path) for path in retained} == retained,
            "controller changed retained private data or home metadata")
    require(all(path.read_bytes() == contents for path in sentinels),
            "controller changed retained private data bytes")
    require(set(root.iterdir()) == {home, state, sentinels[0]}
            and set(home.iterdir()) == {sentinels[1]}
            and set(state.iterdir()) == {sentinels[2], state / "run"}
            and set((state / "run").iterdir()) == {state / "run" / "git"}
            and not list((state / "run" / "git").iterdir()),
            "controller created unexpected private fixture state")
    for path in (root, home, state, state / "run", state / "run" / "git"):
        info = path.lstat()
        require(stat.S_ISDIR(info.st_mode) and stat.S_IMODE(info.st_mode) == 0o700
                and info.st_uid == os.geteuid(), "controller fixture lost private directory ownership")


def remove_fixture(root, owned):
    current = root.lstat()
    require(stat.S_ISDIR(current.st_mode) and os.path.samestat(owned, current),
            "controller private fixture root changed; retaining it")
    shutil.rmtree(root)


def cleanup_case(process, root, owned_root, failure):
    try:
        forced = reap_failed_process(process)
        if forced and failure is None:
            failure = AssertionError("controller required forced cleanup")
    except BaseException as error:
        if failure is None:
            return error
        failure.add_note("Controller child cleanup also failed; its private fixture was retained")
        return failure
    try:
        remove_fixture(root, owned_root)
    except (OSError, AssertionError):
        if failure is None:
            failure = AssertionError("controller private fixture cleanup failed")
        else:
            failure.add_note("Controller private fixture cleanup also failed")
    return failure


def run_case(binary, signum):
    root = Path(tempfile.mkdtemp(prefix="haco-signal-"))
    owned_root = root.lstat()
    process = None
    failure = None
    try:
        home, state, endpoint = root / "home", root / "data", root / "control.sock"
        home.mkdir(mode=0o700)
        state.mkdir(mode=0o700)
        sentinels = [root / "retain", home / "retain", state / "retain"]
        contents = b"retain existing private fixture data\n"
        for path in sentinels:
            path.write_bytes(contents)
            path.chmod(0o600)
        retained = {path: metadata(path) for path in [home, *sentinels]}
        # No caller environment, catalog, Policy, credentials or fake commands.
        # Composition only configures lazy provider callbacks; with no bindings,
        # GitBroker.Start creates data/run/git and no guest/provider endpoint.
        env = {
            "PATH": "", "HOME": str(home), "LC_ALL": "C",
            "HACO_ROOT": str(state), "HACO_CONTROL_SOCKET": str(endpoint),
            "HACO_UI_LANGUAGE": "en", "HACO_LOG_LEVEL": "info",
            "HACO_LOG_FORMAT": "json",
        }
        # Assertions report fixed categories and numeric status, never raw logs.
        # No output pipe can keep collection blocked after the owned child exits.
        process = subprocess.Popen(
            [str(binary)], env=env, cwd=home, stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True,
        )
        ready = wait_ready(process, endpoint)
        require(os.path.samestat(ready, private_endpoint(endpoint)),
                "controller endpoint changed before shutdown")
        graceful_shutdown(process, signum)
        check_retained(root, retained, contents)
    except BaseException as error:
        failure = error
    finally:
        failure = cleanup_case(process, root, owned_root, failure)
    if failure is not None:
        raise failure


def main(binary):
    binary = Path(binary).resolve(strict=True)
    trusted_binary(binary)
    for signum in (signal.SIGINT, signal.SIGTERM):
        run_case(binary, signum)
        print(f"Bare controller / {signum.name} graceful shutdown: PASS", flush=True)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: controller_signals.py /path/to/built/haco-controller")
    main(sys.argv[1])

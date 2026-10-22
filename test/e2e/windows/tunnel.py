#!/usr/bin/env python3
"""Windows TCP acceptance through an ordinary installed Host terminal.

The caller owns the supplied test Env. Incus starts only its bounded application
fixture; the tunnel itself uses the normal user command and installed companion.
No packages, permissions, interop handlers or product files are repaired here.
"""
import argparse
import concurrent.futures
import importlib.util
import json
import os
from pathlib import Path
import queue
import re
import shutil
import socket
import subprocess
import sys
import threading
import time

forward_spec = importlib.util.spec_from_file_location("installed_forward", Path(__file__).resolve().parents[1] / "installed/forward.py")
forward = importlib.util.module_from_spec(forward_spec)
forward_spec.loader.exec_module(forward)
SERVER, arm_application = forward.SERVER, forward.arm_application
TERMINAL_TIMEOUT_SECONDS = 180


def terminal_receipt(terminal):
    # Read observer state only. Never query or stop another process for a receipt.
    record = {"terminal_exit_known": False, "terminal_exit_code": None,
              "terminal_reader": "unknown", "terminal_reader_alive": None,
              "terminal_run_stop": "unknown"}
    try:
        status = terminal.observed_exit_status
        if type(status) is int:
            record.update(terminal_exit_known=True, terminal_exit_code=status)
        reader = terminal.reader_outcome
        if type(reader) is str and reader in ("running", "eof", "failed"):
            record["terminal_reader"] = reader
        stop = terminal.run_stop
        if type(stop) is str and stop in ("not_started", "running", "reader_done", "process_exited", "callback_failed", "timeout"):
            record["terminal_run_stop"] = stop
        alive = terminal._reader.is_alive()
        if type(alive) is bool:
            record["terminal_reader_alive"] = alive
    except Exception:
        # Partial observations remain observations; never interrupt owned cleanup.
        return record
    return record


def process_receipt(*, alive, terminated, observer, application, exit_code):
    record = {}
    for key, value in (("terminal_alive", alive), ("termination_returned", terminated)):
        record[key] = value if type(value) is bool else None
    if type(exit_code) is int:
        record["exit_code"] = exit_code
    try:
        if observer is not None:
            observed = observer.is_alive()
            record["application_observer_alive"] = observed if type(observed) is bool else None
        if application is not None:
            status = application.poll()
            record["application_exit_code"] = status if type(status) is int else None
    except Exception:
        return record
    return record


def write_phase(started, name, *, terminal=None, alive=None, terminated=None, observer=None, application=None, exit_code=None):
    try:
        phases = ("application_start", "application_ready", "host_entry_start", "host_ready", "listener_ready",
                  "native_owner_confirmed", "application_armed", "exchange_completed", "ctrl_c_sent",
                  "tunnel_exit_received", "listener_connect_failed", "host_exit_sent", "host_returned", "terminal_exit_sent",
                  "terminal_run_finished", "failed", "cleanup_start", "terminal_cleanup", "terminal_terminate_start",
                  "terminal_terminate_returned", "application_observer_join_returned", "cleanup_completed", "cleanup_finished")
        record = {"component": "ci", "operation": "windows_tunnel_entry",
                  "phase": name if type(name) is str and name in phases else "unknown",
                  "duration_ms": int((time.monotonic() - started) * 1000)}
        if terminal is not None:
            record.update(terminal_receipt(terminal))
        record.update(process_receipt(alive=alive, terminated=terminated, observer=observer,
                                      application=application, exit_code=exit_code))
        print(json.dumps(record), flush=True)
    except Exception:
        # Serialization, accessor and sink failures must not replace acceptance.
        return


def exchange(port):
    data = bytes(range(256)) * 4096
    with socket.create_connection(("127.0.0.1", port), timeout=15) as client:
        client.sendall(data)
        client.shutdown(socket.SHUT_WR)
        result = bytearray()
        while True:
            part = client.recv(65536)
            if not part:
                break
            result.extend(part)
        if result != data:
            raise RuntimeError("Windows tunnel binary response differs")


def assert_native_owner(port):
    # Only an integer derived from the CLI's loopback readiness reaches this
    # fixed observation. A WSL forwarding listener is not native client proof.
    script = ("$ErrorActionPreference='Stop'; [Console]::OutputEncoding=[Text.UTF8Encoding]::new($false); $connections=@(Get-NetTCPConnection "
              f"-LocalAddress 127.0.0.1 -LocalPort {int(port)} -State Listen); "
              "if($connections.Count -ne 1){throw 'listener ownership ambiguous'}; "
              "(Get-Process -Id $connections[0].OwningProcess).Path | ConvertTo-Json -Compress")
    result = subprocess.run([shutil.which("pwsh"), "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script],
                            capture_output=True, text=True, encoding="utf-8", timeout=15, check=True)
    executable = json.loads(result.stdout)
    root = str(Path(os.environ["LOCALAPPDATA"]) / "Hacocoon" / "client")
    if not re.fullmatch(re.escape(root) + r"\\[a-f0-9]{32}\\haco-tunnel\.exe", executable, re.I):
        raise RuntimeError("listener is not owned by the installed native tunnel client")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--env", required=True)
    parser.add_argument("--distro", default="Hacocoon")
    args = parser.parse_args()
    if os.name != "nt" or not re.fullmatch(r"[a-z0-9][a-z0-9-]{0,63}", args.env) or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,63}", args.distro):
        raise RuntimeError("Windows and exact caller-owned fixture names required")
    if not shutil.which("pwsh"):
        raise RuntimeError("PowerShell 7 is required for native listener observation")
    started = time.monotonic()
    def phase(name, **observations):
        write_phase(started, name, **observations)
    phase("application_start")
    application = subprocess.Popen(["wsl.exe", "-d", args.distro, "-u", "root", "--exec", "incus", "exec", "haco-" + args.env,
                                    "--project", "hacocoon", "--", "python3", "-u", "-c", SERVER], stdin=subprocess.PIPE, stdout=subprocess.PIPE)
    readiness = queue.Queue(maxsize=1)
    def read_ready():
        readiness.put(application.stdout.readline(4096))
    observer = threading.Thread(target=read_ready, daemon=True)
    observer.start()
    terminal = None
    try:
        target = readiness.get(timeout=25).decode("utf-8").strip()
        if not target.isdecimal() or not 1 <= int(target) <= 65535:
            raise RuntimeError("application fixture readiness missing")
        phase("application_ready")
        spec = importlib.util.spec_from_file_location("tunnel_terminal", Path(__file__).with_name("install.py"))
        driver = importlib.util.module_from_spec(spec)
        sys.modules[spec.name] = driver
        spec.loader.exec_module(driver)
        terminal = driver.TerminalProcess()
        stage, sent_at, port = 0, 0, None
        def drive(output, process):
            nonlocal stage, sent_at, port
            fresh = output[sent_at:]
            if stage == 0 and driver.cmd_prompt_count(output):
                phase("host_entry_start")
                process.write("wsl -d " + args.distro + "\r\n")
                stage, sent_at = 1, len(output)
            elif stage == 1 and re.search(r"(?m)^[^\r\n]*@haco-host:[^\r\n]*[#\$]\s*$", fresh):
                phase("host_ready")
                process.write(f"HACO_UI_LANGUAGE=en haco env tunnel --duration 90s --target-port {target} {args.env}; printf '\\nTUNNEL-EXIT:%s\\n' \"$?\"\r")
                stage, sent_at = 2, len(output)
            elif stage == 2:
                match = re.search(r"Listening at 127\.0\.0\.1:(\d+) ", fresh)
                if match:
                    phase("listener_ready")
                    port = int(match.group(1))
                    assert_native_owner(port)
                    phase("native_owner_confirmed")
                    if application.poll() is not None:
                        raise RuntimeError("application fixture exited before tunnel exchange")
                    arm_application(application)
                    phase("application_armed")
                    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as workers:
                        list(workers.map(exchange, [port] * 8))
                    phase("exchange_completed")
                    if application.wait(timeout=15) != 0:
                        raise RuntimeError("application fixture failed")
                    process.write("\x03")
                    phase("ctrl_c_sent")
                    stage = 3
            elif stage == 3:
                # Wait for a complete output line: a terminal read can split the
                # status digits, and an echoed printf is not an exit receipt.
                # Bash's $? is a canonical decimal status in the range 0-255.
                match = re.search(r"(?m)^TUNNEL-EXIT:(0|[1-9][0-9]?|1[0-9]{2}|2[0-4][0-9]|25[0-5])[ \t]*\n", fresh)
                if not match:
                    return
                exit_code = int(match.group(1))
                phase("tunnel_exit_received", exit_code=exit_code)
                if exit_code != 0:
                    raise RuntimeError(f"Windows tunnel exited with code {exit_code} after Ctrl+C")
                try:
                    connection = socket.create_connection(("127.0.0.1", port), timeout=2)
                except OSError:
                    pass
                else:
                    connection.close()
                    raise RuntimeError("Windows listener survived Ctrl+C")
                phase("listener_connect_failed")
                process.write("exit\r")
                phase("host_exit_sent")
                stage, sent_at = 4, len(output)
            elif stage == 4 and driver.cmd_prompt_count(fresh):
                phase("host_returned")
                process.write("exit\r\n")
                phase("terminal_exit_sent")
                stage = 5
        try:
            terminal.run(on_output=drive, timeout=TERMINAL_TIMEOUT_SECONDS)
        finally:
            phase("terminal_run_finished", terminal=terminal)
        if stage != 5:
            raise RuntimeError("ordinary Windows tunnel entry did not complete")
        print("WINDOWS AUTOMATIC TUNNEL / NATIVE OWNER / 8x1MiB / HALF-CLOSE / CTRL+C CLEANUP: PASS")
    except Exception:
        phase("failed", application=application)
        raise
    finally:
        phase("cleanup_start", terminal=terminal)
        try:
            if terminal is not None:
                alive = terminal.proc.isalive()
                phase("terminal_cleanup", terminal=terminal, alive=alive)
                if alive:
                    phase("terminal_terminate_start")
                    terminated = terminal.proc.terminate(force=True)
                    phase("terminal_terminate_returned", terminated=terminated)
            if not application.stdin.closed:
                application.stdin.close()
            if application.poll() is None:
                application.terminate()
                try:
                    application.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    application.kill()
                    application.wait(timeout=5)
            application.stdout.close()
            observer.join(timeout=2)
            phase("application_observer_join_returned", observer=observer)
            phase("cleanup_completed")
        finally:
            phase("cleanup_finished", terminal=terminal)


if __name__ == "__main__":
    main()

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

forward_spec = importlib.util.spec_from_file_location("installed_forward", Path(__file__).resolve().parents[1] / "installed/forward.py")
forward = importlib.util.module_from_spec(forward_spec)
forward_spec.loader.exec_module(forward)
SERVER = forward.SERVER
# Application readiness precedes ordinary Host entry and native helper startup.
# Keep this fixture alive for the bounded terminal journey, including cold WSL
# entry and listener ownership observation, before all eight clients arrive.
TERMINAL_TIMEOUT_SECONDS = 180


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
    application = subprocess.Popen(["wsl.exe", "-d", args.distro, "-u", "root", "--exec", "incus", "exec", "haco-" + args.env,
                                    "--project", "hacocoon", "--", "python3", "-u", "-c", SERVER,
                                    "--accept-timeout", str(TERMINAL_TIMEOUT_SECONDS)], stdout=subprocess.PIPE)
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
                process.write("wsl -d " + args.distro + "\r\n")
                stage, sent_at = 1, len(output)
            elif stage == 1 and re.search(r"(?m)^[^\r\n]*@haco-host:[^\r\n]*[#\$]\s*$", fresh):
                process.write(f"HACO_UI_LANGUAGE=en haco env tunnel --duration 90s --target-port {target} {args.env}; printf '\\nTUNNEL-EXIT:%s\\n' \"$?\"\r")
                stage, sent_at = 2, len(output)
            elif stage == 2:
                match = re.search(r"Listening at 127\.0\.0\.1:(\d+) ", fresh)
                if match:
                    port = int(match.group(1))
                    assert_native_owner(port)
                    if application.poll() is not None:
                        raise RuntimeError("application fixture exited before tunnel exchange")
                    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as workers:
                        list(workers.map(exchange, [port] * 8))
                    if application.wait(timeout=15) != 0:
                        raise RuntimeError("application fixture failed")
                    process.write("\x03")
                    stage = 3
            elif stage == 3 and re.search(r"(?m)^TUNNEL-EXIT:0\s*$", fresh):
                try:
                    connection = socket.create_connection(("127.0.0.1", port), timeout=2)
                except OSError:
                    pass
                else:
                    connection.close()
                    raise RuntimeError("Windows listener survived Ctrl+C")
                process.write("exit\r")
                stage, sent_at = 4, len(output)
            elif stage == 4 and driver.cmd_prompt_count(fresh):
                process.write("exit\r\n")
                stage = 5
        terminal.run(on_output=drive, timeout=TERMINAL_TIMEOUT_SECONDS)
        if stage != 5:
            raise RuntimeError("ordinary Windows tunnel entry did not complete")
        print("WINDOWS AUTOMATIC TUNNEL / NATIVE OWNER / 8x1MiB / HALF-CLOSE / CTRL+C CLEANUP: PASS")
    finally:
        if terminal is not None and terminal.proc.isalive():
            terminal.proc.terminate(force=True)
        if application.poll() is None:
            application.terminate()
            try:
                application.wait(timeout=5)
            except subprocess.TimeoutExpired:
                application.kill()
                application.wait(timeout=5)
        application.stdout.close()
        observer.join(timeout=2)


if __name__ == "__main__":
    main()

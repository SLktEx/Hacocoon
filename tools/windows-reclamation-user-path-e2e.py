#!/usr/bin/env python3
"""Exercise public reclaim through the installed ordinary Host terminal.

Only Windows read-only observations run between dispatch and worker exit. No
retry, record clearing, Job changes or internal mutation commands are used.
"""
import importlib.util
import json
import ntpath
import os
import re
import subprocess
import sys
import time
import uuid
from pathlib import Path


def helper_is_running(rows, helper):
    if isinstance(rows, dict):
        rows = [rows]
    if not isinstance(rows, list):
        raise RuntimeError("Windows process observation unavailable")
    for row in rows:
        if not isinstance(row, dict) or not isinstance(row.get("ExecutablePath"), str) or not row["ExecutablePath"]:
            raise RuntimeError("Windows helper identity unavailable")
        if ntpath.normcase(row["ExecutablePath"]) == ntpath.normcase(str(helper)):
            return True
    return False


def require_complete(result, operation):
    if result.get("operation", "").lower() != operation.lower():
        raise RuntimeError("Saved operation changed; retain evidence")
    if result.get("state") == "failed":
        raise RuntimeError("Reclamation failed; retain recorded failure")
    if result.get("state") == "pending":
        return False
    if result.get("state") != "complete":
        raise RuntimeError("Unknown reclamation state")
    linux = result.get("linux", {})
    if not result.get("linux_started") or linux.get("failure") or linux.get("cleanup_failed"):
        raise RuntimeError("Linux completion unproven")
    if not all(linux.get(k, {}).get("status") == "complete" for k in ("incus_btrfs_loop", "wsl_ext4")):
        raise RuntimeError("Linux stages incomplete")
    windows = result.get("observation", {})
    if not all(windows.get(k) is True for k in ("StopAttempted", "StopRequested", "ResumeAttempted", "Resumed")):
        raise RuntimeError("Windows completion unproven")
    compact = windows.get("Compaction", {})
    if compact.get("Attempted") is not True or compact.get("Completed") is not True or compact.get("Virtual", {}).get("Capacity", 0) <= 0:
        raise RuntimeError("Windows compaction unproven")
    return True


def read_json(command):
    result = subprocess.run(command, capture_output=True, timeout=25)
    if result.returncode or len(result.stdout) > 16384:
        raise RuntimeError("Windows read-only observation failed")
    return json.loads(result.stdout.decode("utf-8-sig"))


def wait_for_worker(helper, registration, operation):
    powershell = str(Path(os.environ["SystemRoot"]) / "System32/WindowsPowerShell/v1.0/powershell.exe")
    script = "[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false);$ErrorActionPreference='Stop';ConvertTo-Json -Compress -InputObject @(Get-CimInstance Win32_Process -Filter \"Name='haco-wsl.exe'\" | Select-Object ExecutablePath)"
    deadline = time.monotonic() + 690
    while time.monotonic() < deadline:
        # These commands never enter WSL, mutate records or launch a worker.
        result = read_json([str(helper), "_status", registration, operation])
        complete = require_complete(result, operation)
        rows = read_json([powershell, "-NoProfile", "-NonInteractive", "-Command", script])
        running = helper_is_running(rows, helper)
        if complete and not running:
            print(json.dumps({"public_worker": "PASS", "observations": result}), flush=True)
            return
        if not running and not complete:
            raise RuntimeError("Worker absent with pending outcome; retain evidence")
        time.sleep(5)
    raise RuntimeError("Worker completion unknown; no WSL restart or retry")


def load_driver(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


def main():
    if os.name != "nt" or sys.argv[1:]:
        raise RuntimeError("requires installed Windows acceptance without overrides")
    driver = load_driver("reclamation_user_driver", "windows-installer-user-path-e2e.py")
    installed = load_driver("reclamation_registration", "windows-reclamation-linux-e2e.py")
    reg = installed.registration()
    helper = Path(os.environ["LOCALAPPDATA"]) / "Hacocoon/reclamation" / uuid.UUID(reg).hex / "haco-wsl.exe"
    if not helper.is_file():
        raise RuntimeError("Installed helper missing")
    terminal = driver.TerminalProcess()
    stage, sent_at = 0, 0
    terminal_confirmed = False

    def drive(output, process):
        nonlocal stage, sent_at, terminal_confirmed
        fresh = output[sent_at:]
        host = re.search(r"(?m)^[^\r\n]*@haco-host:[^\r\n]*[#\$]\s*$", fresh)
        if stage == 0 and driver.cmd_prompt_count(fresh):
            process.write("wsl -d Hacocoon\r\n")
            stage, sent_at = 1, len(output)
        elif stage == 1 and host:
            process.write("haco reclaim --yes\r\n")
            stage, sent_at = 2, len(output)
        elif stage == 2 and "Worker dispatched; reclamation is not yet confirmed." in fresh:
            match = re.search(r"Operation: (\{[0-9a-fA-F-]{36}\})", fresh)
            if not match:
                raise RuntimeError("Dispatch identity unavailable; retain operation")
            operation = "{" + str(uuid.UUID(match[1])) + "}"
            wait_for_worker(helper, reg, operation)
            terminal_confirmed = True
            if driver.cmd_prompt_count(fresh):
                process.write("wsl -d Hacocoon\r\n")
                stage, sent_at = 4, len(output)
            else:
                stage, sent_at = 3, len(output)
        elif stage == 2 and ("could not be confirmed" in fresh or "Dispatch result unavailable" in fresh or "Managed Windows installation unavailable" in fresh):
            raise RuntimeError("Public dispatch failed; no retry or WSL reentry")
        elif stage == 3 and driver.cmd_prompt_count(fresh):
            process.write("wsl -d Hacocoon\r\n")
            stage, sent_at = 4, len(output)
        elif stage == 4 and host:
            process.write("haco reclaim --status && test \"$(cat $HOME/.hacocoon-installer-acceptance)\" = kept-through-restart-and-rerun && printf 'PUBLIC_RECLAIM_STATUS_OK\\n'\r\n")
            stage, sent_at = 5, len(output)
        elif stage == 5 and re.search(r"(?m)^PUBLIC_RECLAIM_STATUS_OK\s*$", fresh):
            if "Reclamation: complete" not in fresh:
                raise RuntimeError("Public result did not confirm combined completion")
            process.write("exit\r\n")
            stage, sent_at = 6, len(output)
        elif stage == 6 and driver.cmd_prompt_count(fresh):
            process.write("exit\r\n")
            stage = 7

    try:
        terminal.run(on_output=drive)
        if stage != 7:
            raise RuntimeError("Public reclamation journey incomplete")
    finally:
        # Do not kill an unknown worker tree after failed dispatch/observation.
        if terminal_confirmed and terminal.proc.isalive():
            terminal.proc.terminate(force=True)
    print("PUBLIC RECLAMATION AND HOST SENTINEL AFTER RESUME: PASS")
    print("Whole Workspace/OCI acceptance: not exercised by this gate")


if __name__ == "__main__":
    main()

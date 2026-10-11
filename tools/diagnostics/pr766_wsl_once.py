#!/usr/bin/env python3
"""One authorized stock WSL/ConPTY experiment, never a Hacocoon acceptance retry.

This branch-only helper deliberately has no restart, update, policy or credential
operation. A clean run cannot clear PR766. Unregister is confined to the newly
imported registration whose identity is revalidated immediately before deletion.
The name-only WSL API is not atomic against a hostile concurrent registry writer.
"""
from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import json
import ntpath
import os
from pathlib import Path
import queue
import re
import subprocess
import sys
import threading
import time
import urllib.request
from dataclasses import dataclass

BRANCH = "investigate/pr766-wsl-once2-20261011"
VENDOR_METADATA = "https://raw.githubusercontent.com/microsoft/WSL/341f1ddf76f089ff6351590b03f92bad30041725/distributions/DistributionInfo.json"
IMAGE_URL = "https://releases.ubuntu.com/26.04.1/ubuntu-26.04.1-wsl-amd64.wsl"
IMAGE_SHA = "48d56724b5c8e60f24893e83e73bbb58c60b3ca22fba3da977075420acd54104"
REGISTRY = r"Software\Microsoft\Windows\CurrentVersion\Lxss"
LIMIT = 16384
ANSI = re.compile(r"\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07]*(?:\x07|\x1b\\)")
PROCESS_NAMES = {"wsl.exe", "wslhost.exe", "wslservice.exe", "wslrelay.exe"}
SERVICE_NAMES = {"WslService", "LxssManager", "vmcompute"}
VERSION_FIELDS = {"WSL version", "Kernel version", "WSLg version", "MSRDC version",
                  "Direct3D version", "DXCore version", "Windows version"}
CHILD_ERRORS = {"console_timeout", "console_fallback", "console_output_limit",
                "console_exit_unconfirmed", "console_observation_failed",
                "observation_size_limit", "observation_timeout",
                "observation_termination_unconfirmed", "observation_pipe_unconfirmed",
                "observation_reader_failed", "observation_inventory_limit",
                "observation_inventory_name", "observation_inventory_pid",
                "observation_service_state", "OSError", "RuntimeError",
                "JSONDecodeError", "UnicodeDecodeError", "BrokenPipeError"}
INVENTORY_ERRORS = {"passive_observation_failed", "capture_budget_expired",
                    "observation_size_limit", "observation_timeout",
                    "observation_termination_unconfirmed", "observation_pipe_unconfirmed",
                    "observation_reader_failed", "observation_inventory_limit",
                    "observation_inventory_name", "observation_inventory_pid",
                    "observation_service_state", "JSONDecodeError", "UnicodeDecodeError",
                    "OSError", "PermissionError", "FileNotFoundError", "TypeError", "AttributeError"}
CAPTURE_DEADLINE = None
CRITICAL_OBSERVATIONS = {"status": "not_collected", "reason": "critical_path_isolation"}
OBSERVE_SCRIPT = r"""
$ErrorActionPreference='Stop'
[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false)
$services=@(Get-CimInstance Win32_Service -Filter "Name='WslService' OR Name='LxssManager' OR Name='vmcompute'" | ForEach-Object {
  @{name=$_.Name;state=$_.State;pid=[int]$_.ProcessId}
})
$processes=@(Get-CimInstance Win32_Process -Filter "Name='wsl.exe' OR Name='wslhost.exe' OR Name='wslservice.exe' OR Name='wslrelay.exe'" | ForEach-Object {
  @{name=$_.Name;pid=[int]$_.ProcessId;parent=[int]$_.ParentProcessId;created=$_.CreationDate.ToUniversalTime().ToString('o')}
})
@{services=$services;processes=$processes} | ConvertTo-Json -Depth 4 -Compress
"""


class Fault(Exception):
    """Fixed, non-sensitive error category; never include child output."""

    def __init__(self, category, *, process_cleanup_unconfirmed=False):
        super().__init__(category)
        self.process_cleanup_unconfirmed = process_cleanup_unconfirmed


def decode(data):
    if data.startswith(b"\xff\xfe") or b"\x00" in data[:80]:
        return data.decode("utf-16-le", errors="strict").lstrip("\ufeff")
    return data.decode("utf-8-sig", errors="strict")


@dataclass
class Result:
    code: int
    out: str
    err: str

    def receipt(self):
        return {"exit_code": self.code, "stdout_chars": len(self.out),
                "stderr_chars": len(self.err),
                "wsl_service_unexpected": "Wsl/Service/E_UNEXPECTED" in self.out + self.err}


def capture(argv, timeout, limit=LIMIT):
    """Bounded memory plus bounded OS pipes, never a temporary output file.

    Each unbuffered reader owns its close. The parent never closes a stream while
    its reader holds it. Descendant-held pipes that do not reach EOF remain a
    failure after the bounded join; no partial output can become success.
    """
    if CAPTURE_DEADLINE is not None:
        timeout = min(timeout, CAPTURE_DEADLINE - time.monotonic())
        if timeout <= 0:
            raise Fault("capture_budget_expired")
    child = subprocess.Popen(argv, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE, bufsize=0)
    buffers = [bytearray(), bytearray()]
    overflow, reader_error = threading.Event(), threading.Event()

    def drain(stream, buffer):
        try:
            while chunk := stream.read(1024):
                available = max(0, limit - len(buffer))
                buffer.extend(chunk[:available])
                if len(chunk) > available:
                    overflow.set()
        except Exception:
            reader_error.set()
        finally:
            try:
                stream.close()
            except Exception:
                reader_error.set()

    threads = [threading.Thread(target=drain, args=(stream, buffer), daemon=True)
               for stream, buffer in zip((child.stdout, child.stderr), buffers)]
    for thread in threads:
        thread.start()
    deadline, failure = time.monotonic() + timeout, None
    while child.poll() is None:
        if overflow.is_set() or time.monotonic() >= deadline:
            failure = "observation_size_limit" if overflow.is_set() else "observation_timeout"
            try:
                child.kill()
                child.wait(timeout=5)
            except Exception:
                failure = "observation_termination_unconfirmed"
            break
        try:
            child.wait(timeout=min(0.05, max(0.001, deadline - time.monotonic())))
        except subprocess.TimeoutExpired:
            pass
    join_deadline = time.monotonic() + 2
    for thread in threads:
        thread.join(timeout=max(0, join_deadline - time.monotonic()))
    pipes_unconfirmed = any(thread.is_alive() for thread in threads)
    if failure:
        raise Fault(failure, process_cleanup_unconfirmed=pipes_unconfirmed or failure == "observation_termination_unconfirmed")
    if pipes_unconfirmed:
        raise Fault("observation_pipe_unconfirmed", process_cleanup_unconfirmed=True)
    if overflow.is_set() or reader_error.is_set():
        raise Fault("observation_size_limit" if overflow.is_set() else "observation_reader_failed")
    return Result(child.returncode, *(decode(bytes(buffer)) for buffer in buffers))


def canonical(path):
    value = str(path)
    if value.startswith("\\\\?\\"):
        value = value[4:]
    if not ntpath.isabs(value) or "%" in value:
        raise Fault("noncanonical_path")
    return ntpath.normcase(ntpath.normpath(value))


def path_identity(path):
    path = Path(path)
    for part in (path, *path.parents):
        info = part.lstat()
        if part.is_symlink() or getattr(info, "st_file_attributes", 0) & 0x400:
            raise Fault("reparse_path")
    info = path.stat(follow_symlinks=False)
    if not path.is_dir() or not info.st_ino:
        raise Fault("directory_identity_unavailable")
    return (info.st_dev, info.st_ino)


def registrations():
    import winreg  # Only Windows execution, never imported by pure Linux tests.
    rows = []
    try:
        root = winreg.OpenKey(winreg.HKEY_CURRENT_USER, REGISTRY, 0, winreg.KEY_READ)
    except FileNotFoundError:
        return rows
    with root:
        count = winreg.QueryInfoKey(root)[0]
        if count > 256:
            raise Fault("registration_inventory_limit")
        for index in range(count):
            guid = winreg.EnumKey(root, index)
            with winreg.OpenKey(root, guid, 0, winreg.KEY_READ) as key:
                rows.append({"guid": guid,
                             "name": winreg.QueryValueEx(key, "DistributionName")[0],
                             "path": canonical(winreg.QueryValueEx(key, "BasePath")[0])})
    return rows


def matching_registration(rows, name):
    matches = [row for row in rows if row["name"].casefold() == name.casefold()]
    if len(matches) != 1 or matches[0]["name"] != name:
        raise Fault("registration_ambiguous")
    row = matches[0]
    if not re.fullmatch(r"\{[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}\}", row["guid"]):
        raise Fault("registration_guid_invalid")
    return row


def assert_owned(record, rows, path, identity):
    if not record.get("import_succeeded") or not record.get("pre_absent"):
        raise Fault("import_ownership_unconfirmed")
    actual = matching_registration(rows, record["name"])
    if actual["guid"] in record.get("pre_guids", ()):
        raise Fault("registration_not_new")
    if actual != record["registration"] or actual["path"] != canonical(path):
        raise Fault("registration_identity_changed")
    if tuple(identity) != tuple(record["directory_identity"]):
        raise Fault("directory_identity_changed")


def selected_observations(document):
    """Select typed fields afresh; backend objects must never reach the sink."""
    result = {"services": [], "processes": []}
    for kind, allowed in (("services", SERVICE_NAMES), ("processes", PROCESS_NAMES)):
        rows = document.get(kind)
        if not isinstance(rows, list) or len(rows) > 64:
            raise Fault("observation_inventory_limit")
        for row in rows:
            if not isinstance(row, dict) or row.get("name") not in allowed:
                raise Fault("observation_inventory_name")
            item = {"name": row["name"]}
            for key in ("pid", "parent"):
                if key in row:
                    if type(row[key]) is not int or not 0 <= row[key] <= 0xffffffff:
                        raise Fault("observation_inventory_pid")
                    item[key] = row[key]
            if kind == "services":
                if row.get("state") not in ("Running", "Stopped", "Start Pending", "Stop Pending", "Paused", "Unknown"):
                    raise Fault("observation_service_state")
                item["state"] = row["state"]
            elif isinstance(row.get("created"), str) and re.fullmatch(r"[0-9T:.+Z-]{20,40}", row["created"]):
                item["created"] = row["created"]
            result[kind].append(item)
    return result


class Journal:
    def __init__(self, output, identity):
        self.output = Path(output)
        self.handle = self.output.open("x+", encoding="utf-8")
        self.started = time.monotonic()
        self.data = {"schema": 1, "identity": identity, "events": [],
                     "outcome": "inconclusive", "cleanup": "not_started"}
        self.save()

    def save(self):
        try:
            encoded = json.dumps(self.data, ensure_ascii=True, indent=2)
            if len(encoded) > 1 << 20:
                self.data["sink_failed"] = True
                return
            self.handle.seek(0)
            self.handle.write(encoded)
            self.handle.truncate()
            self.handle.flush()
        except Exception:
            self.data["sink_failed"] = True

    def event(self, phase, **values):
        if len(self.data["events"]) >= 200:
            self.data["sink_failed"] = True
            return
        self.data["events"].append({"phase": phase, "utc": datetime.now(timezone.utc).isoformat(), "elapsed_ms": int((time.monotonic() - self.started) * 1000), **values})
        self.save()
        try:
            print(json.dumps({"component": "ci", "operation": "pr766_wsl_once", "phase": phase}), flush=True)
        except Exception:
            self.data["sink_failed"] = True

    def close(self):
        try:
            self.handle.close()
        except Exception:
            self.data["sink_failed"] = True


def observe_outside_lifecycle(journal, phase):
    """Optional inventory before import / after cleanup, never a test prerequisite.

    The first authorized run timed out here before reaching its baseline. Keep
    this same five-second bound, report missing evidence, and do not let an
    inventory replace the actual lifecycle result or exact-owned cleanup.
    """
    try:
        result = capture(["powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", OBSERVE_SCRIPT], 5)
        if result.code:
            raise Fault("passive_observation_failed")
        value = {"status": "available", **selected_observations(json.loads(result.out))}
    except Exception as exc:
        category = str(exc) if isinstance(exc, Fault) else type(exc).__name__
        value = {"status": "unavailable", "category": category if category in INVENTORY_ERRORS else "unrecognized_observation_error"}
        journal.data["observations_incomplete"] = True
        if getattr(exc, "process_cleanup_unconfirmed", False) or category in ("observation_termination_unconfirmed", "observation_pipe_unconfirmed"):
            value["process_cleanup"] = "unconfirmed"
            journal.data["inventory_process_cleanup_unconfirmed"] = True
    journal.event(phase, observations=value)


def verified_image(image, download, deadline):
    image = Path(image)
    created = None
    if not image.exists():
        image = Path(download)
        with urllib.request.urlopen(IMAGE_URL, timeout=30) as response, image.open("xb") as target:
            info = os.fstat(target.fileno())
            created = (info.st_dev, info.st_ino)
            if response.url != IMAGE_URL:
                raise Fault("image_redirect")
            count = 0
            while chunk := response.read(1 << 20):
                count += len(chunk)
                if count > 3 << 30 or time.monotonic() > deadline:
                    raise Fault("image_download_bound")
                target.write(chunk)
    if image.is_symlink() or getattr(image.lstat(), "st_file_attributes", 0) & 0x400:
        raise Fault("image_reparse")
    path_identity(image.parent)
    if not image.is_file() or image.stat().st_size > 3 << 30:
        raise Fault("image_file_invalid")
    before = file_identity(image)
    if created is not None and before[:2] != created:
        raise Fault("image_identity_changed")
    digest = hashlib.sha256()
    with image.open("rb") as source:
        while chunk := source.read(1 << 20):
            if time.monotonic() > deadline:
                raise Fault("image_verification_bound")
            digest.update(chunk)
    if digest.hexdigest() != IMAGE_SHA:
        raise Fault("image_hash_mismatch")
    if file_identity(image) != before:
        raise Fault("image_identity_changed")
    return image


def file_identity(path):
    info = path.lstat()
    if path.is_symlink() or getattr(info, "st_file_attributes", 0) & 0x400 or not path.is_file() or not info.st_ino or info.st_nlink != 1:
        raise Fault("file_identity_unavailable")
    return (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns)


def cleanup_files(home, home_id, target, record, image, image_id, journal):
    """Delete only this run's verified image and empty owned directories."""
    if path_identity(home) != home_id:
        raise Fault("home_identity_changed")
    if target.exists():
        if path_identity(target) != tuple(record["directory_identity"]):
            raise Fault("directory_identity_changed")
        target.rmdir()  # Refuses residual data; never recursive.
    if image.parent == home:
        if path_identity(home) != home_id:
            raise Fault("home_identity_changed")
        if file_identity(image) != image_id:
            raise Fault("image_identity_changed")
        image.unlink()
    if path_identity(home) != home_id:
        raise Fault("home_identity_changed")
    home.rmdir()  # Refuses any additional or partial-download data.
    journal.event("test_data_removed")


def listed(wsl):
    result = capture([wsl, "--list", "--quiet"], 10)
    if result.code:
        raise Fault("distribution_listing_failed")
    return [line.strip() for line in result.out.replace("\x00", "").splitlines() if line.strip()]


def cleanup(wsl, name, path, record, journal):
    journal.data["cleanup"] = "unconfirmed"
    journal.save()
    assert_owned(record, registrations(), path, path_identity(path))
    names = listed(wsl)
    if sum(value.casefold() == name.casefold() for value in names) != 1:
        raise Fault("cleanup_name_ambiguous")
    # Refresh identity after the native list observation, immediately before
    # the name-only unregister API. Trusted single-job TOCTOU limit remains.
    assert_owned(record, registrations(), path, path_identity(path))
    journal.event("unregister_started")
    result = capture([wsl, "--unregister", name], 45)
    journal.event("unregister_returned", **result.receipt())
    rows = registrations()
    absent = name.casefold() not in {value.casefold() for value in listed(wsl)}
    absent = absent and all(row["guid"] != record["registration"]["guid"] and row["name"].casefold() != name.casefold() for row in rows)
    if not path.exists():
        state = "absent"
    elif path_identity(path) != tuple(record["directory_identity"]):
        state = "changed"
    else:
        state = "empty" if not any(path.iterdir()) else "residual"
    journal.event("cleanup_observed", registration_absent=absent, directory_state=state)
    if result.code or not absent or state not in ("absent", "empty"):
        raise Fault("cleanup_partial")
    journal.data["cleanup"] = "verified"
    journal.save()


def normalized(text):
    return ANSI.sub("", text).replace("\r", "")


def version_fields(output):
    values = dict(re.findall(r"(?m)^([A-Za-z0-9 .()-]{1,40}):\s*([0-9]+(?:\.[0-9]+){1,5}(?:-[0-9]+){0,3})\s*$", output))
    selected = {key: value for key, value in values.items() if key in VERSION_FIELDS}
    if "WSL version" not in selected:
        raise Fault("wsl_version_unrecognized")
    return selected


def console_child(name, wsl):
    from winpty import PtyProcess
    # String "0" survives pywinpty's truthiness fallback and forces ConPTY.
    proc = PtyProcess.spawn([str(Path(wsl).with_name("cmd.exe")), "/d", "/q"], dimensions=(24, 100), backend="0")
    received = queue.Queue(maxsize=128)
    outcome = {"reader": "running"}

    def read():
        try:
            while True:
                received.put(proc.read(1024), timeout=1)
        except EOFError:
            outcome["reader"] = "eof"
        except Exception:
            outcome["reader"] = "failed"

    reader = threading.Thread(target=read, daemon=True)
    reader.start()
    deadline, output, stage = time.monotonic() + 45, "", 0

    def receipt(phase, **fields):
        value = {"phase": phase, "utc": datetime.now(timezone.utc).isoformat(),
                 "monotonic_ns": time.monotonic_ns(), "cmd_pid": proc.pid,
                 "observations": dict(CRITICAL_OBSERVATIONS), **fields}
        print(json.dumps(value), flush=True)

    failure = None
    try:
        # Builtin read has a finite fallback and creates no timer descendant.
        script = "trap 'echo; echo DIAG_INT; exit 0' INT; echo; echo DIAG_READY; read -r -t 15 value; echo; echo DIAG_FALLBACK; exit 9"
        command = subprocess.list2cmdline([wsl, "--distribution", name, "--user", "root", "--exec", "/bin/bash", "--noprofile", "--norc", "-c", script]) + "\r\n"
        while time.monotonic() < deadline:
            try:
                output += received.get(timeout=0.05)
            except queue.Empty:
                pass
            if len(output) > 32768:
                raise Fault("console_output_limit")
            text = normalized(output)
            if stage == 0 and re.search(r"(?m)^[A-Za-z]:\\[^\n>]*>\s*$", text):
                proc.write(command)
                stage, output = 1, ""
            elif stage == 1 and re.search(r"(?m)^DIAG_READY$", text):
                receipt("ready")
                proc.write("\x03")
                receipt("ctrl_c_sent")
                stage, output = 2, ""
            elif stage == 2 and re.search(r"(?m)^DIAG_INT$", text) and re.search(r"(?m)^[A-Za-z]:\\[^\n>]*>\s*$", text):
                receipt("wsl_returned")
                proc.write("exit\r\n")
                stage = 3
            if "\nDIAG_FALLBACK\n" in text:
                raise Fault("console_fallback")
            if stage == 3 and outcome["reader"] == "eof" and not proc.isalive():
                reader.join(timeout=1)
                if proc.exitstatus != 0 or reader.is_alive():
                    raise Fault("console_exit_unconfirmed")
                receipt("cmd_exited", exit_code=0, reader="eof")
                return
        raise Fault("console_timeout")
    except Exception as exc:
        failure = exc
        raise
    finally:
        if failure is not None:
            # Preserve the primary failure even if the native cleanup API fails.
            status = "unconfirmed"
            try:
                if not proc.isalive():
                    status = "already_exited"
                elif proc.terminate(force=True) and not proc.isalive():
                    status = "terminated"
            except Exception:
                pass
            try:
                print(json.dumps({"phase": "failure_cleanup", "state": status}), flush=True)
            except Exception:
                pass


def run_console(name, wsl, journal):
    # Only fixed phase receipts cross this child boundary, not terminal output.
    journal.data["process_cleanup"] = "unconfirmed"
    journal.save()
    result = capture([sys.executable, __file__, "--console-child", name, "--wsl", wsl], 60, limit=65536)
    expected = ["ready", "ctrl_c_sent", "wsl_returned", "cmd_exited"]
    rows = [json.loads(line) for line in result.out.splitlines()]
    pid, previous_ns = None, 0
    for index, row in enumerate(rows):
        if isinstance(row, dict) and row.get("phase") == "failure_cleanup" and result.code and index == len(rows) - 1:
            state = row.get("state")
            journal.event("console_failure_cleanup", state=state if state in ("unconfirmed", "already_exited", "terminated") else "unknown")
            continue
        if index >= len(expected) or not isinstance(row, dict) or row.get("phase") != expected[index]:
            raise Fault("console_receipt_invalid")
        phase = row["phase"]
        if type(row.get("cmd_pid")) is not int or not 0 < row["cmd_pid"] <= 0xffffffff or (pid is not None and pid != row["cmd_pid"]):
            raise Fault("console_receipt_invalid")
        pid = row["cmd_pid"]
        if type(row.get("monotonic_ns")) is not int or row["monotonic_ns"] <= previous_ns:
            raise Fault("console_receipt_invalid")
        previous_ns = row["monotonic_ns"]
        if not isinstance(row.get("utc"), str) or not re.fullmatch(r"[0-9T:.+Z-]{20,40}", row["utc"]):
            raise Fault("console_receipt_invalid")
        try:
            observed = datetime.fromisoformat(row["utc"])
            if observed.utcoffset() is None or observed.utcoffset().total_seconds() != 0:
                raise ValueError()
        except ValueError:
            raise Fault("console_receipt_invalid") from None
        if phase == "cmd_exited" and (type(row.get("exit_code")) is not int or row["exit_code"] != 0 or row.get("reader") != "eof"):
            raise Fault("console_exit_unconfirmed")
        fields = {"cmd_pid": pid, "observed_monotonic_ns": previous_ns, "observed_utc": row["utc"]}
        if phase == "cmd_exited":
            fields.update(exit_code=0, reader="eof")
        if row.get("observations") != CRITICAL_OBSERVATIONS:
            raise Fault("console_receipt_invalid")
        fields["observations"] = dict(CRITICAL_OBSERVATIONS)
        journal.event(phase, **fields)
    if result.code or len(rows) != len(expected):
        category = "unrecognized_child_error"
        try:
            error = json.loads(result.err)
            if isinstance(error, dict) and error.get("error") in CHILD_ERRORS:
                category = error["error"]
        except (ValueError, TypeError):
            pass
        journal.event("console_failed", category=category)
        raise Fault("console_sequence_failed")
    journal.data["process_cleanup"] = "normal_exit"
    journal.event("helper_exited", exit_code=result.code)


def experiment(wsl, name, journal):
    journal.event("before_baseline", observations=dict(CRITICAL_OBSERVATIONS))
    baseline = capture([wsl, "--distribution", name, "--user", "root", "--exec", "/bin/true"], 10)
    journal.event("baseline_returned", **baseline.receipt())
    if baseline.code:
        raise Fault("baseline_failed")
    run_console(name, wsl, journal)
    final = capture([wsl, "--distribution", name, "--user", "root", "--exec", "/bin/true"], 10)
    journal.event("final_launch_returned", **final.receipt())
    journal.data["outcome"] = "not_reproduced" if final.code == 0 else "reproduced" if final.receipt()["wsl_service_unexpected"] else "inconclusive"
    journal.event("after_final_launch", observations=dict(CRITICAL_OBSERVATIONS))


def require_context(env):
    if os.name != "nt" or env.get("GITHUB_REPOSITORY") != "SLktEx/Hacocoon" or env.get("GITHUB_REF") != "refs/heads/" + BRANCH or env.get("GITHUB_RUN_ATTEMPT") != "1":
        raise Fault("invocation_outside_approved_job")
    run_id = env.get("GITHUB_RUN_ID", "")
    if not re.fullmatch(r"[0-9]{1,20}", run_id):
        raise Fault("invalid_run_identity")
    return run_id


def run(args):
    global CAPTURE_DEADLINE
    env = os.environ
    run_id = require_context(env)
    started = time.monotonic()
    CAPTURE_DEADLINE = started + 480  # Reserve up to 90 seconds for cleanup.
    sha = env.get("GITHUB_SHA", "")
    image_version = env.get("ImageVersion", "")
    if not re.fullmatch(r"[a-f0-9]{40}", sha) or not re.fullmatch(r"[0-9.]{1,40}", image_version):
        raise Fault("runner_identity_invalid")
    identity = {"run": run_id, "attempt": 1, "sha": env.get("GITHUB_SHA"), "image_sha256": IMAGE_SHA, "vendor_metadata": VENDOR_METADATA}
    journal = Journal(args.output, identity)
    root = Path(env["RUNNER_TEMP"])
    path_identity(root)
    home = root / ("pr766-once-" + run_id)
    home.mkdir()
    home_id = path_identity(home)
    target = home / "distro"
    name = "HacoDiag766-" + run_id
    wsl = str(Path(env["SystemRoot"]) / "System32" / "wsl.exe")
    record = {}
    try:
        version = capture([wsl, "--version"], 10)
        if version.code:
            raise Fault("wsl_version_unavailable")
        journal.event("versions", versions=version_fields(version.out), runner_image=image_version)
        observe_outside_lifecycle(journal, "inventory_before_import")
        image = verified_image(args.image, home / "ubuntu.wsl", time.monotonic() + 240)
        image_id = file_identity(image)
        journal.event("image_verified", sha256=IMAGE_SHA)
        previous = registrations()
        if any(value.casefold() == name.casefold() for value in listed(wsl)) or any(row["name"].casefold() == name.casefold() for row in previous):
            raise Fault("fixture_already_exists")
        target.mkdir()
        directory_id = path_identity(target)
        if file_identity(image) != image_id:
            raise Fault("image_identity_changed")
        journal.event("import_started", fixture_name=name)
        result = capture([wsl, "--import", name, str(target), str(image), "--version", "2"], 120)
        journal.event("import_returned", **result.receipt())
        if result.code:
            raise Fault("import_failed")
        registration = matching_registration(registrations(), name)
        record = {"name": name, "import_succeeded": True, "pre_absent": True,
                  "registration": registration, "directory_identity": directory_id,
                  "pre_guids": [row["guid"] for row in previous]}
        assert_owned(record, registrations(), target, path_identity(target))
        journal.event("ownership_recorded", guid=registration["guid"], directory_identity=directory_id)
        experiment(wsl, name, journal)
    except Exception as exc:
        journal.data["primary_failure"] = str(exc) if isinstance(exc, Fault) else type(exc).__name__
        if getattr(exc, "process_cleanup_unconfirmed", False):
            journal.data["process_cleanup_unconfirmed"] = True
        journal.event("experiment_stopped")
    finally:
        CAPTURE_DEADLINE = started + 570
        if record:
            try:
                cleanup(wsl, name, target, record, journal)
                cleanup_files(home, home_id, target, record, image, image_id, journal)
            except Exception as exc:
                journal.data["cleanup"] = "unconfirmed"
                journal.data["cleanup_failure"] = str(exc) if isinstance(exc, Fault) else type(exc).__name__
        else:
            journal.data["cleanup"] = "not_imported_or_ownership_unconfirmed"
        observe_outside_lifecycle(journal, "inventory_after_cleanup")
        journal.event("finished")
        journal.close()
    return 0 if journal.data["outcome"] in ("not_reproduced", "reproduced") and journal.data["cleanup"] == "verified" and "primary_failure" not in journal.data and not journal.data.get("sink_failed") and not journal.data.get("inventory_process_cleanup_unconfirmed") else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output")
    parser.add_argument("--image")
    parser.add_argument("--console-child")
    parser.add_argument("--wsl")
    args = parser.parse_args()
    try:
        if args.console_child:
            if not re.fullmatch(r"HacoDiag766-[0-9]{1,20}", args.console_child):
                raise Fault("invalid_child_identity")
            console_child(args.console_child, args.wsl)
            return 0
        return run(args)
    except Exception as exc:
        print(json.dumps({"component": "ci", "operation": "pr766_wsl_once", "error": str(exc) if isinstance(exc, Fault) else type(exc).__name__}), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())

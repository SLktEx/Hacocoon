#!/usr/bin/env python3
"""Installed Linux stages on the existing dedicated Windows/WSL acceptance runner.

This is an internal integration gate, separate from the exact daily user-path
gate. The optional worker check stops/compacts/resumes only the enrolled runner
WSL. It never independently mounts or deletes storage.
"""
import json
import os
import subprocess
import uuid
import sys
import re
from pathlib import Path

def registration():
    import winreg
    found = []
    with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\Microsoft\Windows\CurrentVersion\Lxss") as root:
        for i in range(winreg.QueryInfoKey(root)[0]):
            name = winreg.EnumKey(root, i)
            with winreg.OpenKey(root, name) as key:
                distro, kind = winreg.QueryValueEx(key, "DistributionName")
                version, version_kind = winreg.QueryValueEx(key, "Version")
                if kind == winreg.REG_SZ and distro == "Hacocoon":
                    assert version_kind == winreg.REG_DWORD and version == 2
                    found.append("{" + str(uuid.UUID(name)) + "}")
    assert len(found) == 1, "exact dedicated WSL registration required"
    return found[0]

def worker_cycle(reg):
    import ctypes
    from ctypes import wintypes
    helper = Path(os.environ["LOCALAPPDATA"]) / "Hacocoon" / "reclamation" / uuid.UUID(reg).hex / "haco-wsl.exe"
    assert helper.is_file(), "packaged enrolled Windows helper missing"
    def invoke(*args):
        p = subprocess.run([str(helper), *args], capture_output=True, text=True,
                           encoding="utf-8", timeout=210, env={**os.environ, "HACO_LOG_FORMAT": "json"})
        if p.returncode != 0:
            # Select only fixed structured diagnostics; never print raw child logs.
            for line in p.stderr[:16384].splitlines():
                try:
                    record = json.loads(line)
                except (ValueError, TypeError):
                    continue
                if not isinstance(record, dict):
                    continue
                stage = record.get("stage")
                native_error = record.get("native_error")
                if stage in ("other", "pin_executable", "process_start", "readiness"):
                    diagnostic = {"worker_failure_stage": stage}
                    if type(native_error) is int and 0 < native_error <= 0xffffffff:
                        diagnostic["native_error"] = native_error
                    print(json.dumps(diagnostic), flush=True)
            if args[0] == "_launch":
                # Read-only status does not start WSL or retry a failed dispatch.
                try:
                    observed = subprocess.run([str(helper), "_status", args[1], args[2]],
                        capture_output=True, text=True, encoding="utf-8", timeout=20)
                    if observed.returncode == 0 and len(observed.stdout) <= 16384:
                        saved = json.loads(observed.stdout)
                        if isinstance(saved, dict) and saved.get("state") in ("pending", "failed", "complete"):
                            print(json.dumps({"retained_state": saved["state"],
                                "linux_started": saved.get("linux_started") is True}), flush=True)
                except (subprocess.TimeoutExpired, ValueError, OSError):
                    print("Retained worker status unavailable; outcome remains unconfirmed", flush=True)
        assert p.returncode == 0, f"Windows helper {args[0]} failed with exit {p.returncode}; retain its operation"
        assert len(p.stdout) <= 16384, "unbounded Windows helper output"
        return p.stdout
    intent = json.loads(invoke("_prepare", reg))
    assert intent["state"] == "pending"
    operation = "{" + str(uuid.UUID(intent["operation"])) + "}"
    # Launch exactly once. A startup timeout/failure never means no worker exists.
    dispatched = invoke("_launch", reg, operation)
    match = re.fullmatch(r"Dispatched Windows worker ([1-9][0-9]*); inspect the prepared operation for completion\.\s*", dispatched)
    assert match, "unrecognized worker dispatch; inspect retained operation"
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    kernel.OpenProcess.argtypes = (wintypes.DWORD, wintypes.BOOL, wintypes.DWORD)
    kernel.OpenProcess.restype = wintypes.HANDLE
    kernel.WaitForSingleObject.argtypes = (wintypes.HANDLE, wintypes.DWORD)
    kernel.WaitForSingleObject.restype = wintypes.DWORD
    kernel.CloseHandle.argtypes = (wintypes.HANDLE,)
    kernel.CloseHandle.restype = wintypes.BOOL
    handle = kernel.OpenProcess(0x00100000, False, int(match[1]))  # SYNCHRONIZE only
    if handle:
        try:
            assert kernel.WaitForSingleObject(handle, 660000) == 0, "worker not terminal; retain pending evidence, do not restart WSL"
        finally:
            kernel.CloseHandle(handle)
    else:
        assert ctypes.get_last_error() == 87, "worker state unavailable; do not restart WSL"
    # No WSL query is issued until the Windows worker has exited.
    result = json.loads(invoke("_status", reg, operation))
    assert result["state"] == "complete", f"worker terminal state {result['state']}; retain recorded failure"
    assert result.get("linux_started") and not result["linux"].get("failure")
    assert all(result["linux"][k]["status"] == "complete" for k in ("incus_btrfs_loop", "wsl_ext4"))
    windows = result["observation"]
    assert all(windows[k] for k in ("StopAttempted", "StopRequested", "ResumeAttempted", "Resumed"))
    compact = windows["Compaction"]
    assert compact["Attempted"] and compact["Completed"] and compact["Virtual"]["Capacity"] > 0
    print(json.dumps({"worker_stages": "PASS", "observations": result}))


def main():
    assert sys.argv[1:] in ([], ["--with-worker"]), "unknown test arguments"
    assert os.name == "nt", "requires the installed Windows/WSL gate"
    reg = registration()
    def run(*args, expected=0):
        p = subprocess.run(["wsl.exe", "--distribution-id", reg, "--user", "root",
                            "--cd", "/", "--exec", "/usr/bin/env", "-i",
                            "PATH=/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", *args],
                           capture_output=True, text=True, encoding="utf-8", timeout=450)
        assert p.returncode == expected, f"installed reclamation command exit {p.returncode}, expected {expected}"
        return p.stdout
    identity = json.loads(run("/usr/bin/python3", "-I", "/usr/local/libexec/hacocoon-wsl-interop", "--read-registration"))
    assert identity["registration_id"] == reg
    # Ordinary setup establishes Incus-owned Host/pool use after cold WSL entry.
    run("haco", "setup")
    # Existing installer sentinel was created through the ordinary Host session.
    marker = 'test "$(cat "$HOME/.hacocoon-installer-acceptance")" = kept-through-restart-and-rerun'
    run("incus", "exec", "haco-host", "--project", "hacocoon", "--", "sh", "-ec", marker)
    foreign = str(uuid.uuid4())
    assert foreign != identity["installation_id"]
    refused = json.loads(run("haco", "_reclaim-linux", reg, foreign, expected=1))
    assert refused["failure"] == "identity_changed"
    assert all(refused[k]["status"] == "skipped" and not refused[k]["attempted"] for k in ("incus_btrfs_loop", "wsl_ext4"))
    report = json.loads(run("haco", "_reclaim-linux", reg, identity["installation_id"]))
    assert report["protocol_version"] == 1 and not report.get("failure") and not report.get("cleanup_failed")
    for k in ("incus_btrfs_loop", "wsl_ext4"):
        assert report[k]["status"] == "complete" and report[k]["attempted"]
        assert type(report[k]["kernel_trimmed_bytes"]) is int and report[k]["kernel_trimmed_bytes"] >= 0
        assert report[k]["filesystem_before"]["capacity_bytes"] == report[k]["filesystem_after"]["capacity_bytes"] > 0
        for field in ("filesystem_before", "filesystem_after"):
            assert 0 <= report[k][field]["used_bytes"] <= report[k][field]["capacity_bytes"]
    pool = report["incus_btrfs_loop"]
    assert pool["before"]["logical_bytes"] == pool["after"]["logical_bytes"] > 0
    assert "before" not in report["wsl_ext4"] and "after" not in report["wsl_ext4"]
    run("incus", "exec", "haco-host", "--project", "hacocoon", "--", "sh", "-ec", marker)
    print(json.dumps({"linux_stages": "PASS", "foreign_identity": "refused",
                      "host_sentinel": "retained", "observations": report}))
    if sys.argv[1:] == ["--with-worker"]:
        worker_cycle(reg)
        # Worker resume proves exact WSL startup; ordinary setup proves Host readiness.
        run("haco", "setup")
        run("incus", "exec", "haco-host", "--project", "hacocoon", "--", "sh", "-ec", marker)
        print("Worker Host sentinel and ordinary setup after resume: PASS")
    else:
        print("Windows VHDX compaction: not exercised by this gate")
    print("Whole Workspace/OCI persistent-data acceptance: not exercised by this gate")

if __name__ == "__main__":
    main()

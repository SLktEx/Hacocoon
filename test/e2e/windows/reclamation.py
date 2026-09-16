#!/usr/bin/env python3
"""Exercise public reclaim through the installed ordinary Host terminal.

Only Windows read-only observations run between dispatch and worker exit. No
retry, record clearing, Job changes or internal mutation commands are used.
"""
import argparse
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


def failure_summary(result):
    # Fixed vocabulary/booleans only: never emit arbitrary child fields or logs.
    if not isinstance(result, dict):
        return {"worker_result": "unrecognized"}
    allowed = {"identity_unavailable", "identity_changed", "pool_unavailable", "pool_trim_failed", "outer_trim_failed", "cleanup_failed", "canceled"}
    linux = result.get("linux")
    linux = linux if isinstance(linux, dict) else {}
    windows = result.get("observation")
    windows = windows if isinstance(windows, dict) else {}
    def boolean(value):
        return value if type(value) is bool else None
    def stage(key):
        value = linux.get(key)
        value = value.get("status") if isinstance(value, dict) else None
        return value if value in ("complete", "failed", "skipped") else "unrecorded"
    reason = linux.get("failure")
    compact = windows.get("Compaction")
    compact = compact if isinstance(compact, dict) else {}
    linux_failure = "unrecorded"
    if isinstance(result.get("linux"), dict):
        linux_failure = "none" if reason in (None, "") else reason if isinstance(reason, str) and reason in allowed else "unrecognized"
    return {"worker_result": result.get("state") if result.get("state") in ("pending", "failed", "complete", "interrupted") else "unrecognized",
            "linux_started": boolean(result.get("linux_started")),
            "linux_report_present": isinstance(result.get("linux"), dict),
            "linux_failure": linux_failure,
            "linux_pool": stage("incus_btrfs_loop"), "linux_outer": stage("wsl_ext4"),
            "windows_stop_attempted": boolean(windows.get("StopAttempted")),
            "windows_stop_requested": boolean(windows.get("StopRequested")),
            "windows_failure": windows.get("Failure") if windows.get("Failure") in ("stop", "compact", "compact_attached", "resume") else "unrecorded",
            "windows_native_error": windows.get("NativeError") if type(windows.get("NativeError")) is int and 0 < windows["NativeError"] <= 0xffffffff else None,
            "windows_open_attempts": compact.get("OpenAttempts") if type(compact.get("OpenAttempts")) is int and 0 <= compact["OpenAttempts"] <= 10000 else None,
            "windows_compaction_attempted": boolean(compact.get("Attempted")),
            "windows_resumed": boolean(windows.get("Resumed"))}


def read_json(command):
    result = subprocess.run(command, capture_output=True, timeout=25)
    if result.returncode or len(result.stdout) > 16384:
        raise RuntimeError("Windows read-only observation failed")
    return json.loads(result.stdout.decode("utf-8-sig"))


def process_query():
    # Windows process metadata only: do not enter WSL to observe its shutdown.
    # Existing helper identity checks still receive only helper executable paths.
    return """[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false);$ErrorActionPreference='Stop';
$rows=@(Get-CimInstance Win32_Process -Property Name,ExecutablePath,ProcessId,ParentProcessId,CreationDate);
$helpers=@($rows | Where-Object { $_.Name -eq 'haco-wsl.exe' } | Select-Object ExecutablePath);
$counts=@{};foreach($name in @('wslhost.exe','wsl.exe','vmmemWSL')) { $counts[$name]=@($rows | Where-Object { $_.Name -eq $name }).Count };
$byId=@{};foreach($row in $rows) { $byId[[string]$row.ProcessId]=$row };
$classes=@{'wsl.exe'='wsl';'haco-wsl.exe'='reclamation';'ssh.exe'='ssh';'Code.exe'='editor';'cmd.exe'='shell';'powershell.exe'='powershell';'pwsh.exe'='powershell';'python.exe'='python';'python3.exe'='python';'WindowsTerminal.exe'='terminal';'OpenConsole.exe'='terminal';'conhost.exe'='terminal';'wslservice.exe'='service';'haco-notify.exe'='notification'};
foreach($name in @('haco-review.exe','haco-notify.exe')) { $classes[$name]='notification' };
foreach($name in @('haco-vscode.exe','haco-agent-host.exe','haco-tunnel.exe')) { $classes[$name]='hacocoon-client' };
$classes['wslhost.exe']='wsl-host';$classes['wslrelay.exe']='wsl-relay';
foreach($name in @('explorer.exe','svchost.exe','services.exe','taskhostw.exe','taskeng.exe','RuntimeBroker.exe','WmiPrvSE.exe','SearchIndexer.exe','dllhost.exe')) { $classes[$name]='windows-service' };
$origins=@{};$hostOrigins=@{};
foreach($row in @($rows | Where-Object { $_.Name -in @('wsl.exe','wslhost.exe') })) {
    $chain=@();$child=$row;$seen=@{};
    for($depth=0;$depth -lt 8;$depth++) {
        $parent=$byId[[string]$child.ParentProcessId];
        if($null -eq $parent -or $seen.ContainsKey([string]$parent.ProcessId) -or
           $null -eq $parent.CreationDate -or $null -eq $child.CreationDate -or
           $parent.CreationDate -gt $child.CreationDate) { $chain+='unavailable';break };
        $seen[[string]$parent.ProcessId]=$true;
        $kind=$classes[[string]$parent.Name];
        if($null -eq $kind) { $chain+='other';break };
        $chain+=$kind;$child=$parent;
    };
    $key=$chain -join '/';
    $selected=if($row.Name -eq 'wsl.exe') { $origins } else { $hostOrigins };
    if($selected.ContainsKey($key)) { $selected[$key]++ } else { $selected[$key]=1 };
};
ConvertTo-Json -Compress -Depth 3 -InputObject @{ helpers=$helpers; counts=$counts; origins=$origins; host_origins=$hostOrigins }
"""


def observed_process_counts(snapshot):
    # Counts describe the Windows session as a whole, not the selected distro.
    # They diagnose timing only and never establish stop/detach or mutation authority.
    keys = ("wslhost.exe", "wsl.exe", "vmmemWSL")
    counts = snapshot.get("counts") if isinstance(snapshot, dict) else None
    if not isinstance(counts, dict) or set(counts) != set(keys) or not all(type(counts[k]) is int and 0 <= counts[k] <= 4096 for k in keys):
        return {"state": "unavailable"}
    return {"state": "observed", **{key: counts[key] for key in keys}}


def observed_process_origins(snapshot, process="wsl.exe"):
    # Parent names are categories, not executable/ownership authentication. The
    # Windows snapshot omits missing/reused parents and never emits names or PIDs.
    kinds = {"wsl", "reclamation", "ssh", "editor", "shell", "powershell",
             "python", "terminal", "service", "notification", "hacocoon-client",
             "wsl-host", "wsl-relay", "windows-service", "other", "unavailable"}
    field = {"wsl.exe": "origins", "wslhost.exe": "host_origins"}.get(process)
    origins = snapshot.get(field) if isinstance(snapshot, dict) and field else None
    counts = observed_process_counts(snapshot)
    if not isinstance(origins, dict) or len(origins) > 64 or counts["state"] != "observed":
        return {"state": "unavailable"}
    for chain, count in origins.items():
        if (not isinstance(chain, str) or len(chain) > 128 or
                not 1 <= len(chain.split("/")) <= 8 or
                any(kind not in kinds for kind in chain.split("/")) or
                type(count) is not int or not 1 <= count <= 4096):
            return {"state": "unavailable"}
    if sum(origins.values()) != counts[process]:
        return {"state": "unavailable"}
    return {"state": "observed", "chains": dict(sorted(origins.items()))}


def wait_for_worker(helper, registration, operation):
    powershell = str(Path(os.environ["SystemRoot"]) / "System32/WindowsPowerShell/v1.0/powershell.exe")
    script = process_query()
    started = time.monotonic()
    previous_counts = None

    def observe_processes():
        nonlocal previous_counts
        snapshot = read_json([powershell, "-NoProfile", "-NonInteractive", "-Command", script])
        counts = {"processes": observed_process_counts(snapshot),
                  "origins": observed_process_origins(snapshot),
                  "host_origins": observed_process_origins(snapshot, "wslhost.exe")}
        if counts != previous_counts:
            print(json.dumps({"component": "ci", "operation": "reclamation_windows_processes",
                              "duration_ms": int((time.monotonic() - started) * 1000),
                              "scope": "all-windows-wsl-processes", "counts": counts["processes"],
                              "origins": counts["origins"],
                              "host_origins": counts["host_origins"]}), flush=True)
            previous_counts = counts
        return snapshot.get("helpers") if isinstance(snapshot, dict) else None

    def observe_result():
        result = read_json([str(helper), "_status", registration, operation])
        try:
            complete = require_complete(result, operation)
        except (RuntimeError, TypeError, AttributeError):
            print(json.dumps(failure_summary(result)), flush=True)
            try:
                observe_processes()
            except (OSError, subprocess.TimeoutExpired, ValueError, TypeError, RuntimeError):
                # Diagnostic failure must not replace the original worker failure.
                print(json.dumps({"component": "ci", "operation": "reclamation_windows_processes",
                                  "state": "unavailable"}), flush=True)
            raise
        return result, complete

    deadline = time.monotonic() + 690
    while time.monotonic() < deadline:
        # These commands never enter WSL, mutate records or launch a worker.
        result, complete = observe_result()
        rows = observe_processes()
        running = helper_is_running(rows, helper)
        if not running and not complete:
            # Completion can be persisted between the status read and process
            # observation. Read once after exit before declaring it unknown.
            result, complete = observe_result()
        if complete and not running:
            print(json.dumps({"public_worker": "PASS", "observations": result}), flush=True)
            return
        if not running and not complete:
            raise RuntimeError("Worker absent with pending outcome; retain evidence")
        time.sleep(5)
    raise RuntimeError("Worker completion unknown; no WSL restart or retry")


def load_driver(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).parent / filename)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


def require_absent_history(result):
    if result != {"operation": "", "state": "none"}:
        raise RuntimeError("Fresh installation has saved or unrecognized reclamation evidence; retain it")


def absent_status_completed(output):
    # A prompt can be repainted before the command runs. Only its explicit
    # completion line establishes that the status response has arrived.
    match = re.search(r"(?m)^HACO_ABSENT_STATUS_EXIT:([0-9]+)\r?\n", output)
    if not match:
        return False
    if match[1] != "0":
        raise RuntimeError("Ordinary Host reclamation status failed")
    response = output[:match.start()]
    if not any(text in response for text in ("No saved reclamation result. No operation was started by this status check.",
                                            "容量回収の保存結果はありません。この確認で新しい操作は開始していません。")):
        raise RuntimeError("Ordinary Host did not report absent reclamation history")
    return True


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--retention-manifest", required=True)
    args = parser.parse_args()
    if os.name != "nt":
        raise RuntimeError("requires installed Windows acceptance without overrides")
    driver = load_driver("reclamation_user_driver", "install.py")
    installed = load_driver("reclamation_registration", "reclamation_worker.py")
    reg = installed.registration()
    helper = Path(os.environ["LOCALAPPDATA"]) / "Hacocoon/reclamation" / uuid.UUID(reg).hex / "haco-wsl.exe"
    if not helper.is_file():
        raise RuntimeError("Installed helper missing")
    reclamation_retention = load_driver("reclamation_retention", Path(__file__).resolve().parents[3] / "tools/reclamation_retention.py")
    retention = reclamation_retention.load_manifest(args.retention_manifest)
    require_absent_history(read_json([str(helper), "_status", reg]))
    terminal = driver.TerminalProcess()
    stage, sent_at = 0, 0
    terminal_confirmed = False

    def drive(output, process):
        nonlocal stage, sent_at, terminal_confirmed
        fresh = output[sent_at:]
        if stage in (1, 4):
            driver.reject_failed_host_entry(fresh)
        host = re.search(r"(?m)^[^\r\n]*@haco-host:[^\r\n]*[#\$]\s*$", fresh)
        if stage == 0 and driver.cmd_prompt_count(fresh):
            process.write("wsl -d Hacocoon\r\n")
            stage, sent_at = 1, len(output)
        elif stage == 1 and host:
            process.write("haco reclaim --status; printf '\\nHACO_ABSENT_STATUS_EXIT:%s\\n' \"$?\"\r\n")
            stage, sent_at = 10, len(output)
        elif stage == 10 and absent_status_completed(fresh):
            require_absent_history(read_json([str(helper), "_status", reg]))
            print("Public status before first reclamation: PASS; no operation record created", flush=True)
            process.write("haco reclaim --yes\r\n")
            stage, sent_at = 2, len(output)
        elif stage == 2 and any(text in fresh for text in ("Worker dispatched; reclamation is not yet confirmed.", "容量回収の処理を起動しました。完了はまだ確認できていません。")):
            match = re.search(r"(?:Operation:|操作の記録:) (\{[0-9a-fA-F-]{36}\})", fresh)
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
        elif stage == 2 and any(text in fresh for text in ("could not be confirmed", "Dispatch result unavailable", "Managed Windows installation unavailable",
                                                          "Windowsの容量回収結果を確認できません。", "起動結果を確認できません。", "Windowsの導入情報を確認できません。")):
            raise RuntimeError("Public dispatch failed; no retry or WSL reentry")
        elif stage == 3 and driver.cmd_prompt_count(fresh):
            process.write("wsl -d Hacocoon\r\n")
            stage, sent_at = 4, len(output)
        elif stage == 4 and host:
            process.write("haco reclaim --status && test \"$(cat $HOME/.hacocoon-installer-acceptance)\" = kept-through-restart-and-rerun && printf 'PUBLIC_RECLAIM_STATUS_OK\\n'\r\n")
            stage, sent_at = 5, len(output)
        elif stage == 5 and re.search(r"(?m)^PUBLIC_RECLAIM_STATUS_OK\s*$", fresh):
            if not any(text in fresh for text in ("Reclamation: complete", "容量回収: 完了")):
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
    reclamation_retention.verify_installed(retention, reg)


if __name__ == "__main__":
    main()

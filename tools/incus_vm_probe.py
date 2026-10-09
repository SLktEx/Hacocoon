#!/usr/bin/env python3
"""Probe an optional Incus VM on an already initialized, isolated local daemon."""
import argparse
import errno
import fcntl
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tempfile
import time
import uuid

NAME = re.compile(r"hci-[0-9]{1,20}-[0-9]{1,10}-vm\Z")
TOKEN = re.compile(r"[0-9a-f]{32}\Z")
SCALAR = re.compile(r"[A-Za-z0-9_. :+|()/=-]{1,256}\Z")
OWNER = "user.hacocoon.vm-probe"
# Incus 7.0.1 instance_utils.go + driver_qemu.go + client/util.go. This narrow, source-grounded
# fallback is deliberately NOT a generic search for 'KVM' in arbitrary errors.
MISSING_KVM = re.compile(
    r'Error: (?:Failed instance creation: )?Instance type "virtual-machine" '
    r'is not supported on this server: KVM support is missing \(no /dev/kvm\)\Z')


class ProbeError(Exception):
    pass


def command(args, timeout=30):
    try:
        result = subprocess.run(["incus", *args], capture_output=True, text=True,
                                timeout=timeout, stdin=subprocess.DEVNULL, env=dict(os.environ, LC_ALL="C"))
    except subprocess.TimeoutExpired as exc:
        raise ProbeError("command_timeout") from exc
    except OSError as exc:
        raise ProbeError("command_unavailable") from exc
    if len(result.stdout) + len(result.stderr) > 1 << 20:
        raise ProbeError("command_output_too_large")
    return result


def checked(args, timeout=30):
    result = command(args, timeout)
    if result.returncode:
        raise ProbeError("command_failed")
    return result.stdout


def query(path):
    try:
        response = json.loads(checked(["query", "--raw", path]))
    except (ValueError, RecursionError) as exc:
        raise ProbeError("invalid_json") from exc
    # Normal query output suppresses {} metadata; --raw preserves it, but can
    # exit zero for an API error. Require a successful synchronous envelope.
    if (not isinstance(response, dict) or response.get("type") != "sync"
            or type(response.get("status_code")) is not int or response["status_code"] != 200
            or type(response.get("error_code")) is not int or response["error_code"] != 0
            or response.get("error") != "" or "metadata" not in response):
        raise ProbeError("invalid_api_response")
    return response["metadata"]


def kvm_observation():
    """Permissions refer to this client, not the root Incus daemon."""
    result = {}
    try:
        device = os.stat("/dev/kvm")
        result.update(state="present", character_device=stat.S_ISCHR(device.st_mode),
                      mode=oct(stat.S_IMODE(device.st_mode)), uid=device.st_uid,
                      gid=device.st_gid, client_read_write=os.access("/dev/kvm", os.R_OK | os.W_OK))
    except OSError as exc:
        return {"state": "missing" if exc.errno == errno.ENOENT else "observation_error",
                "errno": exc.errno}
    if not result["character_device"]:
        return result
    try:
        fd = os.open("/dev/kvm", os.O_RDWR | os.O_CLOEXEC)
        try:
            # Linux KVM_GET_API_VERSION, documented to return 12. Diagnostic
            # only: actual Incus launch/exec determines support, not this client.
            result["client_api_version"] = fcntl.ioctl(fd, 0xAE00, 0)
        finally:
            os.close(fd)
    except OSError as exc:
        result["client_errno"] = exc.errno
    return result


def cpu_observation():
    try:
        words = set(Path("/proc/cpuinfo").read_text().split())
        return {"vmx": "vmx" in words, "svm": "svm" in words}
    except OSError:
        return {"state": "observation_error"}


def server_observation(server):
    if not isinstance(server, dict) or not isinstance(server.get("environment"), dict):
        raise ProbeError("invalid_server")
    environment = server["environment"]
    required = ("server_version", "kernel", "kernel_version", "kernel_architecture", "os_name", "os_version")
    for key in required:
        value = environment.get(key)
        if not isinstance(value, str) or not SCALAR.fullmatch(value):
            raise ProbeError("invalid_server_identity")
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", environment["server_version"]):
        raise ProbeError("invalid_server_identity")
    return {key: value for key, value in environment.items()
            if key in (*required, "driver", "driver_version")
            and isinstance(value, str) and SCALAR.fullmatch(value)}


def instance(name):
    rows = query("/1.0/instances?project=default&recursion=1")
    if not isinstance(rows, list) or len(rows) > 1000:
        raise ProbeError("invalid_inventory")
    names = set()
    found = None
    for row in rows:
        if not isinstance(row, dict) or not isinstance(row.get("name"), str) or row["name"] in names:
            raise ProbeError("invalid_inventory")
        names.add(row["name"])
        if row["name"] == name:
            found = row
    return found


def owned(row, receipt):
    if (not isinstance(row, dict) or row.get("name") != receipt["name"]
            or row.get("type") != "virtual-machine"
            or not isinstance(row.get("config"), dict)
            or row["config"].get(OWNER) != receipt["owner"]):
        raise ProbeError("ownership_mismatch")
    return row


def validate_receipt(receipt):
    if (not isinstance(receipt, dict) or not isinstance(receipt.get("name"), str)
            or not NAME.fullmatch(receipt["name"])
            or not isinstance(receipt.get("owner"), str) or not TOKEN.fullmatch(receipt["owner"])
            or type(receipt.get("launch_attempted")) is not bool):
        raise ProbeError("invalid_receipt")


def wait_for_operations():
    # A killed CLI can leave an asynchronous daemon operation alive. Never
    # infer safe absence while that operation can still create the instance.
    deadline = time.monotonic() + 90
    while True:
        groups = query("/1.0/operations?project=default&recursion=1")
        if not isinstance(groups, dict):
            raise ProbeError("invalid_operations")
        active = False
        for rows in groups.values():
            if not isinstance(rows, list):
                raise ProbeError("invalid_operations")
            for row in rows:
                if not isinstance(row, dict) or type(row.get("status_code")) is not int:
                    raise ProbeError("invalid_operations")
                if not 100 <= row["status_code"] < 600:
                    raise ProbeError("invalid_operations")
                active |= row["status_code"] < 200
        if not active:
            return
        if time.monotonic() >= deadline:
            raise ProbeError("operations_still_active")
        time.sleep(2)


def cleanup(receipt):
    validate_receipt(receipt)
    if not receipt["launch_attempted"]:
        return
    wait_for_operations()
    row = instance(receipt["name"])
    if row is not None:
        owned(row, receipt)
        checked(["delete", receipt["name"], "--project=default", "--force"], 90)
    wait_for_operations()
    if instance(receipt["name"]) is not None:
        raise ProbeError("cleanup_instance_remains")


def wait_for_guest(receipt):
    deadline = time.monotonic() + 180
    while True:
        owned(instance(receipt["name"]), receipt)
        result = command(["exec", receipt["name"], "--project=default", "--", "sh", "-c",
                          'test "$(cat /proc/1/comm)" = systemd || exit 1; '
                          'state=$(systemctl is-system-running); '
                          'case "$state" in running|degraded) printf "vm-systemd-ready\\n";; *) exit 1;; esac'])
        if result.returncode == 0 and result.stdout.strip() == "vm-systemd-ready":
            return
        if time.monotonic() >= deadline:
            raise ProbeError("guest_not_ready")
        time.sleep(2)


def save(path, receipt):
    # Unique, exclusively created temporary file, including in shared /tmp.
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, prefix=".incus-vm-probe-", delete=False) as target:
        temporary = Path(target.name)
        try:
            target.write(json.dumps(receipt, indent=2) + "\n")
            target.flush()
            os.fsync(target.fileno())
            temporary.replace(path)
            directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
            try:
                os.fsync(directory)
            finally:
                os.close(directory)
        finally:
            temporary.unlink(missing_ok=True)


def run_probe(name, output):
    if not NAME.fullmatch(name):
        raise ProbeError("invalid_name")
    # Never overwrite an earlier ownership receipt, including an interrupted run.
    with output.open("x") as target:
        target.write("{}\n")
    receipt = {"schema": 1, "name": name, "owner": uuid.uuid4().hex,
               "launch_attempted": False, "status": "error", "stage": "preflight",
               "cleanup": "not_needed", "future_vm_backend": "inconclusive",
               "kvm": kvm_observation(), "cpu": cpu_observation()}
    save(output, receipt)
    try:
        if instance(name) is not None:
            raise ProbeError("name_already_exists")
        receipt["server"] = server_observation(query("/1.0"))
        pool = checked(["profile", "device", "get", "default", "root", "pool", "--project=default"]).strip()
        if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,62}", pool):
            raise ProbeError("invalid_root_pool")
        receipt.update(stage="launch", launch_attempted=True, cleanup="pending")
        save(output, receipt)  # Durable reservation before any fallible creation.
        # Root disk only: no bridge, Host mount, control socket, or container fallback.
        launched = command(["launch", "images:ubuntu/26.04", name, "--vm", "--quiet", "--no-profiles",
                            "--storage", pool, "--project=default", "-c", "limits.cpu=2",
                            "-c", "limits.memory=1GiB", "-c", OWNER + "=" + receipt["owner"]], 600)
        receipt["launch_exit_code"] = launched.returncode
        if launched.returncode:
            if (launched.returncode == 1 and receipt["kvm"]["state"] == "missing"
                    and MISSING_KVM.fullmatch(launched.stderr.strip())):
                receipt.update(status="unsupported", reason="missing_kvm_device",
                               future_vm_backend="unavailable_on_this_runner")
            else:
                raise ProbeError("launch_failed")
        else:
            row = owned(instance(name), receipt)
            fingerprint = row["config"].get("volatile.base_image", "")
            if re.fullmatch(r"[0-9a-f]{64}", fingerprint):
                receipt["image_fingerprint"] = fingerprint
            receipt["stage"] = "guest"
            wait_for_guest(receipt)
            receipt["stage"] = "stop"
            checked(["stop", name, "--project=default", "--timeout=60"], 90)
            if owned(instance(name), receipt).get("status") != "Stopped":
                raise ProbeError("stop_not_observed")
            receipt["stage"] = "restart"
            checked(["start", name, "--project=default"], 90)
            wait_for_guest(receipt)
            receipt.update(status="supported", guest_systemd="ready_after_restart",
                           future_vm_backend="feasible_on_this_runner_not_product_acceptance")
    except ProbeError as exc:
        receipt.update(status="error", reason=str(exc), future_vm_backend="inconclusive")
    finally:
        if receipt["launch_attempted"]:
            try:
                cleanup(receipt)
                receipt["cleanup"] = "verified_absent"
            except ProbeError as exc:
                receipt.update(status="error", cleanup=str(exc), future_vm_backend="inconclusive")
        save(output, receipt)
    print(json.dumps(receipt, sort_keys=True))
    return 0 if receipt["status"] in ("supported", "unsupported") else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("probe", "cleanup"))
    parser.add_argument("--name", default="")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        if args.action == "probe":
            return run_probe(args.name, args.output)
        if not args.output.exists():
            return 0  # Setup failed before this probe reserved any resource.
        receipt = json.loads(args.output.read_text())
        cleanup(receipt)
        return 0
    except (ProbeError, OSError, ValueError, RecursionError):
        print("Incus VM probe: observation or receipt failed; support is inconclusive", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())

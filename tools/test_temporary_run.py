"""Observe ordinary product run cleanup on the disposable real-Incus GHA fixture."""
import json
import os
import re
import signal
import subprocess
import sys
import time

if os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("HACO_CI_RUNNER_ENVIRONMENT") != "github-hosted":
    raise SystemExit("temporary run acceptance requires the disposable GHA host")
product, workspace = sys.argv[1:]
def invoke(*args, expected=0):
    result = subprocess.run([product, *args], text=True, capture_output=True, timeout=300)
    if result.returncode != expected:
        raise RuntimeError("ordinary product command returned an unexpected status")
    return result.stdout
def rows():
    return {row["name"]: row for row in json.loads(invoke("env", "list", "--json"))}
def absent(name):
    if name in rows():
        return False
    result = subprocess.run(["incus", "list", "haco-" + name, "--project", "hacocoon", "--format", "csv", "-c", "n"], capture_output=True, text=True, timeout=15)
    if result.returncode:
        raise RuntimeError("could not verify provider absence")
    return ("haco-" + name) not in result.stdout.splitlines()

baseline = rows()
result = json.loads(invoke("run", "--rm", "--json", "--", "sh", "-ec", "test \"$PWD\" = /workspace; printf temporary-ok"))
assert result["execution"]["stdout"] == "temporary-ok" and result["cleaned_up"] is True
assert absent(result["environment"])
failed = json.loads(invoke("run", "--json", "--", "sh", "-c", "exit 17", expected=17))
assert failed["execution"]["exit_code"] == 17 and failed["cleaned_up"] is True and absent(failed["environment"])
invoke("run", "--workspace", workspace, "--", "sh", "-ec", "printf retained > /workspace/temporary-run-retained")
with open(os.path.join(workspace, "temporary-run-retained"), encoding="utf-8") as stream:
    assert stream.read() == "retained"

process = subprocess.Popen([product, "run", "--rm", "--", "sh", "-ec", "printf started > /workspace/temporary-run-started; exec sleep 600"], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
try:
    target = None
    deadline = time.monotonic() + 300
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError("temporary execution ended before cancellation")
        current = rows()
        new = {name: row for name, row in current.items() if name not in baseline}
        if len(new) == 1:
            name, row = next(iter(new.items()))
            if not re.fullmatch(r"run-[a-f0-9]{16}", name) or not re.fullmatch(r"temporary:[a-f0-9]{32}", row["workspace"]["path"]):
                raise RuntimeError("unexpected temporary observation target")
            probe = subprocess.run(["incus", "exec", "haco-" + name, "--project", "hacocoon", "--", "/bin/cat", "/workspace/temporary-run-started"], capture_output=True, text=True, timeout=15)
            if probe.returncode == 0 and probe.stdout == "started":
                target = name
                break
        time.sleep(1)
    if target is None:
        raise RuntimeError("temporary command did not reach its execution marker")
    process.send_signal(signal.SIGINT)
    process.communicate(timeout=30)
    if process.returncode != 130:
        raise RuntimeError("canceled product command did not report cancellation")
    deadline = time.monotonic() + 60
    while not absent(target):
        if time.monotonic() >= deadline:
            raise RuntimeError("canceled temporary runtime cleanup was not confirmed")
        time.sleep(1)
    assert rows() == baseline, "retained Environment metadata changed"
    print("TEMPORARY PRODUCT RUN / EXIT 17 / RETAINED WORKSPACE / CANCELLATION CLEANUP: PASS")
finally:
    if process.poll() is None:
        process.send_signal(signal.SIGINT)
        process.communicate(timeout=30)

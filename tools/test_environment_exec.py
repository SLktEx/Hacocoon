"""Ordinary persistent Environment execution on a disposable real-Incus host."""
import json
import os
import secrets
import signal
import subprocess
import sys
import time
from environment_exec_stream import verify_streaming

if os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("HACO_CI_RUNNER_ENVIRONMENT") != "github-hosted":
    raise SystemExit("Environment acceptance requires the disposable GHA host")
product, image = sys.argv[1:]
name = "exec-" + secrets.token_hex(5)
def invoke(*args, expected=0):
    result = subprocess.run([product, *args], text=True, capture_output=True, timeout=300)
    if result.returncode != expected:
        raise RuntimeError("ordinary product command returned an unexpected status")
    return result.stdout
def rows():
    return {row["name"]: row for row in json.loads(invoke("env", "ls", "--json"))}
baseline = rows()
created = False
try:
    invoke("open", "--new", image, "--name", name, "--client", "none")
    created = True
    assert invoke("exec", name, "--", "sh", "-ec", 'test "$PWD" = /workspace; printf exec-ok') == "exec-ok"
    invoke("exec", name, "--", "sh", "-c", "exit 17", expected=17)
    verify_streaming(product, name, rows)
    process = subprocess.Popen([product, "exec", name, "--", "sh", "-ec", "printf started; exec sleep 600"], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    try:
        assert process.stdout.read(7) == b"started"
        process.send_signal(signal.SIGINT)
        process.communicate(timeout=30)
        assert process.returncode == 130
        assert name in rows(), "exec cancellation removed the Environment"
    finally:
        if process.poll() is None:
            process.terminate()
            process.communicate(timeout=30)
    invoke("stop", name)
    invoke("exec", name, "--", "true", expected=1)
finally:
    if created:
        invoke("rm", "-f", name)
assert rows() == baseline
print("ENVIRONMENT EXEC / EXIT 17 / STREAM / CANCEL / STOPPED REFUSAL: PASS")

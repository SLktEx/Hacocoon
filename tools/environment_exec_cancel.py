"""Exact execution-lifetime observation for the real-Incus product fixture."""

import json
import os
import re
import select
import signal
import subprocess
import sys
import time


# A builtin read uses the shell's own PID, with no short-lived cat/subshell PID.
# Strip through the last ') ' because the comm field can contain spaces and ')'.
# Field 22 (starttime) is field 20 after comm. Disable globbing before splitting.
STAT_READER = r'''
set -efu
read_stat() {
    IFS= read -r stat < "$1" || return 1
    case "$stat" in "$2 ("*") "*) ;; *) return 1 ;; esac
    set -- ${stat##*) }
    [ "$#" -ge 20 ] || return 1
    state=$1
    case "$state" in R|S|D|Z|T|t|X|x|K|W|P|I) ;; *) return 1 ;; esac
    shift 19
    start=$1
    case "$start" in ''|*[!0-9]*) return 1 ;; esac
}
'''

# exec preserves this PID/starttime across the shell -> sleep transition. The
# receipt proves the execution started; it does not claim sleep already entered.
START_COMMAND = STAT_READER + r'''
read_stat /proc/1/stat 1 || exit 90
init_start=$start
read_stat /proc/$$/stat "$$" || exit 90
printf 'EXEC-IDENTITY %s %s %s\n' "$$" "$start" "$init_start"
exec sleep 600
'''

# Observation only: never signal a PID, stop an Env, or search for a substitute.
# PID reuse proves the original is gone; a same-identity zombie is still present.
# Validate PID1 after observing the target, so restart cannot prove cancellation.
OBSERVE_COMMAND = STAT_READER + r'''
pid=$1
expected_start=$2
expected_init=$3
result=unknown
if [ ! -d "/proc/$pid" ]; then
    result=absent
elif read_stat "/proc/$pid/stat" "$pid"; then
    if [ "$start" != "$expected_start" ]; then
        result=reused
    elif [ "$state" = Z ]; then
        result=zombie
    else
        result=present
    fi
elif [ ! -d "/proc/$pid" ]; then
    result=absent
fi
read_stat /proc/1/stat 1 || exit 90
if [ "$start" != "$expected_init" ]; then
    result=environment_changed
fi
printf '%s\n' "$result"
'''


def remaining(deadline):
    value = deadline - time.monotonic()
    if value <= 0:
        raise RuntimeError("exec cancellation observation timed out")
    return value


def cleanup_process(process, primary):
    """Clean up only our Popen child, preserving any earlier failed assertion."""
    failed = False
    try:
        if process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=30)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=10)
    except (OSError, subprocess.TimeoutExpired):
        failed = True
    try:
        process.stdout.close()
    except OSError:
        failed = True
    if failed:
        if primary is None:
            raise RuntimeError("exec fixture local process cleanup failed") from None
        primary.add_note("exec fixture local process cleanup also failed")


def run_product(product, *args, expected=0, timeout=300, limit=1024 * 1024):
    """Bound captured stdout; unused guest stderr never enters diagnostics."""
    deadline = time.monotonic() + timeout
    process = None
    try:
        process = subprocess.Popen([product, *args], stdout=subprocess.PIPE,
                                   stderr=subprocess.DEVNULL, bufsize=0)
        output = bytearray()
        while True:
            ready, _, _ = select.select([process.stdout], [], [], remaining(deadline))
            if not ready:
                raise RuntimeError("ordinary product command timed out")
            part = os.read(process.stdout.fileno(), min(4096, limit + 1 - len(output)))
            if not part:
                break
            output.extend(part)
            if len(output) > limit:
                raise RuntimeError("ordinary product output exceeded its bound")
        process.wait(timeout=remaining(deadline))
        if process.returncode != expected:
            raise RuntimeError("ordinary product command returned an unexpected status")
        remaining(deadline)
        return output.decode("utf-8", errors="strict")
    except (OSError, subprocess.TimeoutExpired, UnicodeError):
        raise RuntimeError("ordinary product command or output failed") from None
    finally:
        if process is not None:
            cleanup_process(process, sys.exc_info()[1])


def cleanup_environment(environment, original, invoke, rows, primary):
    """Point-in-time ownership check; the name-based CLI has no atomic compare/delete."""
    try:
        if original is None:
            raise RuntimeError("exec fixture cleanup retained an unverified Environment")
        current = rows().get(environment)
        if current is None:
            return
        if current != original:
            raise RuntimeError("exec fixture cleanup retained a changed Environment")
        invoke("rm", "-f", environment)
    except Exception:
        if primary is None:
            raise RuntimeError("exec fixture Environment cleanup failed or ownership was unknown") from None
        primary.add_note("exec fixture Environment cleanup also failed or ownership was unknown")


def read_identity(process, deadline):
    """Read one bounded receipt without an unbounded blocking readline."""
    receipt = bytearray()
    while not receipt.endswith(b"\n"):
        ready, _, _ = select.select([process.stdout], [], [], remaining(deadline))
        if not ready:
            raise RuntimeError("exec identity readiness timed out")
        part = os.read(process.stdout.fileno(), 1)
        if not part:
            raise RuntimeError("exec exited before its identity receipt")
        receipt.extend(part)
        if len(receipt) > 96:
            raise RuntimeError("exec identity receipt exceeded its bound")
    match = re.fullmatch(rb"EXEC-IDENTITY ([1-9][0-9]{0,9}) ([0-9]{1,20}) ([0-9]{1,20})\n", receipt)
    if match is None or int(match[1]) <= 1:
        raise RuntimeError("exec identity receipt was invalid")
    return tuple(field.decode("ascii") for field in match.groups())


def require_running(invoke, environment, original, timeout):
    status = json.loads(invoke("env", "status", environment, "--json", timeout=timeout))
    if status.get("environment") != original or status.get("state") != "running":
        raise RuntimeError("exec cancellation changed the running Environment identity")


def verify_cancellation(product, environment, original, invoke, rows):
    if not original or not original.get("runtime_ref") or not original.get("created_at"):
        raise RuntimeError("exec cancellation Environment identity was unavailable")
    if rows().get(environment) != original:
        raise RuntimeError("exec cancellation Environment identity changed before launch")
    require_running(invoke, environment, original, 300)
    process = subprocess.Popen(
        [product, "exec", environment, "--", "sh", "-ec", START_COMMAND],
        stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, bufsize=0,
    )
    try:
        identity = read_identity(process, time.monotonic() + 300)
        deadline = time.monotonic() + 30  # Existing cancellation budget, shared by all observations.
        process.send_signal(signal.SIGINT)
        process.wait(timeout=remaining(deadline))
        if process.returncode != 130:
            raise RuntimeError("exec cancellation did not return exit 130")
        if rows(timeout=remaining(deadline)).get(environment) != original:
            raise RuntimeError("exec cancellation changed the retained Environment")
        last = "unobserved"
        while True:
            if time.monotonic() >= deadline:
                raise RuntimeError("exec cancellation target remained: " + last)
            # Ordinary execution also requires the controller to release the first
            # exec's lifecycle lock. No provider privilege or auto-start is added.
            result = invoke("exec", environment, "--", "sh", "-ec", OBSERVE_COMMAND,
                            "sh", *identity, timeout=remaining(deadline), limit=32)
            if result in ("absent\n", "reused\n"):
                break
            if result == "environment_changed\n":
                raise RuntimeError("exec cancellation guest incarnation changed")
            if result not in ("present\n", "zombie\n"):
                raise RuntimeError("exec cancellation target absence was unknown")
            last = result.strip()
            time.sleep(min(0.1, remaining(deadline)))
        require_running(invoke, environment, original, remaining(deadline))
        remaining(deadline)
    except (subprocess.TimeoutExpired, OSError):
        # Exception strings may include argv or captured guest output.
        raise RuntimeError("exec cancellation command or observation failed") from None
    finally:
        cleanup_process(process, sys.exc_info()[1])
    print("ENVIRONMENT EXEC CANCEL / EXACT EXECUTION ABSENT / RUNNING ENV RETAINED: PASS")

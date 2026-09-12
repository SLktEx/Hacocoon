#!/usr/bin/env python3
"""Exercise the shipped BAT final wait in a real Windows pseudoconsole."""
import os
import queue
import re
import sys
import threading
import time
from winpty import PtyProcess

environment = {k: v for k, v in os.environ.items() if k.upper() not in {"CI", "HACO_INSTALL_NO_PAUSE"}}
process = PtyProcess.spawn([os.path.join(os.environ["SystemRoot"], "System32", "cmd.exe"), "/d"],
                          cwd=sys.argv[1], env=environment)
chunks = queue.Queue()
closing = threading.Event()

def reader():
    try:
        while True:
            chunks.put(process.read(4096))
    except (EOFError, OSError):
        if not closing.is_set():
            chunks.put(None)

reader_thread = threading.Thread(target=reader, daemon=True)
reader_thread.start()
output = ""

def until(predicate, timeout):
    global output
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            chunk = chunks.get(timeout=min(0.1, max(0.001, deadline - time.monotonic())))
        except queue.Empty:
            continue
        if chunk is None:
            raise AssertionError("terminal exited before completing the BAT wait")
        chunk = re.sub(r"\x1b\][^\x07]*?(?:\x07|\x1b\\)", "", chunk)
        chunk = re.sub(r"\x1b\[[0-?]*[ -/]*[@-~]", "", chunk)
        output += chunk
        if predicate(output):
            return True
    return False

try:
    process.write("install-windows.bat\r\n")
    assert until(lambda s: "Press any key to close this installer window." in s, 20), "missing final wait"
    # A queued second command would run immediately if the BAT returned early.
    # Observe a quiet interval, then explicitly provide one key.
    assert not until(lambda s: re.search(r"[A-Za-z]:\\[^\r\n>]*>\s*$", s) is not None, 0.5), "BAT returned before a key"
    process.write(" ")
    assert until(lambda s: re.search(r"[A-Za-z]:\\[^\r\n>]*>\s*$", s) is not None, 10), "key did not release BAT"
    process.write("echo HACO_PAUSE_RESULT=%ERRORLEVEL%\r\n")
    assert until(lambda s: re.search(r"HACO_PAUSE_RESULT=37(?:\r|\n)", s) is not None, 10), "BAT lost exit 37"
    print("WINDOWS BAT CONPTY WAIT / KEYPRESS / EXIT 37: PASS")
finally:
    closing.set()
    process.close(force=True)
    reader_thread.join(timeout=2)

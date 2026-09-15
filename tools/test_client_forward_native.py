"""Installed CLI/controller forwarding on the caller's test-owned Environment.

Creates only bounded application processes. The parent owns Environment cleanup.
No Incus proxy, network exception, package installation or service changes.
"""
import argparse
import concurrent.futures
import os
import re
import selectors
import signal
import socket
import subprocess
import time

_SERVER = r'''
import select,socket,sys,threading
s=socket.socket();s.bind(("127.0.0.1",0));s.listen(16)
print(s.getsockname()[1],flush=True)
# This is application-fixture control, not a product command or permission.
# Host entry and native listener observation must not consume accept's budget.
if not select.select([sys.stdin],[],[],STARTUP_SECONDS)[0] or sys.stdin.buffer.read(1)!=b'G':
    raise SystemExit('application fixture was not armed')
s.settimeout(ACCEPT_SECONDS)
def handle(c):
    with c:
        c.settimeout(10)
        chunks=[]
        while True:
            b=c.recv(65536)
            if not b: break
            chunks.append(b)
        c.sendall(b"".join(chunks))
workers=[]
for i in range(8):
    c,_=s.accept();t=threading.Thread(target=handle,args=(c,));t.start();workers.append(t)
for t in workers: t.join()
s.close()
'''


def server_source(startup_seconds=180, accept_seconds=40):
    # Small budgets let the regression exercise delayed preparation faithfully.
    # Production acceptance always uses the constants above, without overrides.
    if not 0 < startup_seconds <= 180 or not 0 < accept_seconds <= 40:
        raise ValueError("invalid application fixture deadline")
    return f"STARTUP_SECONDS={float(startup_seconds)!r}\nACCEPT_SECONDS={float(accept_seconds)!r}\n" + _SERVER


SERVER = server_source()


def arm_application(process):
    process.stdin.write(b'G')
    process.stdin.flush()
    process.stdin.close()

def line(process, timeout=20):
    with selectors.DefaultSelector() as selector:
        selector.register(process.stdout, selectors.EVENT_READ)
        if not selector.select(timeout):
            raise RuntimeError("application readiness timed out")
        return process.stdout.readline(4096).decode("utf-8", errors="strict").strip()

def stop(process):
    if process.poll() is None:
        os.killpg(process.pid, signal.SIGTERM)
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait(timeout=5)

def main():
    p = argparse.ArgumentParser()
    p.add_argument("--haco", required=True)
    p.add_argument("--env", required=True)
    p.add_argument("--ref", required=True)
    p.add_argument("--project", required=True)
    a = p.parse_args()
    for value in (a.env, a.ref, a.project):
        if not re.fullmatch(r"[a-z0-9][a-z0-9-]{0,63}", value):
            raise ValueError("invalid fixture identity")
    processes = []
    try:
        server = subprocess.Popen(["incus", "exec", a.ref, "--project", a.project,
                                   "--", "python3", "-c", SERVER], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                  start_new_session=True)
        processes.append(server)
        target = line(server)
        if not target.isdecimal() or not 1 <= int(target) <= 65535:
            raise RuntimeError("invalid application readiness")
        tunnel = subprocess.Popen([a.haco, "env", "tunnel", "--target-port", target,
                                   "--duration", "45s", a.env], stdout=subprocess.PIPE,
                                  env=dict(os.environ, HACO_UI_LANGUAGE="en"), start_new_session=True)
        processes.append(tunnel)
        ready = line(tunnel)
        match = re.match(r"Listening at 127\.0\.0\.1:(\d+) → ", ready)
        if not match:
            raise RuntimeError("CLI listener readiness missing")
        port = int(match.group(1))
        arm_application(server)
        data = bytes(range(256)) * 8192
        def exchange(_):
            with socket.create_connection(("127.0.0.1", port), timeout=15) as s:
                s.sendall(data); s.shutdown(socket.SHUT_WR)
                answer = bytearray()
                while True:
                    part = s.recv(65536)
                    if not part: break
                    answer.extend(part)
                if answer != data:
                    raise RuntimeError("forwarded binary bytes differ")
        with concurrent.futures.ThreadPoolExecutor(max_workers=8) as executor:
            list(executor.map(exchange, range(8)))
        if server.wait(timeout=10) != 0:
            raise RuntimeError("application fixture failed")
        stop(tunnel)
        with socket.socket() as s:
            s.settimeout(1)
            if s.connect_ex(("127.0.0.1", port)) == 0:
                raise RuntimeError("listener remained after cancellation")
        print("client stream forwarding: PASS installed controller, 8 concurrent 2MiB round trips, half-close, cancellation and listener cleanup")
    finally:
        for process in reversed(processes):
            if process.stdin is not None and not process.stdin.closed:
                process.stdin.close()
            stop(process)

if __name__ == "__main__":
    main()

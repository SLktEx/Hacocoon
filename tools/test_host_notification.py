#!/usr/bin/env python3
"""Installed trusted Host acceptance: real controller subscription without audit mounts."""
import json
import os
from pathlib import Path
import socket
import stat
import sys
import subprocess
import time
import urllib.error
import urllib.request

binary = Path('/usr/local/bin/haco-notify')
info = binary.lstat()
assert stat.S_ISREG(info.st_mode) and info.st_uid == 0 and info.st_gid == 0
assert stat.S_IMODE(info.st_mode) == 0o755
assert os.environ.get('HACO_CLIENT_MODE') == 'controller'
assert len(sys.argv) in (1, 2)
if len(sys.argv) == 2:
    assert os.environ.get('WSL_DISTRO_NAME') == sys.argv[1], 'wrong native notification distribution'
    subprocess.run(['systemctl','is-enabled','--quiet','hacocoon-notify.service'], check=True)
    subprocess.run(['systemctl','is-active','--quiet','hacocoon-notify.service'], check=True)
assert not Path('/var/lib/hacocoon/audit/capabilities.jsonl').exists()
with socket.socket() as reservation:
    reservation.bind(('127.0.0.1', 0))
    port = reservation.getsockname()[1]
process = subprocess.Popen([str(binary), 'web', '--listen', '127.0.0.1:' + str(port)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
try:
    url = 'http://127.0.0.1:' + str(port) + '/api/v1/events?limit=1'
    deadline = time.monotonic() + 10
    while True:
        assert process.poll() is None, 'notification process exited'
        try:
            opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
            with opener.open(url, timeout=2) as response:
                result = json.load(response)
            break
        except urllib.error.HTTPError:
            raise
        except urllib.error.URLError:
            if time.monotonic() >= deadline: raise
            time.sleep(0.1)
    assert result['schema_version'] == 1 and len(result['events']) <= 1
    assert not result.get('error') and result['next_offset'] >= 0
    allowed = {'schema_version','event_id','request_id','time','kind','environment','capability','action','code','requires_attention','recovery_required','next_offset'}
    for event in result['events']:
        assert set(event) <= allowed, 'non-public event fields'
    print('PASS installed Host notification controller subscription without audit projection')
finally:
    if process.poll() is None:
        process.terminate()
        try: process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)
with socket.socket() as probe:
    probe.settimeout(1)
    assert probe.connect_ex(('127.0.0.1', port)) != 0, 'notification listener survived cleanup'
print('PASS owned notification listener cleanup')

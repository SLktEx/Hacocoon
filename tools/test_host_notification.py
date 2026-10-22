#!/usr/bin/env python3
"""Installed trusted Host acceptance: real controller subscription without audit mounts."""
import json
import os
import re
from pathlib import Path
import socket
import stat
import sys
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

SERVICE = 'hacocoon-notify.service'
SERVICE_PROPERTIES = ('ActiveState', 'SubState', 'Result', 'ExecMainStatus', 'NRestarts')
# Fixed systemd v259 tables, not arbitrary unit/journal data:
# https://github.com/systemd/systemd/blob/v259/src/basic/unit-def.c
# https://github.com/systemd/systemd/blob/v259/src/core/service.c
SERVICE_ENUMS = {
    'ActiveState': frozenset(('active', 'reloading', 'inactive', 'failed', 'activating',
                              'deactivating', 'maintenance', 'refreshing')),
    'SubState': frozenset(('dead', 'condition', 'start-pre', 'start', 'start-post', 'running',
                           'exited', 'refresh-extensions', 'reload', 'reload-signal', 'reload-notify',
                           'reload-post', 'stop', 'stop-watchdog', 'stop-sigterm', 'stop-sigkill',
                           'stop-post', 'final-watchdog', 'final-sigterm', 'final-sigkill', 'failed',
                           'dead-before-auto-restart', 'failed-before-auto-restart', 'dead-resources-pinned',
                           'auto-restart', 'auto-restart-queued', 'cleaning', 'mounting')),
    'Result': frozenset(('success', 'resources', 'protocol', 'timeout', 'exit-code', 'signal',
                         'core-dump', 'watchdog', 'start-limit-hit', 'oom-kill', 'exec-condition')),
}


def service_properties(raw):
    fields = {}
    for line in raw.decode('utf-8').splitlines():
        key, separator, value = line.partition('=')
        if not separator or key not in SERVICE_PROPERTIES or key in fields:
            raise ValueError('invalid service observation')
        if key in SERVICE_ENUMS:
            if value not in SERVICE_ENUMS[key]:
                raise ValueError('unknown service state')
            fields[key] = value
        else:
            if not re.fullmatch(r'-?[0-9]{1,11}', value):
                raise ValueError('invalid service counter')
            number = int(value)
            minimum, maximum = (0, 2**32 - 1) if key == 'NRestarts' else (-2**31, 2**31 - 1)
            if not minimum <= number <= maximum:
                raise ValueError('invalid service counter')
            fields[key] = number
    if set(fields) != set(SERVICE_PROPERTIES):
        raise ValueError('incomplete service observation')
    return fields


def notification_service_diagnostic(exit_code, run=subprocess.run):
    diagnostic = {'component': 'ci', 'operation': 'host-notification-service-state',
                  'original_exit_code': exit_code if type(exit_code) is int and -2**31 <= exit_code < 2**31 else None}
    args = ['systemctl', 'show', '--no-pager', '--property=' + ','.join(SERVICE_PROPERTIES), SERVICE]
    try:
        # Capture only selected properties to an anonymous file; never buffer
        # unbounded stdout in memory or retain provider stderr/journal bodies.
        with tempfile.TemporaryFile() as output:
            result = run(args, stdin=subprocess.DEVNULL, stdout=output, stderr=subprocess.DEVNULL,
                         timeout=3, check=False)
            if result.returncode != 0:
                return dict(diagnostic, observation='query_failed')
            output.seek(0)
            raw = output.read(4097)
        if len(raw) > 4096:
            return dict(diagnostic, observation='oversized')
        return dict(diagnostic, observation='observed', properties=service_properties(raw))
    except subprocess.TimeoutExpired:
        return dict(diagnostic, observation='timeout')
    except OSError:
        return dict(diagnostic, observation='unavailable')
    except (UnicodeError, ValueError):
        return dict(diagnostic, observation='invalid')


def require_notification_service_active(run=subprocess.run):
    try:
        run(['systemctl', 'is-active', '--quiet', SERVICE], check=True)
    except subprocess.CalledProcessError as failure:
        try:
            print(json.dumps(notification_service_diagnostic(failure.returncode, run), sort_keys=True), file=sys.stderr)
        except (OSError, ValueError):
            pass  # A broken diagnostic stream must not replace the original failure.
        raise  # Preserve the original one-shot failure, including after a later active observation.


def main():
    binary = Path('/usr/local/bin/haco-notify')
    info = binary.lstat()
    assert stat.S_ISREG(info.st_mode) and info.st_uid == 0 and info.st_gid == 0
    assert stat.S_IMODE(info.st_mode) == 0o755
    assert os.environ.get('HACO_CLIENT_MODE') == 'controller'
    assert len(sys.argv) in (1, 2)
    if len(sys.argv) == 2:
        assert os.environ.get('WSL_DISTRO_NAME') == sys.argv[1], 'wrong native notification distribution'
        subprocess.run(['systemctl','is-enabled','--quiet','hacocoon-notify.service'], check=True)
        require_notification_service_active()
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


if __name__ == '__main__':
    main()

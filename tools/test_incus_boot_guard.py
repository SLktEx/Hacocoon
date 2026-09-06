#!/usr/bin/env python3
"""Provider startup regressions; never operate on installed Incus state."""
from concurrent.futures import ThreadPoolExecutor
import importlib.util
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('guard', ROOT / 'modules/runtime/incus/packaging/incus-boot-guard.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)
BOOT = '8e951216-a1e4-4397-818a-4710984907ad'
OLD = {'boot': BOOT, 'init_start': 100, 'pidns': 111}
NEW = {'boot': BOOT, 'init_start': 200, 'pidns': 222}


class BootGuardTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.fd = os.open(self.root, os.O_RDONLY | os.O_DIRECTORY)
        self.addCleanup(os.close, self.fd)
        self.state = self.root / '.hacocoon-boot-guard'

    def run_guard(self, identity=OLD, initialize=False, available=False):
        guard.Guard(self.root, identity, uid=os.getuid(), available=lambda _: available).run(initialize, root_fd=self.fd)

    def record(self, network='haco-host0', content=None):
        directory = self.root / 'networks' / network
        directory.mkdir(parents=True, exist_ok=True)
        path = directory / 'dnsmasq.pid'
        path.write_text(content or f'name: dnsmasq\npid: {os.getpid()}\n')
        path.chmod(0o600)
        return path

    def marker(self):
        return json.loads((self.state / 'namespace.json').read_text())['namespace']

    def test_wsl_namespace_restart_retires_reused_pid_without_signalling(self):
        self.run_guard()
        path = self.record()  # Numeric PID now names the running test process.
        original = path.read_bytes()
        config = path.parent / 'dnsmasq.raw'
        config.write_text('preserve network configuration')
        with patch.object(os, 'kill', side_effect=AssertionError('must never signal')):
            self.run_guard(NEW)
        self.assertFalse(path.exists())
        self.assertEqual(config.read_text(), 'preserve network configuration')
        archives = list(self.state.glob('retired-*/networks/haco-host0/dnsmasq.pid'))
        self.assertEqual([p.read_bytes() for p in archives], [original])
        self.assertEqual(self.marker(), NEW)

    def test_same_namespace_restart_retains_current_records(self):
        self.run_guard()
        path = self.record()
        inode = path.stat().st_ino
        self.run_guard(available=True)
        self.assertEqual(path.stat().st_ino, inode)
        self.assertEqual(list(self.state.glob('retired-*')), [])

    def test_native_reboot_with_reused_namespace_numbers(self):
        self.run_guard()
        path = self.record()
        changed = dict(OLD, boot='b312cf19-3e4c-48aa-8e90-d15c64a3853f')
        self.run_guard(changed)
        self.assertFalse(path.exists())

    def test_active_daemon_blocks_retirement(self):
        self.run_guard()
        path = self.record()
        with self.assertRaises(guard.Refused):
            self.run_guard(NEW, available=True)
        self.assertTrue(path.exists())
        self.assertEqual(self.marker(), OLD)

    def test_unknown_records_require_live_daemon_adoption(self):
        path = self.record()
        with self.assertRaises(guard.Refused):
            self.run_guard()
        with self.assertRaises(guard.Refused):
            self.run_guard(initialize=True)
        self.run_guard(initialize=True, available=True)
        self.assertTrue(path.exists())
        self.assertEqual(self.marker(), OLD)
        with self.assertRaises(guard.Refused):
            self.run_guard(NEW, initialize=True, available=True)

    def test_symlink_pidfile_is_not_followed(self):
        self.run_guard()
        path = self.record()
        target = self.root / 'important'
        target.write_text('preserve')
        path.unlink()
        path.symlink_to(target)
        with self.assertRaises(OSError):
            self.run_guard(NEW)
        self.assertEqual(target.read_text(), 'preserve')
        self.assertEqual(self.marker(), OLD)

    def test_symlink_network_directory_is_rejected(self):
        self.run_guard()
        target = self.root / 'outside-network'
        target.mkdir()
        (self.root / 'networks').mkdir()
        (self.root / 'networks' / 'haco-host0').symlink_to(target)
        with self.assertRaises(OSError):
            self.run_guard(NEW)

    def test_hardlink_and_writable_record_fail_before_any_moves(self):
        self.run_guard()
        first = self.record('a')
        second = self.record('z')
        os.link(second, self.root / 'alias')
        with self.assertRaises(guard.Refused):
            self.run_guard(NEW)
        self.assertTrue(first.exists())
        (self.root / 'alias').unlink()
        second.chmod(0o666)
        with self.assertRaises(guard.Refused):
            self.run_guard(NEW)
        self.assertTrue(first.exists())

    def test_interrupted_retirement_resumes_without_losing_records(self):
        self.run_guard()
        self.record('a', 'first')
        self.record('b', 'second')
        rename = os.rename
        count = 0
        def interrupted(*args, **kwargs):
            nonlocal count
            count += 1
            if count == 2:
                raise OSError('injected interruption')
            return rename(*args, **kwargs)
        with patch.object(os, 'rename', interrupted):
            with self.assertRaises(OSError):
                self.run_guard(NEW)
        self.assertEqual(self.marker(), OLD)
        self.run_guard(NEW)
        saved = [p.read_text() for p in self.state.glob('retired-*/networks/*/dnsmasq.pid')]
        self.assertCountEqual(saved, ['first', 'second'])
        self.assertEqual(self.marker(), NEW)

    def test_concurrent_starts_are_serialized(self):
        self.run_guard()
        self.record()
        with ThreadPoolExecutor(max_workers=2) as pool:
            list(pool.map(lambda _: self.run_guard(NEW), range(2)))
        self.assertEqual(len(list(self.state.glob('retired-*/networks/haco-host0/dnsmasq.pid'))), 1)

    def test_proxy_pid_is_retired_without_touching_devices(self):
        self.run_guard()
        directory = self.root / 'devices' / 'hacocoon_haco-host'
        directory.mkdir(parents=True)
        pid = directory / 'proxy.haco-control'
        pid.write_text(f'pid: {os.getpid()}\n')
        disk = directory / 'disk.haco--wsl--init.init'
        disk.write_text('retained device data')
        self.run_guard(NEW)
        self.assertFalse(pid.exists())
        self.assertEqual(disk.read_text(), 'retained device data')
        self.assertEqual(len(list(self.state.glob('retired-*/devices/hacocoon_haco-host/proxy.haco-control'))), 1)

    def test_corrupt_marker_cannot_authorize_retirement(self):
        self.run_guard()
        path = self.record()
        (self.state / 'namespace.json').write_text('{"version":1,"namespace":{}}')
        with self.assertRaises(guard.Refused):
            self.run_guard(NEW)
        self.assertTrue(path.exists())

    def test_fifo_is_refused_without_blocking(self):
        self.run_guard()
        path = self.record()
        path.unlink()
        os.mkfifo(path, 0o600)
        with self.assertRaises(guard.Refused):
            self.run_guard(NEW)

    def test_linked_state_directory_is_not_followed(self):
        target = self.root / 'other'
        target.mkdir()
        self.state.symlink_to(target)
        with self.assertRaises(OSError):
            self.run_guard()
        self.assertEqual(list(target.iterdir()), [])

    def test_namespace_identity_is_complete(self):
        identity = guard.namespace_identity()
        self.assertEqual(set(identity), {'boot', 'init_start', 'pidns'})
        self.assertGreater(identity['pidns'], 0)


class DaemonDetectionTests(unittest.TestCase):
    def detect(self, executable, main_pid='248', namespace=111, uid=0):
        def readlink(path):
            return '/usr/lib/systemd/systemd' if path == '/proc/1/exe' else executable

        def info(path):
            return SimpleNamespace(st_ino=111 if path == '/proc/self/ns/pid' else namespace, st_uid=uid)

        with patch.object(os, 'listdir', return_value=['self', '1', '248']), \
                patch.object(os, 'readlink', side_effect=readlink), \
                patch.object(os, 'stat', side_effect=info), \
                patch.object(guard.subprocess, 'run', return_value=SimpleNamespace(stdout=main_pid)) as run:
            result = guard.daemon_available(Path('/var/lib/incus'))
            if result:
                self.assertEqual(run.call_args.args[0], ['/usr/bin/systemctl', 'show', 'incus.service', '--property=MainPID', '--value'])
            return result

    def test_idle_systemd_activation_socket_is_not_a_running_daemon(self):
        self.assertFalse(self.detect('/usr/bin/python3', main_pid='0'))

    def test_managed_daemon_and_replaced_executable_are_detected(self):
        for suffix in ('', ' (deleted)'):
            self.assertTrue(self.detect('/usr/libexec/incus/incusd' + suffix))

    def test_unmanaged_helpers_or_wrong_owner_refuse_retirement(self):
        for kwargs in ({'main_pid': '0'}, {'uid': 1000}):
            with self.assertRaises(guard.Refused):
                self.detect('/usr/libexec/incus/incusd', **kwargs)

    def test_daemon_in_another_pid_namespace_does_not_authorize_adoption(self):
        self.assertFalse(self.detect('/usr/libexec/incus/incusd', namespace=222))


if __name__ == '__main__':
    unittest.main()

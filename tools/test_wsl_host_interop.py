import importlib.util
import os
import hashlib
from pathlib import Path
import unittest
import shutil
import subprocess
import tempfile
from unittest import mock
import types
import stat

spec = importlib.util.spec_from_file_location("interop", Path(__file__).resolve().parents[1] / "scripts/setup-wsl-host-interop.py")
interop = importlib.util.module_from_spec(spec)
spec.loader.exec_module(interop)


class InteropTests(unittest.TestCase):
    def test_only_real_drive_mounts_and_multiple_drives(self):
        mounts = [{"target": "/mnt/" + name, "fstype": kind, "options": options} for name, kind, options in [
            ("c", "9p", "rw,aname=drvfs;path=C:"), ("d", "drvfs", "rw"),
            ("e", "ext4", "rw"), ("fake", "drvfs", "rw"), ("z/other", "drvfs", "rw")]]
        self.assertEqual(interop.drive_mounts(mounts), ["/mnt/c", "/mnt/d"])

    def test_owner_profiles_and_device_collision_fail_closed(self):
        devices = interop.desired_devices(["/mnt/c", "/mnt/e"])
        config = {"config": {"user.hacocoon.role": "trusted-host"}, "profiles": [], "devices": {}}
        self.assertEqual(set(interop.plan(config, devices)), set(devices))
        config["devices"] = devices.copy()
        self.assertEqual(interop.plan(config, devices), [])
        for changed in [dict(config, profiles=["default"]), dict(config, config={}),
                        dict(config, devices={"foreign": {"path": "/mnt/c"}}),
                        dict(config, devices={"haco-wsl-init": {"type": "disk", "source": "/evil"}})]:
            with self.assertRaises(ValueError):
                interop.plan(changed, devices)



class WindowsPathTests(unittest.TestCase):
    def test_only_existing_drive_entries_no_linux_or_traversal(self):
        value = '/usr/local/bin:/mnt/c/Windows/System32:/mnt/q/Tools With Spaces:/mnt/e/absent:/mnt/q/../secret:/mnt/q/bin\nBAD:/mnt/q/Tools With Spaces'
        self.assertEqual(interop.windows_paths(value, ['/mnt/c', '/mnt/q']),
                         ['/mnt/c/Windows/System32', '/mnt/q/Tools With Spaces'])

    def test_native_socket_path_restored_after_run_recreation(self):
        # /run tmpfs hides an Incus disk mounted directly there on reboot.
        # Exercise the standard tmpfiles rule against a fresh filesystem root.
        device = interop.desired_devices(['/mnt/q'])['haco-wsl-interop']
        self.assertEqual(device['source'], '/run/WSL')
        self.assertFalse(device['path'].startswith('/run/'))
        self.assertEqual(device['readonly'], 'true')
        if not shutil.which('systemd-tmpfiles'):
            self.skipTest('systemd-tmpfiles required for native layout regression')
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            (root / 'etc/tmpfiles.d').mkdir(parents=True)
            (root / 'run').mkdir()
            rules = [line for line in interop.SOCKET_LAYOUT.splitlines() if line.startswith("printf ")]
            self.assertEqual(len(rules), 1)
            rule = rules[0].split("'", 4)[3]
            config = root / 'etc/tmpfiles.d/hacocoon-wsl.conf'
            config.write_text(rule + '\n')
            subprocess.run(['systemd-tmpfiles', '--root=' + str(root), '--create', str(config)], check=True)
            self.assertEqual((root / 'run/WSL').readlink(), Path(device['path']))
            # A fresh /run after reboot gets the same native address again.
            (root / 'run/WSL').unlink()
            subprocess.run(['systemd-tmpfiles', '--root=' + str(root), '--create', str(config)], check=True)
            self.assertEqual((root / 'run/WSL').readlink(), Path(device['path']))

    def test_no_fixed_drive_letter_list(self):
        mounts = [{'target': '/mnt/q', 'fstype': '9p', 'options': 'rw,aname=drvfs;path=Q:'}]
        self.assertEqual(interop.drive_mounts(mounts), ['/mnt/q'])
        devices = interop.desired_devices(interop.drive_mounts(mounts))
        self.assertEqual(devices['haco-wsl-drive-q']['path'], '/mnt/q')
        self.assertNotIn('haco-wsl-drive-c', devices)

class DistributionTests(unittest.TestCase):
    def test_validated_record_and_collision(self):
        with tempfile.TemporaryDirectory() as directory:
            record = Path(directory) / 'distribution.json'
            original = Path.stat
            def owned(path, *args, **kwargs):
                info = original(path, *args, **kwargs)
                return types.SimpleNamespace(st_mode=info.st_mode, st_uid=0, st_size=info.st_size)
            with mock.patch.object(interop, 'DISTRIBUTION_RECORD', record), mock.patch.object(Path, 'stat', owned), mock.patch.dict(interop.os.environ, {'WSL_DISTRO_NAME':'Hacocoon-Test'}):
                self.assertEqual(interop.distribution_record(True), 'Hacocoon-Test')
                self.assertEqual(interop.distribution_record(), 'Hacocoon-Test')
                with mock.patch.dict(interop.os.environ, {'WSL_DISTRO_NAME':'Other'}):
                    with self.assertRaises(ValueError): interop.distribution_record(True)
                self.assertEqual(interop.distribution_record(), 'Hacocoon-Test')
                record.write_text('"bad;command"')
                with self.assertRaises(ValueError): interop.distribution_record()
                record.unlink()
                record.symlink_to(Path(directory) / 'missing')
                with self.assertRaises(ValueError): interop.distribution_record(True)

    def test_foreign_host_distribution_is_rejected(self):
        config = {'profiles':[], 'config':{'user.hacocoon.role':'trusted-host', 'environment.WSL_DISTRO_NAME':'Other'}}
        with self.assertRaises(ValueError): interop.plan(config, {}, 'Hacocoon')


class NotificationServiceTests(unittest.TestCase):
    @unittest.skipUnless(hasattr(os, 'O_NOFOLLOW'), 'Linux executable ownership contract')
    def test_executable_revision_changes_and_refuses_unsafe_files(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'notify'
            path.write_bytes(b'first')
            path.chmod(0o700)
            original = os.fstat
            def owned(fd):
                info = original(fd)
                return types.SimpleNamespace(st_mode=info.st_mode, st_uid=0, st_nlink=info.st_nlink, st_size=info.st_size, st_mtime_ns=info.st_mtime_ns)
            with mock.patch.object(os, 'fstat', owned):
                first = interop.notification_executable_revision(path)
                self.assertEqual(first, hashlib.sha256(b'first').hexdigest())
                path.write_bytes(b'second')
                self.assertNotEqual(first, interop.notification_executable_revision(path))
                link = Path(directory) / 'link'
                link.symlink_to(path)
                with self.assertRaises(OSError): interop.notification_executable_revision(link)
                path.chmod(0o722)
                with self.assertRaises(ValueError): interop.notification_executable_revision(path)
                path.chmod(0o700)
                os.link(path, Path(directory) / 'hardlink')
                with self.assertRaises(ValueError): interop.notification_executable_revision(path)

    def test_unit_environment_is_quoted_and_parser_accepts_it(self):
        unit = interop.notification_unit(['/mnt/c/Tools With Spaces', '/mnt/c/Percent%And"Quote'], 'Hacocoon-Test', 'a' * 64)
        self.assertIn('HACO_CONTROL_SOCKET=/var/lib/hacocoon-control.sock', unit)
        self.assertIn('HOME=/root', unit)
        self.assertIn('UMask=0077', unit)
        self.assertIn('%%', unit)
        self.assertNotIn('/var/lib/hacocoon/audit', unit)
        with self.assertRaises(ValueError): interop.notification_unit(['/mnt/c/bad\nvalue'], 'Hacocoon', 'a' * 64)
        with self.assertRaises(ValueError): interop.notification_unit([], 'Hacocoon;command', 'a' * 64)
        if not shutil.which('systemd-analyze'):
            self.skipTest('systemd-analyze required for unit parser acceptance')
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'hacocoon-notify.service'
            # Verify the real unit syntax with an available harmless executable.
            path.write_text(unit.replace('/usr/local/bin/haco-notify', '/bin/true'))
            subprocess.run(['systemd-analyze', 'verify', str(path)], check=True, capture_output=True)

    def test_owned_service_enable_disable_and_foreign_refusal(self):
        with tempfile.TemporaryDirectory() as directory:
            directory = Path(directory)
            target = directory / 'hacocoon-notify.service'
            def fresh_systemd(args, **kwargs):
                if args[1] == 'reset-failed':
                    raise subprocess.CalledProcessError(1, args, stderr='Unit not loaded')
                return subprocess.CompletedProcess(args, 1 if args[1] == 'is-failed' else 0)
            run = mock.Mock(side_effect=fresh_systemd)
            unit = interop.notification_unit(['/mnt/c/Windows/System32'], 'Hacocoon', 'a' * 64)
            original = Path.lstat
            def owned(path, *args, **kwargs):
                info = original(path, *args, **kwargs)
                return types.SimpleNamespace(st_mode=info.st_mode, st_uid=0, st_nlink=info.st_nlink, st_size=info.st_size)
            with mock.patch.object(Path, 'lstat', owned):
                interop.install_notification_unit(unit, True, directory, run)
                self.assertEqual(target.read_text(), unit)
                self.assertEqual([call.args[0][1] for call in run.call_args_list], ['daemon-reload','is-failed','enable','restart','is-active'])
                run.reset_mock()
                run.side_effect = None
                run.return_value = subprocess.CompletedProcess([], 0)
                interop.install_notification_unit(unit, None, directory, run)
                self.assertEqual(run.call_args_list[0].args[0], ['systemctl','is-enabled','--quiet','hacocoon-notify.service'])
                self.assertEqual([call.args[0][1] for call in run.call_args_list], ['is-enabled','is-active','enable'])
                run.reset_mock()
                run.side_effect = lambda args, **kwargs: subprocess.CompletedProcess(args, 3 if args[1] == 'is-active' and not kwargs.get('check') else 0)
                interop.install_notification_unit(unit, None, directory, run)
                self.assertIn(mock.call(['systemctl','restart','hacocoon-notify.service'], check=True), run.call_args_list)
                self.assertIn(mock.call(['systemctl','reset-failed','hacocoon-notify.service'], check=True), run.call_args_list)
                run.side_effect = None
                run.reset_mock()
                changed = unit.replace('a' * 64, 'b' * 64)
                interop.install_notification_unit(changed, True, directory, run)
                self.assertIn(mock.call(['systemctl','restart','hacocoon-notify.service'], check=True), run.call_args_list)
                run.reset_mock()
                interop.install_notification_unit(unit, False, directory, run)
                run.assert_called_once_with(['systemctl','disable','--now','hacocoon-notify.service'], check=True)
                run.reset_mock()
                run.return_value = subprocess.CompletedProcess([], 1)
                interop.install_notification_unit(unit, None, directory, run)
                run.assert_called_once_with(['systemctl','is-enabled','--quiet','hacocoon-notify.service'], check=False)
                run.reset_mock()
                target.write_text('[Service]\nExecStart=/bin/false\n')
                with self.assertRaises(ValueError): interop.install_notification_unit(unit, True, directory, run)
                run.assert_not_called()
                self.assertIn('/bin/false', target.read_text())
                target.unlink()
                target.symlink_to('/dev/null')
                with self.assertRaises(ValueError): interop.install_notification_unit(unit, True, directory, run)
                run.assert_not_called()


class NativeBinfmtTests(unittest.TestCase):
    handler = 'enabled\ninterpreter /init\nflags: P\noffset 0\nmagic 4d5a\n'

    def test_healthy_native_handler_is_not_recreated(self):
        with tempfile.TemporaryDirectory() as name:
            root = Path(name)
            (root / 'status').write_text('enabled\n')
            (root / 'WSLInterop').write_text(self.handler)
            run = mock.Mock()
            interop.ensure_native_binfmt(root, root / 'missing-generator', run)
            run.assert_not_called()
            (root / 'WSLInterop').write_text(self.handler.replace('/init', '/foreign'))
            with self.assertRaises(ValueError): interop.ensure_native_binfmt(root, root / 'missing', run)
            run.assert_not_called()

    def test_missing_handler_uses_only_wsl_native_service(self):
        with tempfile.TemporaryDirectory() as name:
            root = Path(name)
            (root / 'status').write_text('enabled\n')
            generated = root / 'override.conf'
            generated.write_text('ExecStart=/bin/sh -c native :WSLInterop:M::MZ::/init:P')
            calls = []
            def run(args, **kwargs):
                calls.append((args, kwargs))
                (root / 'WSLInterop').write_text(self.handler)
            # The fixture belongs to the test user; model the required native
            # root-owned regular generator metadata without changing real units.
            metadata = types.SimpleNamespace(st_mode=stat.S_IFREG | 0o644, st_uid=0)
            original = Path.lstat
            def lstat(path, *args, **kwargs):
                return metadata if path == generated else original(path, *args, **kwargs)
            with mock.patch.object(Path, 'lstat', lstat):
                interop.ensure_native_binfmt(root, generated, run)
            self.assertEqual(calls, [(['systemctl','restart','systemd-binfmt.service'], {'check': True})])
            (root / 'status').write_text('disabled\n')
            with self.assertRaises(ValueError): interop.ensure_native_binfmt(root, generated, run)
            self.assertEqual(len(calls), 1)


if __name__ == '__main__':
    unittest.main()

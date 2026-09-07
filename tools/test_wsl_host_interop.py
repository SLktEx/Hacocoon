import importlib.util
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

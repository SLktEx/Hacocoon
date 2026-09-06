import importlib.util
from pathlib import Path
import unittest

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

    def test_native_absolute_socket_symlink_keeps_its_mount_path(self):
        # Real fresh WSL uses 1_interop -> /run/WSL/<pid>_interop. Mounting
        # this directory at another guest path makes native connect fail ENOENT.
        device = interop.desired_devices(['/mnt/q'])['haco-wsl-interop']
        self.assertEqual(device['source'], '/run/WSL')
        self.assertEqual(device['path'], device['source'])
        self.assertEqual(device['readonly'], 'true')
        config = {'config': {'user.hacocoon.role': 'trusted-host',
                            'environment.WSL_INTEROP': '/run/WSL/1_interop'},
                  'profiles': [], 'devices': interop.desired_devices(['/mnt/q'])}
        self.assertEqual(interop.plan(config, config['devices']), [])
    def test_no_fixed_drive_letter_list(self):
        mounts = [{'target': '/mnt/q', 'fstype': '9p', 'options': 'rw,aname=drvfs;path=Q:'}]
        self.assertEqual(interop.drive_mounts(mounts), ['/mnt/q'])
        devices = interop.desired_devices(interop.drive_mounts(mounts))
        self.assertEqual(devices['haco-wsl-drive-q']['path'], '/mnt/q')
        self.assertNotIn('haco-wsl-drive-c', devices)

if __name__ == '__main__':
    unittest.main()

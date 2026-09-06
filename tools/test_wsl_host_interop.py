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


if __name__ == "__main__":
    unittest.main()

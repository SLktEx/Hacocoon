import copy
import json
import unittest
from cleanup_ci_base_asset import cleanup


class CleanupTest(unittest.TestCase):
    def fixture(self):
        owner = "a" * 32
        name = "haco-base-" + owner
        asset = dict(id="base-" + owner, owner=owner, provider="runtime.incus", scope="hacocoon/haco-local-default",
                     native_ref="instance/" + name, state="ready", base=dict(name="fixture/base", revision="sha256:" + "b" * 64),
                     binding=json.dumps(dict(version=1, project="hacocoon", pool="haco-local-default", source="local:" + "b" * 64)))
        config = {"user.hacocoon.kind": "base", "user.hacocoon.owner": owner, "user.hacocoon.base-name": "fixture/base",
                  "user.hacocoon.base-revision": asset["base"]["revision"], "boot.autostart": "false", "security.privileged": "false",
                  "security.nesting": "false", "volatile.base_image": "b" * 64}
        devices = {"root": dict(type="disk", path="/", pool="haco-local-default")}
        instance = dict(name=name, type="container", status="Stopped", profiles=[], ephemeral=False,
                        config=config, expanded_config=copy.deepcopy(config), devices=devices, expanded_devices=copy.deepcopy(devices))
        return dict(version=7, base_assets={asset["id"]: asset}), name, instance

    def test_exact_cleanup_and_refusal(self):
        for mode in ("ok", "absent", "lease", "foreign-owner", "host-device", "running", "duplicate", "still-present", "failed-delete"):
            with self.subTest(mode=mode):
                data, name, instance = self.fixture()
                if mode == "lease": data["workspace_leases"] = {"dev": {}}
                if mode == "foreign-owner": instance["config"]["user.hacocoon.owner"] = "c" * 32
                if mode == "host-device": instance["devices"]["host"] = dict(type="disk", source="/", path="/host")
                if mode == "running": instance["status"] = "Running"
                calls = []
                def run(*args):
                    calls.append(args)
                    if args[0] == "delete":
                        self.assertEqual(args, ("delete", name, "--project", "hacocoon"))
                        if mode == "failed-delete": raise RuntimeError("failed")
                        return ""
                    rows = [] if mode == "absent" or (len(calls) > 1 and mode != "still-present") else [instance]
                    if mode == "duplicate": rows += rows
                    return json.dumps(rows)
                if mode in ("ok", "absent"): cleanup(data, name, run)
                else:
                    with self.assertRaises(RuntimeError): cleanup(data, name, run)
                deletes = sum(c[0] == "delete" for c in calls)
                self.assertEqual(deletes, int(mode in ("ok", "still-present", "failed-delete")))


if __name__ == "__main__": unittest.main()

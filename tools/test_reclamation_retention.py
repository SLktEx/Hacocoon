import json
import unittest
from unittest.mock import Mock
import reclamation_retention as subject

RECORD = {"version": 1, "nonce": "1" * 16, "workspace": "import-" + "2" * 16,
          "oci": "oci:import-" + "3" * 16, "snapshot": "snap-" + "4" * 32, "commit": "5" * 40}
SAVED = {"id": RECORD["snapshot"], "state": "ready", "oci": True, "workspaces": 1}

class RetentionTests(unittest.TestCase):
    def host(self, *args):
        self.calls.append(args)
        if args[:2] == ("snapshot", "list"):
            return json.dumps([SAVED])
        if args[:2] == ("snapshot", "restore"):
            return json.dumps({"environment": args[-1], "state": "running", "workspace": "independent", "oci": "independent-oci"})
        return ""

    def setUp(self):
        self.calls = []

    def test_reattachment_and_saved_data_are_separate_and_never_deleted(self):
        guest = Mock()
        subject.verify(RECORD, self.host, guest)
        self.assertEqual(guest.call_count, 2)
        self.assertIn(RECORD["commit"], guest.call_args_list[0].args[1])
        self.assertIn('rootfs-kept', guest.call_args_list[0].args[1])
        self.assertIn('test ! -e /root/transfer-marker', guest.call_args_list[1].args[1])
        self.assertTrue(any(args[:2] == ("env", "create") and RECORD["oci"] in args for args in self.calls))
        self.assertFalse(any("delete" in args for args in self.calls))
        self.assertEqual(sum(args[:2] == ("snapshot", "restore") for args in self.calls), 1)

    def test_untrusted_manifest_refused_before_commands(self):
        for key, value in [("nonce", "../foreign"), ("oci", "oci:existing"), ("commit", "-option"), ("version", True)]:
            record = {**RECORD, key: value}
            with self.assertRaises(RuntimeError):
                subject.verify(record, self.host, Mock())
        self.assertEqual(self.calls, [])

    def test_missing_snapshot_prevents_restore_or_reattachment(self):
        host = Mock(return_value="[]")
        with self.assertRaises(RuntimeError):
            subject.verify(RECORD, host, Mock())
        self.assertEqual(host.call_count, 1)
        self.assertEqual(host.call_args.args, ("snapshot", "list", "--json"))

    def test_failed_content_check_retains_resources_without_retry(self):
        guest = Mock(side_effect=RuntimeError("content mismatch"))
        with self.assertRaises(RuntimeError):
            subject.verify(RECORD, self.host, guest)
        self.assertEqual(guest.call_count, 1)
        self.assertEqual(len(self.calls), 2)
        self.assertFalse(any("delete" in args for args in self.calls))

    def test_restore_cannot_reuse_current_data(self):
        def host(*args):
            if args[:2] == ("snapshot", "restore"):
                return json.dumps({"environment": args[-1], "state": "running", "workspace": RECORD["workspace"], "oci": RECORD["oci"]})
            return self.host(*args)
        guest = Mock()
        with self.assertRaises(RuntimeError):
            subject.verify(RECORD, host, guest)
        guest.assert_not_called()

if __name__ == "__main__":
    unittest.main()

"""Regressions for native source-guard observations; no provider mutations."""
import copy
import unittest
from unittest.mock import patch
import verify_installed_source_guard as guard

# Representative nft JSON from a real Incus/WSL table, with fixture identities.
TABLE = "haco_guard_0000000001"
BRIDGE = "hbr000000000001"
MAC = "02:00:00:00:00:01"
GENERATION = "env-" + "1" * 32
FIXTURE = {"nftables": [
    {"table": {"family": "inet", "name": TABLE}},
    {"chain": {"family": "inet", "table": TABLE, "name": "prerouting",
               "type": "filter", "hook": "prerouting", "prio": -300, "policy": "accept"}},
    {"rule": {"family": "inet", "table": TABLE, "chain": "prerouting", "expr": [
        {"match": {"op": "==", "left": {"meta": {"key": "iifname"}}, "right": BRIDGE}},
        {"match": {"op": "!=", "left": {"payload": {"protocol": "ether", "field": "saddr"}}, "right": MAC}},
        {"drop": None}]}},
    {"rule": {"family": "inet", "table": TABLE, "chain": "prerouting", "expr": [
        {"match": {"op": "==", "left": {"meta": {"key": "iifname"}}, "right": BRIDGE}},
        {"match": {"op": "==", "left": {"payload": {"protocol": "ip", "field": "saddr"}}, "right": "0.0.0.0"}},
        {"match": {"op": "==", "left": {"payload": {"protocol": "udp", "field": "sport"}}, "right": 68}},
        {"match": {"op": "==", "left": {"payload": {"protocol": "udp", "field": "dport"}}, "right": 67}},
        {"accept": None}]}},
    {"rule": {"family": "inet", "table": TABLE, "chain": "prerouting", "expr": [
        {"match": {"op": "==", "left": {"meta": {"key": "iifname"}}, "right": BRIDGE}},
        {"match": {"op": "!=", "left": {"payload": {"protocol": "ip", "field": "saddr"}},
                   "right": {"prefix": {"addr": "10.97.100.0", "len": 24}}}},
        {"drop": None}]}},
]}


class GuardObservationTest(unittest.TestCase):
    def verify(self, document):
        guard.verify_rules(document, TABLE, BRIDGE, MAC, "10.97.100.0/24")

    def test_native_shape(self):
        self.verify(FIXTURE)

    def test_rejects_bypasses_wrong_identity_and_hook(self):
        for case in ("missing", "reorder", "accept", "wide_dhcp", "mac", "subnet", "hook", "extra_chain"):
            with self.subTest(case=case):
                data = copy.deepcopy(FIXTURE)
                rows = data["nftables"]
                if case == "missing": rows.pop()
                if case == "reorder": rows[2], rows[3] = rows[3], rows[2]
                if case == "accept": rows[2]["rule"]["expr"][-1] = {"accept": None}
                if case == "wide_dhcp": rows[3]["rule"]["expr"].pop(1)
                if case == "mac": rows[2]["rule"]["expr"][1]["match"]["right"] = "02:00:00:00:00:ff"
                if case == "subnet": rows[4]["rule"]["expr"][1]["match"]["right"]["prefix"]["len"] = 8
                if case == "hook": rows[1]["chain"]["prio"] = 0
                if case == "extra_chain": rows.append(copy.deepcopy(rows[1]))
                with self.assertRaises(ValueError): self.verify(data)

    def test_rejects_untrusted_arguments_before_query(self):
        with patch.object(guard, "query") as query:
            for instance, generation in [("--help", GENERATION), ("haco-good?project=default", GENERATION), ("haco-good", "old")]:
                with self.assertRaises(ValueError): guard.verify(instance, generation)
            query.assert_not_called()

    def test_rejects_same_name_different_generation(self):
        with patch.object(guard, "query", return_value={"config": {"user.hacocoon.instance-id": "env-" + "2" * 32}, "status": "Running"}) as query:
            with self.assertRaises(ValueError): guard.verify("haco-good", GENERATION)
            self.assertEqual(query.call_count, 1)


if __name__ == "__main__":
    unittest.main()

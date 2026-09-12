#!/usr/bin/env python3
"""Read-only native source-guard acceptance for one already-created test Env."""
import argparse
import hashlib
import ipaddress
import json
import re
import subprocess


class ObservationError(ValueError):
    """Controlled diagnostic; never contains native response content."""


def query(argv):
    result = subprocess.run(argv, capture_output=True, timeout=30, check=True)
    if len(result.stdout) > 1024 * 1024:
        raise ObservationError("oversized native response")
    return json.loads(result.stdout)


def match(left, op, right):
    return {"match": {"left": left, "op": op, "right": right}}


def verify_rules(document, table, bridge, mac, subnet):
    entries = document["nftables"]
    tables = [x["table"] for x in entries if "table" in x]
    chains = [x["chain"] for x in entries if "chain" in x]
    rules = [x["rule"] for x in entries if "rule" in x]
    if any(len(x) != 1 or next(iter(x)) not in ("metainfo", "table", "chain", "rule") for x in entries):
        raise ObservationError("unexpected guard entry")
    if len(tables) != 1 or tables[0]["name"] != table or tables[0]["family"] != "inet":
        raise ObservationError("wrong guard table")
    if len(chains) != 1:
        raise ObservationError("unexpected guard chains")
    chain = chains[0]
    expected = dict(family="inet", table=table, name="prerouting", type="filter",
                    hook="prerouting", prio=-300, policy="accept")
    if any(chain.get(k) != v for k, v in expected.items()):
        raise ObservationError("wrong guard hook")
    interface = match({"meta": {"key": "iifname"}}, "==", bridge)
    payload = lambda protocol, field: {"payload": {"protocol": protocol, "field": field}}
    prefix = ipaddress.IPv4Network(subnet)
    wanted = [
        [interface, match(payload("ether", "saddr"), "!=", mac), {"drop": None}],
        [interface, match(payload("ip", "saddr"), "==", "0.0.0.0"),
         match(payload("udp", "sport"), "==", 68), match(payload("udp", "dport"), "==", 67), {"accept": None}],
        [interface, match(payload("ip", "saddr"), "!=", {"prefix": {
            "addr": str(prefix.network_address), "len": prefix.prefixlen}}), {"drop": None}],
    ]
    if len(rules) != 3:
        raise ObservationError("unexpected guard rule count")
    for rule, expressions in zip(rules, wanted):
        if (rule.get("family"), rule.get("table"), rule.get("chain")) != ("inet", table, "prerouting") or rule.get("expr") != expressions:
            raise ObservationError("guard rule order or meaning differs")


def verify(instance, generation):
    if not re.fullmatch(r"haco-[a-z0-9][a-z0-9-]{0,100}", instance):
        raise ObservationError("invalid fixture instance")
    if not re.fullmatch(r"env-[a-f0-9]{32}", generation):
        raise ObservationError("invalid fixture generation")
    get_instance = lambda: query(["incus", "query", "/1.0/instances/" + instance + "?project=hacocoon"])
    before = get_instance()
    if before["config"].get("user.hacocoon.instance-id") != generation:
        raise ObservationError("fixture generation differs")
    if before["status"] != "Running":
        raise ObservationError("fixture is not running")
    nic = before["expanded_devices"]["eth0"]
    bridge, mac = nic["network"], nic["hwaddr"]
    if nic.get("type") != "nic" or nic.get("security.port_isolation") != "true" or not re.fullmatch(r"hbr[a-f0-9]{12}", bridge) or not re.fullmatch(r"02(?::[a-f0-9]{2}){5}", mac):
        raise ObservationError("unexpected isolated NIC")
    network = query(["incus", "query", "/1.0/networks/" + bridge + "?project=default"])
    config = network["config"]
    if config.get("user.hacocoon.owner") != "environment-network-v1" or config.get("ipv4.nat") != "false" or config.get("ipv6.address") != "none":
        raise ObservationError("unexpected managed bridge")
    subnet = str(ipaddress.IPv4Interface(config["ipv4.address"]).network)
    table = "haco_guard_" + hashlib.sha256(instance.encode()).digest()[:8].hex()[-10:]
    verify_rules(query(["nft", "-j", "list", "table", "inet", table]), table, bridge, mac, subnet)
    after = get_instance()
    if after["config"].get("user.hacocoon.instance-id") != generation or after["status"] != "Running" or after["expanded_devices"]["eth0"] != nic:
        raise ObservationError("fixture changed during observation")
    return {"status": "PASS", "generation": generation, "guard_rules": "verified", "spoofed_packets_tested": False}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("instance")
    parser.add_argument("generation")
    args = parser.parse_args()
    try:
        print(json.dumps(verify(args.instance, args.generation)))
    except (KeyError, TypeError, ValueError, subprocess.SubprocessError) as error:
        detail = str(error) if isinstance(error, ObservationError) else type(error).__name__
        raise SystemExit("source guard observation failed: " + detail)

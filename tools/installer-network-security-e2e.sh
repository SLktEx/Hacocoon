#!/usr/bin/env bash
set -euo pipefail

haco_bin="${HACO_BIN:-haco}"
project="${HACO_E2E_INCUS_PROJECT:-hacocoon}"
network_project="${HACO_E2E_NETWORK_PROJECT:-default}"
workspace="${HACO_E2E_NETWORK_WORKSPACE:-$(mktemp -d)}"
suffix="${RANDOM}-$$"
env_a="network-a-$suffix"
env_b="network-b-$suffix"
ref_a="haco-$env_a"
ref_b="haco-$env_b"
created_a=0
created_b=0
docker_compat_ifaces=()

fail() {
  printf 'installer network security: %s\n' "$*" >&2
  exit 1
}

cleanup_docker_forwarding() {
  local iface
  command -v iptables >/dev/null 2>&1 || return 0
  iptables -w 5 -nL DOCKER-USER >/dev/null 2>&1 || return 0
  for iface in "${docker_compat_ifaces[@]}"; do
    while iptables -w 5 -C DOCKER-USER -i "$iface" -j ACCEPT >/dev/null 2>&1; do
      iptables -w 5 -D DOCKER-USER -i "$iface" -j ACCEPT >/dev/null 2>&1 || break
    done
    while iptables -w 5 -C DOCKER-USER -o "$iface" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT >/dev/null 2>&1; do
      iptables -w 5 -D DOCKER-USER -o "$iface" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT >/dev/null 2>&1 || break
    done
  done
}

cleanup() {
  set +e
  if [[ "$created_a" == "1" ]]; then
    "$haco_bin" env delete "$env_a" >/dev/null 2>&1 || true
  fi
  if [[ "$created_b" == "1" ]]; then
    "$haco_bin" env delete "$env_b" >/dev/null 2>&1 || true
  fi
  cleanup_docker_forwarding
  rm -rf "$workspace"
}
trap cleanup EXIT

[[ "$(id -u)" == "0" ]] || fail "must run as root so Incus/nft state can be inspected without widening installed user privileges"
command -v "$haco_bin" >/dev/null 2>&1 || fail "haco is unavailable"
command -v incus >/dev/null 2>&1 || fail "incus is unavailable"
command -v nft >/dev/null 2>&1 || fail "nft is unavailable"
command -v ping >/dev/null 2>&1 || fail "ping is unavailable for Physical Host reachability acceptance"
command -v python3 >/dev/null 2>&1 || fail "python3 is unavailable for Incus runtime-state inspection"
mkdir -p "$workspace/a" "$workspace/b"
printf 'network-security\n' > "$workspace/probe.txt"

find_guard_table() {
  local bridge="$1"
  local mac="$2"
  local family table name raw
  while read -r family table name; do
    [[ "$family" == "table" && "$table" == "inet" && "$name" == haco_guard_* ]] || continue
    raw="$(nft list table inet "$name" 2>/dev/null || true)"
    if printf '%s\n' "$raw" | grep -Fq "iifname \"$bridge\" ether saddr != $mac drop"; then
      printf '%s\n' "$name"
      return 0
    fi
  done < <(nft list tables)
  return 1
}

assert_guard() {
  local bridge="$1"
  local mac="$2"
  local table raw accepts
  table="$(find_guard_table "$bridge" "$mac")" || fail "no anti-spoofing guard found for bridge $bridge / MAC $mac"
  raw="$(nft list table inet "$table")"
  printf '%s\n' "$raw" | grep -Fq "iifname \"$bridge\" ether saddr != $mac drop" || fail "MAC spoofing guard is missing for $bridge"
  printf '%s\n' "$raw" | grep -Fq "iifname \"$bridge\" ip saddr 0.0.0.0 udp sport 68 udp dport 67 accept" || fail "narrow DHCP bootstrap exception is missing for $bridge"
  printf '%s\n' "$raw" | grep -F "iifname \"$bridge\" ip saddr != " | grep -Fq ' drop' || fail "IPv4 source-subnet guard is missing for $bridge"
  accepts="$(printf '%s\n' "$raw" | grep -F "iifname \"$bridge\"" | grep -E '(^|[[:space:]])accept$' | sed 's/^[[:space:]]*//; s/[[:space:]]*$//' || true)"
  if [[ "$accepts" != "iifname \"$bridge\" ip saddr 0.0.0.0 udp sport 68 udp dport 67 accept" ]]; then
    fail "guard $table has unexpected accept rules for $bridge: ${accepts:-<none>}"
  fi
  printf '%s\n' "$table"
}

docker_forward_policy() {
  iptables -w 5 -S FORWARD 2>/dev/null | awk '$1 == "-P" && $2 == "FORWARD" {print $3; exit}'
}

configure_docker_forwarding() {
  local iface="$1"
  local policy

  # GitHub-hosted Ubuntu starts Docker before the packaged installer runs.
  # Docker can therefore own the legacy IPv4 FORWARD chain with policy DROP,
  # which also catches bridged DHCP when br_netfilter is active. Incus and
  # Hacocoon use separate nftables chains, and an accept there cannot override
  # a later Docker drop. Add scoped coexistence rules for both the managed
  # bridge and the Incus host-side veth so this E2E can identify which interface
  # the bridged packet actually exposes to Docker's legacy FORWARD chain.
  command -v iptables >/dev/null 2>&1 || return 0
  policy="$(docker_forward_policy || true)"
  [[ "$policy" == "DROP" ]] || return 0
  iptables -w 5 -nL DOCKER-USER >/dev/null 2>&1 || fail "Docker FORWARD policy is DROP but DOCKER-USER is unavailable"

  if ! iptables -w 5 -C DOCKER-USER -i "$iface" -j ACCEPT >/dev/null 2>&1; then
    iptables -w 5 -I DOCKER-USER 1 -i "$iface" -j ACCEPT
  fi
  if ! iptables -w 5 -C DOCKER-USER -o "$iface" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT >/dev/null 2>&1; then
    iptables -w 5 -I DOCKER-USER 1 -o "$iface" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
  fi
  docker_compat_ifaces+=("$iface")
}

environment_ipv4() {
  local ref="$1"
  local bridge="$2"
  local address=""
  local state_json=""
  local attempt

  # Incus runtime state is the same authoritative source used by the egress
  # identity resolver. Avoid depending on the guest image having ip/awk/cut
  # installed, and allow enough time for systemd-networkd + DHCP on a fresh
  # managed bridge to settle on slower WSL/GitHub-hosted runners.
  for attempt in $(seq 1 60); do
    state_json="$(incus list "$ref" --project "$project" --format json 2>/dev/null || true)"
    address="$(printf '%s' "$state_json" | python3 -c '
import json, sys
try:
    rows = json.load(sys.stdin)
except Exception:
    rows = []
for row in rows:
    network = ((row.get("state") or {}).get("network") or {})
    ordered = []
    if "eth0" in network:
        ordered.append(network["eth0"])
    ordered.extend(v for k, v in network.items() if k != "eth0")
    for iface in ordered:
        for item in (iface.get("addresses") or []):
            address = item.get("address", "")
            if item.get("family") == "inet" and address and not address.startswith("127.") and not address.startswith("169.254."):
                print(address)
                raise SystemExit(0)
' 2>/dev/null || true)"
    address="$(printf '%s' "$address" | head -n1 | tr -d '[:space:]')"
    if [[ "$address" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      printf '%s\n' "$address"
      return 0
    fi
    sleep 0.5
  done

  printf 'installer network security: IPv4 diagnostics for %s on %s\n' "$ref" "$bridge" >&2
  if command -v iptables >/dev/null 2>&1; then
    printf '%s\n' '--- live Docker FORWARD / DOCKER-USER counters ---' >&2
    iptables -w 5 -v -nL FORWARD >&2 || true
    iptables -w 5 -v -nL DOCKER-USER >&2 || true
  fi
  incus list "$ref" --project "$project" --format json >&2 || true
  incus exec "$ref" --project "$project" -- sh -c 'command -v ip >/dev/null 2>&1 && ip -4 -o addr show || true' >&2 || true
  incus network show "$bridge" --project "$network_project" >&2 || true
  incus network list-leases "$bridge" --project "$network_project" >&2 || true
  return 1
}

printf '==> Network security: create two isolated Environments\n'
"$haco_bin" env create --read-only --workspace "$workspace/a" "$env_a" >/dev/null
created_a=1
"$haco_bin" env create --read-only --workspace "$workspace/b" "$env_b" >/dev/null
created_b=1

bridge_a="$(incus config device get "$ref_a" eth0 network --project "$project")"
bridge_b="$(incus config device get "$ref_b" eth0 network --project "$project")"
mac_a="$(incus config device get "$ref_a" eth0 hwaddr --project "$project" | tr '[:upper:]' '[:lower:]')"
mac_b="$(incus config device get "$ref_b" eth0 hwaddr --project "$project" | tr '[:upper:]' '[:lower:]')"
host_veth_a="$(incus config get "$ref_a" volatile.eth0.host_name --project "$project" | tr -d '[:space:]')"
host_veth_b="$(incus config get "$ref_b" volatile.eth0.host_name --project "$project" | tr -d '[:space:]')"

[[ "$bridge_a" == hbr* && "$bridge_b" == hbr* ]] || fail "Environment NICs are not bound to Hacocoon dedicated bridges: $bridge_a / $bridge_b"
[[ "$bridge_a" != "$bridge_b" ]] || fail "two Environments unexpectedly share bridge $bridge_a"
[[ "$mac_a" == 02:* && "$mac_b" == 02:* ]] || fail "Environment MAC identities are not managed local-unicast addresses: $mac_a / $mac_b"
[[ "$mac_a" != "$mac_b" ]] || fail "two Environments unexpectedly share managed MAC $mac_a"
[[ -n "$host_veth_a" && -n "$host_veth_b" ]] || fail "Incus did not expose host-side Environment veth names"
[[ "$host_veth_a" != "$host_veth_b" ]] || fail "two Environments unexpectedly share host veth $host_veth_a"

for bridge in "$bridge_a" "$bridge_b"; do
  incus network show "$bridge" --project "$network_project" >/dev/null || fail "dedicated bridge $bridge is missing"
  [[ "$(incus network get "$bridge" ipv4.nat --project "$network_project")" == "false" ]] || fail "$bridge unexpectedly enables NAT"
  [[ "$(incus network get "$bridge" ipv4.firewall --project "$network_project")" == "true" ]] || fail "$bridge does not retain Incus DHCP/checksum plumbing"
  [[ "$(incus network get "$bridge" ipv4.dhcp --project "$network_project")" == "true" ]] || fail "$bridge does not provide managed DHCP"
  [[ "$(incus network get "$bridge" ipv4.routing --project "$network_project")" == "true" ]] || fail "$bridge does not provide the managed host route"
  [[ "$(incus network get "$bridge" ipv6.address --project "$network_project")" == "none" ]] || fail "$bridge unexpectedly enables IPv6"
  [[ "$(incus network get "$bridge" raw.dnsmasq --project "$network_project")" == "port=0" ]] || fail "$bridge DNS listener is not disabled"
done

printf '==> Network security: verify host and per-Environment fail-closed policy\n'
host_firewall="$(nft list table inet hacocoon_sandbox)"
printf '%s\n' "$host_firewall" | grep -Fq 'iifname "hbr*" ct state established,related accept' || fail "host firewall does not allow replies to Physical Host initiated traffic"
printf '%s\n' "$host_firewall" | grep -Fq 'iifname "hbr*" udp sport 68 udp dport 67 accept' || fail "host firewall lacks narrow DHCP input exception"
printf '%s\n' "$host_firewall" | grep -Fq 'iifname "hbr*" ip daddr 169.254.254.1 tcp dport 18080 accept' || fail "host firewall lacks Standard egress proxy exception"
printf '%s\n' "$host_firewall" | grep -Fq 'iifname "hbr*" ip daddr 255.255.255.255 udp sport 68 udp dport 67 accept' || fail "host firewall lacks narrow DHCP forward bootstrap"
printf '%s\n' "$host_firewall" | grep -Fq 'oifname "hbr*" udp sport 67 udp dport 68 accept' || fail "host firewall lacks narrow DHCP forward reply"
printf '%s\n' "$host_firewall" | grep -Fq 'iifname "hbr*" drop' || fail "host firewall does not reject other Environment input/forwarding"
printf '%s\n' "$host_firewall" | grep -Fq 'oifname "hbr*" drop' || fail "host firewall does not reject other forwarding into Environment bridges"

guard_a="$(assert_guard "$bridge_a" "$mac_a")"
guard_b="$(assert_guard "$bridge_b" "$mac_b")"
[[ "$guard_a" != "$guard_b" ]] || fail "two Environments unexpectedly share anti-spoofing table $guard_a"

configure_docker_forwarding "$bridge_a"
configure_docker_forwarding "$bridge_b"
configure_docker_forwarding "$host_veth_a"
configure_docker_forwarding "$host_veth_b"
if command -v iptables >/dev/null 2>&1 && [[ "$(docker_forward_policy || true)" == "DROP" ]]; then
  printf '==> Network security: Docker coexistence interfaces %s/%s via %s/%s\n' "$bridge_a" "$bridge_b" "$host_veth_a" "$host_veth_b"
  iptables -w 5 -v -nL DOCKER-USER || true
fi

printf '==> Network security: Physical Host can initiate traffic to each Environment\n'
ip_a="$(environment_ipv4 "$ref_a" "$bridge_a")" || fail "unable to resolve IPv4 address for $env_a"
ip_b="$(environment_ipv4 "$ref_b" "$bridge_b")" || fail "unable to resolve IPv4 address for $env_b"
ping -4 -n -c 1 -W 3 "$ip_a" >/dev/null || fail "Physical Host cannot reach $env_a at $ip_a"
ping -4 -n -c 1 -W 3 "$ip_b" >/dev/null || fail "Physical Host cannot reach $env_b at $ip_b"

printf '==> Network security: deletion removes only that Environment network authority\n'
"$haco_bin" env delete "$env_a"
created_a=0
if incus network show "$bridge_a" --project "$network_project" >/dev/null 2>&1; then
  fail "deleted Environment bridge $bridge_a still exists"
fi
if nft list table inet "$guard_a" >/dev/null 2>&1; then
  fail "deleted Environment anti-spoofing guard $guard_a still exists"
fi
incus network show "$bridge_b" --project "$network_project" >/dev/null || fail "deleting $env_a damaged $env_b bridge"
nft list table inet "$guard_b" >/dev/null || fail "deleting $env_a damaged $env_b anti-spoofing guard"

"$haco_bin" env delete "$env_b"
created_b=0
if incus network show "$bridge_b" --project "$network_project" >/dev/null 2>&1; then
  fail "deleted Environment bridge $bridge_b still exists"
fi
if nft list table inet "$guard_b" >/dev/null 2>&1; then
  fail "deleted Environment anti-spoofing guard $guard_b still exists"
fi

cleanup_docker_forwarding
docker_compat_ifaces=()

printf 'installer network security: PASS\n'

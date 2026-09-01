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

fail() {
  printf 'installer network security: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  set +e
  if [[ "$created_a" == "1" ]]; then
    "$haco_bin" env delete "$env_a" >/dev/null 2>&1 || true
  fi
  if [[ "$created_b" == "1" ]]; then
    "$haco_bin" env delete "$env_b" >/dev/null 2>&1 || true
  fi
  rm -rf "$workspace"
}
trap cleanup EXIT

[[ "$(id -u)" == "0" ]] || fail "must run as root so Incus/nft state can be inspected without widening installed user privileges"
command -v "$haco_bin" >/dev/null 2>&1 || fail "haco is unavailable"
command -v incus >/dev/null 2>&1 || fail "incus is unavailable"
command -v nft >/dev/null 2>&1 || fail "nft is unavailable"
command -v ping >/dev/null 2>&1 || fail "ping is unavailable for Physical Host reachability acceptance"
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

environment_ipv4() {
  local ref="$1"
  local address=""
  local attempt
  for attempt in $(seq 1 20); do
    address="$(incus exec "$ref" --project "$project" -- sh -c 'ip -4 -o addr show dev eth0 | awk "{print \$4}" | cut -d/ -f1 | head -n1' 2>/dev/null || true)"
    address="$(printf '%s' "$address" | tr -d '[:space:]')"
    if [[ "$address" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      printf '%s\n' "$address"
      return 0
    fi
    sleep 0.25
  done
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

[[ "$bridge_a" == hbr* && "$bridge_b" == hbr* ]] || fail "Environment NICs are not bound to Hacocoon dedicated bridges: $bridge_a / $bridge_b"
[[ "$bridge_a" != "$bridge_b" ]] || fail "two Environments unexpectedly share bridge $bridge_a"
[[ "$mac_a" == 02:* && "$mac_b" == 02:* ]] || fail "Environment MAC identities are not managed local-unicast addresses: $mac_a / $mac_b"
[[ "$mac_a" != "$mac_b" ]] || fail "two Environments unexpectedly share managed MAC $mac_a"

for bridge in "$bridge_a" "$bridge_b"; do
  incus network show "$bridge" --project "$network_project" >/dev/null || fail "dedicated bridge $bridge is missing"
  [[ "$(incus network get "$bridge" ipv4.nat --project "$network_project")" == "false" ]] || fail "$bridge unexpectedly enables NAT"
  [[ "$(incus network get "$bridge" ipv4.firewall --project "$network_project")" == "false" ]] || fail "$bridge unexpectedly delegates spoofing policy to Incus bridge firewall"
  [[ "$(incus network get "$bridge" ipv4.routing --project "$network_project")" == "true" ]] || fail "$bridge does not provide the managed host route"
  [[ "$(incus network get "$bridge" ipv6.address --project "$network_project")" == "none" ]] || fail "$bridge unexpectedly enables IPv6"
  [[ "$(incus network get "$bridge" raw.dnsmasq --project "$network_project")" == "port=0" ]] || fail "$bridge DNS listener is not disabled"
done

printf '==> Network security: verify host and per-Environment fail-closed policy\n'
host_firewall="$(nft list table inet hacocoon_sandbox)"
printf '%s\n' "$host_firewall" | grep -Fq 'iifname "hbr*" ct state established,related accept' || fail "host firewall does not allow replies to Physical Host initiated traffic"
printf '%s\n' "$host_firewall" | grep -Fq 'iifname "hbr*" udp sport 68 udp dport 67 accept' || fail "host firewall lacks narrow DHCP input exception"
printf '%s\n' "$host_firewall" | grep -Fq 'iifname "hbr*" ip daddr 169.254.254.1 tcp dport 18080 accept' || fail "host firewall lacks Standard egress proxy exception"
printf '%s\n' "$host_firewall" | grep -Fq 'iifname "hbr*" drop' || fail "host firewall does not reject other Environment input"
printf '%s\n' "$host_firewall" | grep -Fq 'oifname "hbr*" drop' || fail "host firewall does not reject forwarding into Environment bridges"

guard_a="$(assert_guard "$bridge_a" "$mac_a")"
guard_b="$(assert_guard "$bridge_b" "$mac_b")"
[[ "$guard_a" != "$guard_b" ]] || fail "two Environments unexpectedly share anti-spoofing table $guard_a"

printf '==> Network security: Physical Host can initiate traffic to each Environment\n'
ip_a="$(environment_ipv4 "$ref_a")" || fail "unable to resolve IPv4 address for $env_a"
ip_b="$(environment_ipv4 "$ref_b")" || fail "unable to resolve IPv4 address for $env_b"
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

printf 'installer network security: PASS\n'

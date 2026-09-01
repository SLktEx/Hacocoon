#!/usr/bin/env bash
set -euo pipefail

haco_bin="${HACO_BIN:-haco}"
project="${HACO_E2E_INCUS_PROJECT:-hacocoon}"
workspace="${HACO_E2E_NETWORK_WORKSPACE:-$(mktemp -d)}"
suffix="${RANDOM}-$$"
env_a="network-a-$suffix"
env_b="network-b-$suffix"
ref_a="haco-$env_a"
ref_b="haco-$env_b"
created_a=0
created_b=0
proxy_ipv4="169.254.254.1"
proxy_port="18080"

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
mkdir -p "$workspace/a" "$workspace/b"
printf 'network-security\n' > "$workspace/probe.txt"

find_guard_table() {
  local iface="$1"
  local address="$2"
  local family table name raw
  while read -r family table name; do
    [[ "$family" == "table" && "$table" == "inet" && "$name" == haco_guard_* ]] || continue
    raw="$(nft list table inet "$name" 2>/dev/null || true)"
    if printf '%s\n' "$raw" | grep -Fq "iifname \"$iface\" ip saddr != $address drop"; then
      printf '%s\n' "$name"
      return 0
    fi
  done < <(nft list tables)
  return 1
}

assert_guard() {
  local iface="$1"
  local address="$2"
  local table raw rules
  table="$(find_guard_table "$iface" "$address")" || fail "no exact IPv4 anti-spoofing guard found for $iface / $address"
  raw="$(nft list table inet "$table")"
  printf '%s\n' "$raw" | grep -Eq 'type filter hook prerouting priority (raw|-300); policy accept;' || fail "guard $table does not run at raw/prerouting priority"
  printf '%s\n' "$raw" | grep -Fq "iifname \"$iface\" ip saddr != $address drop" || fail "exact source identity guard is missing for $iface"
  rules="$(printf '%s\n' "$raw" | grep -F "iifname \"$iface\"" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//' || true)"
  [[ "$rules" == "iifname \"$iface\" ip saddr != $address drop" ]] || fail "guard $table contains unexpected interface rules: ${rules:-<none>}"
  printf '%s\n' "$table"
}

assert_managed_ipv4() {
  local address="$1"
  [[ "$address" =~ ^198\.(18|19)\.[0-9]{1,3}\.[0-9]{1,3}$ ]] || fail "routed Environment IPv4 is outside 198.18.0.0/15: $address"
}

printf '==> Network security: create two routed Environments\n'
"$haco_bin" env create --read-only --workspace "$workspace/a" "$env_a" >/dev/null
created_a=1
"$haco_bin" env create --read-only --workspace "$workspace/b" "$env_b" >/dev/null
created_b=1

nictype_a="$(incus config device get "$ref_a" eth0 nictype --project "$project")"
nictype_b="$(incus config device get "$ref_b" eth0 nictype --project "$project")"
iface_a="$(incus config device get "$ref_a" eth0 host_name --project "$project")"
iface_b="$(incus config device get "$ref_b" eth0 host_name --project "$project")"
ip_a="$(incus config device get "$ref_a" eth0 ipv4.address --project "$project" | tr -d '[:space:]')"
ip_b="$(incus config device get "$ref_b" eth0 ipv4.address --project "$project" | tr -d '[:space:]')"
host_a="$(incus config device get "$ref_a" eth0 ipv4.host_address --project "$project" | tr -d '[:space:]')"
host_b="$(incus config device get "$ref_b" eth0 ipv4.host_address --project "$project" | tr -d '[:space:]')"

[[ "$nictype_a" == "routed" && "$nictype_b" == "routed" ]] || fail "Environment NICs are not routed: $nictype_a / $nictype_b"
[[ "$iface_a" == haco* && "$iface_b" == haco* ]] || fail "Environment host veths are not Hacocoon-managed: $iface_a / $iface_b"
[[ "$iface_a" != "$iface_b" ]] || fail "two Environments unexpectedly share host veth $iface_a"
assert_managed_ipv4 "$ip_a"
assert_managed_ipv4 "$ip_b"
[[ "$ip_a" != "$ip_b" ]] || fail "two Environments unexpectedly share routed IPv4 $ip_a"
[[ "$host_a" == "$proxy_ipv4" && "$host_b" == "$proxy_ipv4" ]] || fail "Environment routed host endpoint is not the Standard proxy address: $host_a / $host_b"

printf '==> Network security: verify strict reverse-path and host routes\n'
for entry in "$ref_a:$iface_a:$ip_a" "$ref_b:$iface_b:$ip_b"; do
  IFS=: read -r ref iface address <<<"$entry"
  [[ "$(cat "/proc/sys/net/ipv4/conf/$iface/rp_filter")" == "1" ]] || fail "$iface does not use strict rp_filter"
  ip -4 route show dev "$iface" | grep -Eq "^${address}(/32)?([[:space:]]|$)" || fail "$address does not have an exact host route through $iface"
  broad="$(ip -4 route show dev "$iface" | grep -E '^198\.(18|19)\.' | grep -v -E "^${address}(/32)?([[:space:]]|$)" || true)"
  [[ -z "$broad" ]] || fail "$iface has unexpected broad Hacocoon routes: $broad"
done

printf '==> Network security: verify host and per-Environment fail-closed policy\n'
host_firewall="$(nft list table inet hacocoon_sandbox)"
printf '%s\n' "$host_firewall" | grep -Fq 'iifname "haco*" ip daddr 169.254.254.1 tcp dport 18080 accept' || fail "host firewall lacks Standard egress proxy exception"
printf '%s\n' "$host_firewall" | grep -Fq 'iifname "haco*" drop' || fail "host firewall does not reject other Environment input"
printf '%s\n' "$host_firewall" | grep -Fq 'oifname "haco*" drop' || fail "host firewall does not reject forwarding into Environments"

guard_a="$(assert_guard "$iface_a" "$ip_a")"
guard_b="$(assert_guard "$iface_b" "$ip_b")"
[[ "$guard_a" != "$guard_b" ]] || fail "two Environments unexpectedly share anti-spoofing table $guard_a"

printf '==> Network security: Environment-to-Environment forwarding is blocked\n'
if incus exec "$ref_a" --project "$project" -- ping -4 -n -c 1 -W 1 "$ip_b" >/dev/null 2>&1; then
  fail "$env_a unexpectedly reached $env_b at $ip_b"
fi
if incus exec "$ref_b" --project "$project" -- ping -4 -n -c 1 -W 1 "$ip_a" >/dev/null 2>&1; then
  fail "$env_b unexpectedly reached $env_a at $ip_a"
fi

printf '==> Network security: spoofing another Environment IPv4 cannot reach the proxy\n'
incus exec "$ref_a" --project "$project" -- sh -eu -c "ip address add '$ip_b/32' dev eth0; ip route replace '$proxy_ipv4/32' dev eth0 src '$ip_b'"
if incus exec "$ref_a" --project "$project" -- bash -c "timeout 2 bash -c '</dev/tcp/$proxy_ipv4/$proxy_port'" >/dev/null 2>&1; then
  fail "$env_a reached the proxy while spoofing $env_b source IPv4 $ip_b"
fi

printf '==> Network security: deletion removes only that Environment source authority\n'
"$haco_bin" env delete "$env_a"
created_a=0
if nft list table inet "$guard_a" >/dev/null 2>&1; then
  fail "deleted Environment anti-spoofing guard $guard_a still exists"
fi
[[ ! -e "/proc/sys/net/ipv4/conf/$iface_a/rp_filter" ]] || fail "deleted Environment host veth $iface_a still exists"
nft list table inet "$guard_b" >/dev/null || fail "deleting $env_a damaged $env_b anti-spoofing guard"
[[ -e "/proc/sys/net/ipv4/conf/$iface_b/rp_filter" ]] || fail "deleting $env_a damaged $env_b host veth"

"$haco_bin" env delete "$env_b"
created_b=0
if nft list table inet "$guard_b" >/dev/null 2>&1; then
  fail "deleted Environment anti-spoofing guard $guard_b still exists"
fi
[[ ! -e "/proc/sys/net/ipv4/conf/$iface_b/rp_filter" ]] || fail "deleted Environment host veth $iface_b still exists"

printf 'installer network security: PASS\n'
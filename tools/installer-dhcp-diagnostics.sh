#!/usr/bin/env bash
set -u

haco_bin="${HACO_BIN:-/usr/local/bin/haco}"
project="${HACO_E2E_INCUS_PROJECT:-hacocoon}"
network_project="${HACO_E2E_NETWORK_PROJECT:-default}"
workspace="$(mktemp -d)"
name="dhcp-diagnostic-${RANDOM}-$$"
ref="haco-$name"
created=0
standalone_refs=()
original_bridge_nf_ipv4=""
original_apparmor_userns=""

cleanup() {
  set +e
  local standalone
  for standalone in "${standalone_refs[@]}"; do
    incus delete "$standalone" --project "$project" --force >/dev/null 2>&1 || true
  done
  if [[ "$created" == "1" ]]; then
    "$haco_bin" env delete "$name" >/dev/null 2>&1 || true
  fi
  if [[ -n "$original_bridge_nf_ipv4" ]] && [[ -e /proc/sys/net/bridge/bridge-nf-call-iptables ]]; then
    sysctl -q -w "net.bridge.bridge-nf-call-iptables=$original_bridge_nf_ipv4" >/dev/null 2>&1 || true
  fi
  if [[ -n "$original_apparmor_userns" ]] && [[ -e /proc/sys/kernel/apparmor_restrict_unprivileged_unconfined ]]; then
    sysctl -q -w "kernel.apparmor_restrict_unprivileged_unconfined=$original_apparmor_userns" >/dev/null 2>&1 || true
  fi
  rm -rf "$workspace"
}
trap cleanup EXIT

print_guest_network_state() {
  local guest_ref="$1"
  incus exec "$guest_ref" --project "$project" -- sh -c '
    set +e
    echo "[pid 1]"
    cat /proc/1/comm 2>/dev/null || true
    echo "[processes]"
    ps -ef 2>/dev/null || true
    echo "[ip link/address/route]"
    command -v ip >/dev/null 2>&1 && { ip -s link show eth0; ip -4 -o addr show eth0; ip route; }
    echo "[systemd]"
    command -v systemctl >/dev/null 2>&1 && {
      systemctl is-system-running 2>&1 || true
      systemctl status systemd-networkd --no-pager 2>&1 || true
    }
    echo "[networkctl]"
    command -v networkctl >/dev/null 2>&1 && networkctl status eth0 --no-pager 2>&1 || true
    echo "[networkd journal]"
    command -v journalctl >/dev/null 2>&1 && journalctl -b -u systemd-networkd --no-pager -n 120 2>&1 || true
    echo "[network config files]"
    for f in /etc/netplan/* /etc/systemd/network/* /run/systemd/network/*; do
      [ -f "$f" ] || continue
      echo "### $f"
      sed -n "1,160p" "$f"
    done
  ' || true
}

instance_ipv4() {
  local guest_ref="$1"
  incus list "$guest_ref" --project "$project" --format json 2>/dev/null | python3 -c '
import json, sys
try:
    rows = json.load(sys.stdin)
except Exception:
    rows = []
for row in rows:
    for iface in ((row.get("state") or {}).get("network") or {}).values():
        for item in iface.get("addresses") or []:
            address = item.get("address", "")
            if item.get("family") == "inet" and address and not address.startswith("127.") and not address.startswith("169.254."):
                print(address)
                raise SystemExit(0)
' 2>/dev/null | head -n1
}

standalone_incus_probe() {
  local label="$1"
  local bridge_nf_value="$2"
  local apparmor_userns_value="$3"
  local fingerprint="$4"
  local root_pool="$5"
  local standalone="incus-dhcp-${label}-${RANDOM}-$$"
  local capture="/tmp/${standalone}.tcpdump"
  local capture_pid=""
  local ip=""

  printf '\n=== Standalone Incus DHCP probe: %s (bridge-nf=%s, apparmor-userns=%s) ===\n' "$label" "$bridge_nf_value" "$apparmor_userns_value"
  if [[ -e /proc/sys/net/bridge/bridge-nf-call-iptables ]]; then
    sysctl -w "net.bridge.bridge-nf-call-iptables=$bridge_nf_value" || true
  fi
  if [[ "$apparmor_userns_value" != "keep" ]] && [[ -e /proc/sys/kernel/apparmor_restrict_unprivileged_unconfined ]]; then
    sysctl -w "kernel.apparmor_restrict_unprivileged_unconfined=$apparmor_userns_value" || true
  fi

  # Use the exact image and storage already proven usable by Hacocoon, but do
  # not call Hacocoon and do not use an Environment bridge or Hacocoon nft
  # policy. The only NIC is a vanilla Incus NIC on the installer's incusbr0.
  if ! incus init "$fingerprint" "$standalone" --project "$project" --no-profiles --storage "$root_pool"; then
    printf 'standalone probe %s: incus init failed\n' "$label"
    return 0
  fi
  standalone_refs+=("$standalone")
  if ! incus config device add "$standalone" eth0 nic network=incusbr0 name=eth0 --project "$project"; then
    printf 'standalone probe %s: adding incusbr0 NIC failed\n' "$label"
    return 0
  fi

  if command -v tcpdump >/dev/null 2>&1; then
    timeout 12 tcpdump -ni incusbr0 -e -vvv 'udp port 67 or udp port 68' >"$capture" 2>&1 &
    capture_pid=$!
    sleep 0.2
  fi

  if ! incus start "$standalone" --project "$project"; then
    printf 'standalone probe %s: start failed\n' "$label"
  else
    sleep 8
  fi

  ip="$(instance_ipv4 "$standalone" | tr -d '[:space:]')"
  if [[ -n "$ip" ]]; then
    printf 'STANDALONE_RESULT label=%s bridge_nf=%s apparmor_userns=%s ipv4=%s\n' "$label" "$bridge_nf_value" "$apparmor_userns_value" "$ip"
  else
    printf 'STANDALONE_RESULT label=%s bridge_nf=%s apparmor_userns=%s ipv4=<none>\n' "$label" "$bridge_nf_value" "$apparmor_userns_value"
  fi
  incus list "$standalone" --project "$project" --format json || true
  print_guest_network_state "$standalone"
  printf '%s\n' '[incusbr0 leases]'
  incus network list-leases incusbr0 --project "$network_project" || true

  if [[ -n "$capture_pid" ]]; then
    wait "$capture_pid" 2>/dev/null || true
    printf '%s\n' '[incusbr0 DHCP packet capture]'
    cat "$capture" 2>/dev/null || true
    rm -f "$capture"
  fi

  incus delete "$standalone" --project "$project" --force >/dev/null 2>&1 || true
}

printf '%s\n' '=== Hacocoon DHCP diagnostic reproduction ==='
mkdir -p "$workspace/workspace"
if ! "$haco_bin" env create --read-only --workspace "$workspace/workspace" "$name"; then
  printf '%s\n' 'diagnostic Environment creation itself failed'
  exit 0
fi
created=1

bridge="$(incus config device get "$ref" eth0 network --project "$project" 2>/dev/null || true)"
host_veth="$(incus config get "$ref" volatile.eth0.host_name --project "$project" 2>/dev/null | tr -d '[:space:]' || true)"
fingerprint="$(incus config get "$ref" volatile.base_image --project "$project" 2>/dev/null | tr -d '[:space:]' || true)"
root_pool="$(incus config device get "$ref" root pool --project "$project" 2>/dev/null | tr -d '[:space:]' || true)"
printf 'Environment: %s\nbridge: %s\nhost-veth: %s\nbase-image: %s\nroot-pool: %s\n' "$ref" "${bridge:-<none>}" "${host_veth:-<none>}" "${fingerprint:-<none>}" "${root_pool:-<none>}"

if [[ -e /proc/sys/net/bridge/bridge-nf-call-iptables ]]; then
  original_bridge_nf_ipv4="$(cat /proc/sys/net/bridge/bridge-nf-call-iptables 2>/dev/null || true)"
fi
if [[ -e /proc/sys/kernel/apparmor_restrict_unprivileged_unconfined ]]; then
  original_apparmor_userns="$(cat /proc/sys/kernel/apparmor_restrict_unprivileged_unconfined 2>/dev/null || true)"
fi
printf 'host sysctls: bridge-nf-call-iptables=%s apparmor_restrict_unprivileged_unconfined=%s\n' \
  "${original_bridge_nf_ipv4:-<unavailable>}" "${original_apparmor_userns:-<unavailable>}"

printf '%s\n' '--- instance config/state ---'
incus config show "$ref" --project "$project" --expanded || true
incus list "$ref" --project "$project" --format json || true

printf '%s\n' '--- guest network configuration/status ---'
print_guest_network_state "$ref"

if [[ -n "$bridge" ]]; then
  printf '%s\n' '--- managed bridge config/state ---'
  incus network show "$bridge" --project "$network_project" || true
  incus network list-leases "$bridge" --project "$network_project" || true
  ip -s link show "$bridge" || true
  printf '%s\n' '--- Incus network runtime directory ---'
  ls -la "/var/lib/incus/networks/$bridge" 2>/dev/null || true
  for f in "/var/lib/incus/networks/$bridge"/*; do
    [ -f "$f" ] || continue
    case "$f" in
      *.pid|*.leases|*.hosts|*.raw|*.conf|*dnsmasq*)
        printf '### %s\n' "$f"
        sed -n '1,200p' "$f" 2>/dev/null || true
        ;;
    esac
  done
fi

if [[ -n "$host_veth" ]]; then
  printf '%s\n' '--- host veth counters ---'
  ip -s link show "$host_veth" || true
fi

printf '%s\n' '--- DHCP listeners/processes ---'
ss -lunp 2>/dev/null | grep -E '(:67[[:space:]]|dnsmasq)' || true
ps -ef | grep '[d]nsmasq' || true

printf '%s\n' '--- relevant firewall state ---'
nft list ruleset || true

printf '%s\n' '--- Incus daemon logs ---'
journalctl -u incus.service --no-pager -n 200 || true

printf '%s\n' '--- state after a short DHCP retry window ---'
sleep 5
incus list "$ref" --project "$project" --format json || true
if [[ -n "$bridge" ]]; then
  incus network list-leases "$bridge" --project "$network_project" || true
fi

if [[ -n "$fingerprint" && -n "$root_pool" ]]; then
  if [[ -n "$original_apparmor_userns" ]]; then
    sysctl -q -w "kernel.apparmor_restrict_unprivileged_unconfined=$original_apparmor_userns" || true
  fi
  standalone_incus_probe "baseline-brnf-on" 1 keep "$fingerprint" "$root_pool"
  standalone_incus_probe "baseline-brnf-off" 0 keep "$fingerprint" "$root_pool"
  standalone_incus_probe "apparmor-userns-off" 1 0 "$fingerprint" "$root_pool"
else
  printf '%s\n' 'standalone Incus probes skipped: could not resolve image fingerprint/root pool'
fi

#!/usr/bin/env bash
set -u

haco_bin="${HACO_BIN:-/usr/local/bin/haco}"
project="${HACO_E2E_INCUS_PROJECT:-hacocoon}"
network_project="${HACO_E2E_NETWORK_PROJECT:-default}"
workspace="$(mktemp -d)"
name="dhcp-diagnostic-${RANDOM}-$$"
ref="haco-$name"
created=0

cleanup() {
  set +e
  if [[ "$created" == "1" ]]; then
    "$haco_bin" env delete "$name" >/dev/null 2>&1 || true
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

printf '%s\n' '=== Hacocoon DHCP failure diagnostics ==='
printf '%s\n' '--- host compatibility state ---'
sysctl net.bridge.bridge-nf-call-iptables 2>/dev/null || true
sysctl net.bridge.bridge-nf-call-ip6tables 2>/dev/null || true
sysctl kernel.apparmor_restrict_unprivileged_unconfined 2>/dev/null || true
if [[ -f /etc/sysctl.d/90-hacocoon-incus-userns.conf ]]; then
  printf '%s\n' '### /etc/sysctl.d/90-hacocoon-incus-userns.conf'
  cat /etc/sysctl.d/90-hacocoon-incus-userns.conf || true
fi

mkdir -p "$workspace/workspace"
if ! "$haco_bin" env create --read-only --workspace "$workspace/workspace" "$name"; then
  printf '%s\n' 'diagnostic Environment creation itself failed'
  exit 0
fi
created=1

bridge="$(incus config device get "$ref" eth0 network --project "$project" 2>/dev/null || true)"
host_veth="$(incus config get "$ref" volatile.eth0.host_name --project "$project" 2>/dev/null | tr -d '[:space:]' || true)"
printf 'Environment: %s\nbridge: %s\nhost-veth: %s\n' "$ref" "${bridge:-<none>}" "${host_veth:-<none>}"

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
    [[ -f "$f" ]] || continue
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

printf '%s\n' '--- final state after a short observation window ---'
sleep 5
incus list "$ref" --project "$project" --format json || true
if [[ -n "$bridge" ]]; then
  incus network list-leases "$bridge" --project "$network_project" || true
fi

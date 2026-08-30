#!/usr/bin/env bash
set -euo pipefail

if [[ "${HACO_CI_INCUS_STANDALONE:-}" != "1" ]]; then
  echo "SKIP: standalone Incus E2E is only enabled by tools/ci-incus.sh"
  exit 0
fi

for command in incus awk grep timeout ip; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

prefix="${HACO_CI_PREFIX:?HACO_CI_PREFIX is required}"
[[ "$prefix" =~ ^hci-[0-9]+-[0-9]+$ ]] || {
  echo "unsafe Incus CI prefix: $prefix" >&2
  exit 1
}

primary_network="${prefix}n"
aux_network="${prefix}x"
profile="${prefix}p"
first="${prefix}a"
second="${prefix}b"
volume="${prefix}v"
snapshot="baseline"

pool="$(incus profile device get default root pool --project default)"
[[ -n "$pool" ]] || {
  echo "default Incus profile has no root storage pool" >&2
  exit 1
}

echo "standalone prefix: $prefix"
echo "default storage pool: $pool"
incus storage show "$pool"

incus network create "$primary_network" ipv4.address=auto ipv4.nat=true ipv6.address=none
incus network create "$aux_network" ipv4.address=auto ipv4.nat=false ipv6.address=none
incus profile create "$profile"
incus profile device add "$profile" root disk path=/ pool="$pool"
incus profile device add "$profile" eth0 nic name=eth0 network="$primary_network"

incus launch images:ubuntu/26.04 "$first" --profile "$profile"
incus launch images:ubuntu/26.04 "$second" --profile "$profile"

wait_for_guest() {
  local instance="$1"
  local attempt state stuck_samples=0
  for attempt in $(seq 1 60); do
    if incus exec "$instance" -- sh -c 'test "$(cat /proc/1/comm)" = systemd' >/dev/null 2>&1; then
      state="$(incus exec "$instance" -- systemctl is-system-running 2>/dev/null || true)"
      case "$state" in
        running|degraded) return 0 ;;
      esac

      # Ubuntu 26.04 hosts can break Incus AppArmor namespacing when the host
      # vendor user-namespace restriction is left enabled. That failure leaves
      # systemd helpers stuck in sd-mkuserns and prevents DHCP. Detect a
      # sustained stall rather than waiting the full timeout.
      if [[ "$state" == "initializing" ]] && incus exec "$instance" -- sh -c 'ps -eo comm= | grep -Fxq sd-mkuserns' >/dev/null 2>&1; then
        stuck_samples=$((stuck_samples + 1))
        if (( stuck_samples >= 10 )); then
          echo "guest $instance is persistently stuck in sd-mkuserns; check host Incus/AppArmor compatibility" >&2
          incus exec "$instance" -- systemctl status --no-pager || true
          return 1
        fi
      else
        stuck_samples=0
      fi
    fi
    sleep 1
  done
  echo "guest $instance did not reach usable systemd state within 60s" >&2
  incus exec "$instance" -- systemctl status --no-pager || true
  return 1
}

wait_for_ipv4() {
  local instance="$1"
  local device="$2"
  local attempt address
  for attempt in $(seq 1 30); do
    # Parse on the host so shell positional parameters in awk are never
    # accidentally expanded by the host shell before incus exec runs.
    address="$(incus exec "$instance" -- ip -4 -o addr show dev "$device" scope global 2>/dev/null | awk '{split($4, parts, "/"); print parts[1]; exit}' || true)"
    if [[ -n "$address" ]]; then
      printf '%s\n' "$address"
      return 0
    fi
    sleep 1
  done
  echo "guest $instance did not receive IPv4 on $device within 30s" >&2
  return 1
}

wait_for_guest "$first"
wait_for_guest "$second"

[[ "$(incus list "$first" --format csv -c s)" == "RUNNING" ]]
[[ "$(incus list "$second" --format csv -c s)" == "RUNNING" ]]
incus exec "$first" -- sh -c 'printf standalone-exec-ok >/root/incus-ci-exec'
[[ "$(incus exec "$first" -- cat /root/incus-ci-exec)" == "standalone-exec-ok" ]]

first_ip="$(wait_for_ipv4 "$first" eth0)"
second_ip="$(wait_for_ipv4 "$second" eth0)"
gateway="$(incus exec "$first" -- ip -4 route show default | awk '{print $3; exit}')"
[[ -n "$gateway" ]]
echo "primary network: $primary_network"
echo "first IPv4: $first_ip"
echo "second IPv4: $second_ip"
echo "bridge gateway: $gateway"

# DHCP/default route plus actual reachability of the managed bridge gateway.
incus exec "$first" -- timeout 5 bash -c "exec 3<>/dev/tcp/$gateway/53"
# DNS resolution through the network-provided resolver is a distinct check.
incus exec "$first" -- getent ahostsv4 example.com >/dev/null
# GitHub-hosted runners can block selected public IP literals independently of
# general Internet access (1.1.1.1 is one observed example). Verify real
# hostname-based IPv4 egress to GitHub instead of baking a third-party IP
# policy into the substrate acceptance test.
external_ip="$(incus exec "$first" -- getent ahostsv4 github.com | awk '$2 == "STREAM" {print $1; exit}')"
[[ -n "$external_ip" ]] || {
  echo "guest DNS resolved no IPv4 STREAM address for github.com" >&2
  exit 1
}
echo "public egress target: github.com ($external_ip):443"
if ! incus exec "$first" -- timeout 5 bash -c "exec 3<>/dev/tcp/$external_ip/443"; then
  echo "guest cannot reach github.com IPv4 $external_ip:443 through Incus NAT" >&2
  exit 1
fi
# Managed-network hostname resolution and peer-to-peer traffic.
incus exec "$first" -- getent ahostsv4 "$second" >/dev/null
incus exec "$first" -- ping -c 1 -W 3 "$second" >/dev/null

incus network show "$primary_network"
incus network show "$aux_network"
[[ "$(incus network get "$primary_network" ipv4.nat)" == "true" ]]
[[ "$(incus network get "$primary_network" ipv6.address)" == "none" ]]
[[ "$(sysctl -n net.ipv4.ip_forward)" == "1" ]]
ip link show "$primary_network" >/dev/null

# Safe hot attach/detach coverage on an independent CI-owned managed bridge.
incus config device add "$first" ci-aux nic name=eth1 network="$aux_network"
aux_ip="$(wait_for_ipv4 "$first" eth1)"
[[ -n "$aux_ip" ]]
echo "aux IPv4: $aux_ip"
incus config device remove "$first" ci-aux
for _ in $(seq 1 20); do
  if ! incus exec "$first" -- ip link show eth1 >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if incus exec "$first" -- ip link show eth1 >/dev/null 2>&1; then
  echo "hot-detached eth1 still exists" >&2
  exit 1
fi

# PID 1 and an ordinary systemd service must behave like a system container.
[[ "$(incus exec "$first" -- cat /proc/1/comm)" == "systemd" ]]
incus exec "$first" -- bash -c "cat >/etc/systemd/system/incus-ci.service <<'UNIT'
[Unit]
Description=Incus CI systemd probe

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/sh -c 'printf started >/run/incus-ci-service'
ExecReload=/bin/sh -c 'printf reloaded >/run/incus-ci-service'
UNIT
systemctl daemon-reload
systemctl start incus-ci.service"
incus exec "$first" -- systemctl is-active --quiet incus-ci.service
incus exec "$first" -- systemctl restart incus-ci.service
incus exec "$first" -- systemctl status incus-ci.service --no-pager
incus exec "$first" -- journalctl -u incus-ci.service --no-pager -n 50

# A custom storage volume must remain writable and persistent across restart.
incus storage volume create "$pool" "$volume"
incus storage volume attach "$pool" "$volume" "$first" /mnt/ci-volume
incus exec "$first" -- sh -c 'printf persistent-volume >/mnt/ci-volume/value'
incus restart "$first"
wait_for_guest "$first"
[[ "$(incus exec "$first" -- cat /mnt/ci-volume/value)" == "persistent-volume" ]]
incus exec "$first" -- systemctl is-active --quiet incus-ci.service

# Rootfs snapshot/restore should restore guest filesystem state.
incus exec "$first" -- sh -c 'printf before-snapshot >/root/snapshot-state'
incus snapshot create "$first" "$snapshot"
incus exec "$first" -- sh -c 'printf after-snapshot >/root/snapshot-state'
incus stop "$first"
[[ "$(incus list "$first" --format csv -c s)" == "STOPPED" ]]
incus snapshot restore "$first" "$snapshot"
incus start "$first"
wait_for_guest "$first"
[[ "$(incus exec "$first" -- cat /root/snapshot-state)" == "before-snapshot" ]]
[[ "$(incus exec "$first" -- cat /mnt/ci-volume/value)" == "persistent-volume" ]]

# Explicit lifecycle coverage after restore.
incus restart "$first"
wait_for_guest "$first"
incus stop "$first"
[[ "$(incus list "$first" --format csv -c s)" == "STOPPED" ]]
incus start "$first"
wait_for_guest "$first"
[[ "$(incus list "$first" --format csv -c s)" == "RUNNING" ]]

# Delete is part of the test, not only emergency cleanup.
incus delete "$second" --force
incus delete "$first" --force
if incus info "$first" >/dev/null 2>&1 || incus info "$second" >/dev/null 2>&1; then
  echo "standalone instance remained after delete" >&2
  exit 1
fi

echo "PASS: standalone Incus daemon/container/systemd/network/storage lifecycle"

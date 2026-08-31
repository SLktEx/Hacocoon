#!/usr/bin/env bash
set -euo pipefail

module_path=""
if [[ "${1:-}" == "--module" ]]; then
  module_path="${2:-}"
  shift 2
fi
if [[ $# -ne 0 ]]; then
  echo "Usage: $0 [--module /path/to/haco_nft_bridge.ko]" >&2
  exit 2
fi

if [[ $EUID -ne 0 ]]; then
  echo "Run this probe as root (sudo)." >&2
  exit 2
fi

for cmd in nft modprobe modinfo uname grep gzip; do
  command -v "$cmd" >/dev/null || {
    echo "Missing required command: $cmd" >&2
    exit 2
  }
done

release="$(uname -r)"
echo "kernel: $release"
if [[ "$release" != *microsoft-standard-WSL2* ]]; then
  echo "warning: this host does not look like the stock Microsoft WSL2 kernel" >&2
fi

if [[ -r /proc/config.gz ]]; then
  echo "kernel config:"
  gzip -cd /proc/config.gz | grep -E '^(CONFIG_(NF_TABLES|NF_TABLES_BRIDGE|NETFILTER_FAMILY_BRIDGE|BRIDGE|MODVERSIONS|MODULE_SIG)=|# CONFIG_NF_TABLES_BRIDGE is not set)' || true
else
  echo "warning: /proc/config.gz is not readable" >&2
fi

modprobe bridge

probe_table="haco_probe_$$"
cleanup() {
  nft delete table bridge "$probe_table" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# A bridge table alone is not enough: CONFIG_NF_TABLES_BRIDGE supplies the
# default filter chain type and bridge hooks. Also add representative rules
# copied from the shapes Incus uses for MAC/IP anti-spoofing so the probe does
# not declare success on a chain implementation that cannot host them.
bridge_filter_works() {
  cleanup
  nft add table bridge "$probe_table" || return 1
  nft "add chain bridge $probe_table haco_input { type filter hook input priority -200; policy accept; }" || return 1

  nft "add rule bridge $probe_table haco_input iifname \"haco-probe0\" ether saddr != 02:00:00:00:00:01 drop" || return 1
  nft "add rule bridge $probe_table haco_input iifname \"haco-probe0\" ether type arp arp saddr ether != 02:00:00:00:00:01 drop" || return 1
  nft "add rule bridge $probe_table haco_input iifname \"haco-probe0\" ether type arp arp saddr ip != 192.0.2.10 drop" || return 1
  nft "add rule bridge $probe_table haco_input iifname \"haco-probe0\" ether type ip ip saddr != 192.0.2.10 drop" || return 1
  nft "add rule bridge $probe_table haco_input iifname \"haco-probe0\" ether type ip6 ip6 saddr != 2001:db8::10 drop" || return 1
  nft "add rule bridge $probe_table haco_input iifname \"haco-probe0\" ether type ip6 icmpv6 type 134 drop" || return 1
  nft "add rule bridge $probe_table haco_input iifname \"haco-probe0\" ether type != { arp, ip, ip6 } drop" || return 1

  cleanup
  return 0
}

if bridge_filter_works; then
  echo "PASS: nftables bridge filter hooks and Incus-style anti-spoofing rules already work; no shim is required."
  exit 0
fi

echo "bridge filter/Incus-rule probe failed on the running kernel"

if [[ -z "$module_path" ]]; then
  echo "NEEDS_SHIM: retry with --module /path/to/haco_nft_bridge.ko to test the compatibility shim." >&2
  exit 10
fi
if [[ ! -f "$module_path" ]]; then
  echo "Module not found: $module_path" >&2
  exit 2
fi

module_release="$(modinfo -F vermagic "$module_path" | awk '{print $1}')"
if [[ "$module_release" != "$release" ]]; then
  echo "Refusing module with mismatched vermagic: module=$module_release running=$release" >&2
  exit 11
fi

if ! grep -q '^haco_nft_bridge ' /proc/modules; then
  insmod "$module_path"
fi

if ! bridge_filter_works; then
  echo "FAIL: shim loaded but nftables bridge filter/Incus anti-spoofing rule creation still fails." >&2
  dmesg | tail -n 80 >&2 || true
  exit 12
fi

echo "PASS: shim restored nftables bridge filter support needed by Incus anti-spoofing on $release"
echo "The module is intentionally left loaded for the real Incus/Hacocoon validation."

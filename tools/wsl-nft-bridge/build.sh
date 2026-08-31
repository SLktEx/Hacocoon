#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/../.." && pwd)"
work_root="${HACO_WSL_KERNEL_WORKDIR:-$repo_root/.cache/wsl-kernel}"
out_dir="${HACO_WSL_NFT_BRIDGE_OUT:-$script_dir/out}"

kernel_tag="${1:-${WSL_KERNEL_TAG:-}}"
if [[ -z "$kernel_tag" ]]; then
  release="$(uname -r)"
  if [[ "$release" =~ ^([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)-microsoft-standard-WSL2$ ]]; then
    kernel_tag="linux-msft-wsl-${BASH_REMATCH[1]}"
  else
    echo "Unable to derive Microsoft WSL kernel tag from uname -r: $release" >&2
    echo "Pass a tag explicitly, for example: $0 linux-msft-wsl-6.18.40.1" >&2
    exit 2
  fi
fi

src_dir="$work_root/$kernel_tag"
mkdir -p "$work_root" "$out_dir"

if [[ ! -d "$src_dir/.git" ]]; then
  rm -rf "$src_dir"
  git clone \
    --depth 1 \
    --branch "$kernel_tag" \
    https://github.com/microsoft/WSL2-Linux-Kernel.git \
    "$src_dir"
fi

if ! grep -q '^CONFIG_NF_TABLES=y$' "$src_dir/Microsoft/config-wsl"; then
  echo "Target WSL config does not provide CONFIG_NF_TABLES=y" >&2
  exit 3
fi
if ! grep -q '^CONFIG_NETFILTER_FAMILY_BRIDGE=y$' "$src_dir/Microsoft/config-wsl"; then
  echo "Target WSL config does not provide CONFIG_NETFILTER_FAMILY_BRIDGE=y" >&2
  exit 3
fi
if grep -q '^CONFIG_NF_TABLES_BRIDGE=[ym]$' "$src_dir/Microsoft/config-wsl"; then
  echo "Target kernel already enables CONFIG_NF_TABLES_BRIDGE; shim is unnecessary" >&2
  exit 4
fi

jobs="${HACO_WSL_KERNEL_JOBS:-$(nproc)}"

echo "Building exact Microsoft WSL kernel tree for module symbol versions: $kernel_tag"
make -C "$src_dir" -j"$jobs" KCONFIG_CONFIG=Microsoft/config-wsl

echo "Building Hacocoon bridge nftables compatibility shim"
make -C "$src_dir" M="$script_dir" modules

kernel_release="$(make -s -C "$src_dir" KCONFIG_CONFIG=Microsoft/config-wsl kernelrelease)"
module_path="$script_dir/haco_nft_bridge.ko"
if [[ ! -s "$module_path" ]]; then
  echo "Expected kernel module was not produced: $module_path" >&2
  exit 5
fi

module_release="$(modinfo -F vermagic "$module_path" | awk '{print $1}')"
if [[ "$module_release" != "$kernel_release" ]]; then
  echo "Module vermagic mismatch: module=$module_release kernel=$kernel_release" >&2
  exit 6
fi

install -m 0644 "$module_path" "$out_dir/haco_nft_bridge.ko"
printf '%s\n' "$kernel_release" > "$out_dir/kernel-release.txt"
printf '%s\n' "$kernel_tag" > "$out_dir/kernel-tag.txt"

printf 'Built %s for %s\n' "$out_dir/haco_nft_bridge.ko" "$kernel_release"

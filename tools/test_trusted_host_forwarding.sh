#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
for command in go sudo unshare nsenter ip iptables ping sysctl; do
  command -v "$command" >/dev/null || { printf 'required command missing: %s\n' "$command" >&2; exit 1; }
done
test_dir="$(mktemp -d)"
trap 'rm -rf -- "$test_dir"' EXIT
go test -c -o "$test_dir/incus-network.test" ./modules/runtime/incus
# The binary also refuses the PID-1 network namespace. Product BAT acceptance
# never calls this fixture; it is a separate Linux kernel regression.
sudo -n unshare --net env HACO_FORWARDING_KERNEL_TEST=isolated "$test_dir/incus-network.test" -test.run '^TestTrustedHostForwardingKernel$' -test.v

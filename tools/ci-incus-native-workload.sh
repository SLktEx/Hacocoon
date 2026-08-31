#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

[[ "${GITHUB_ACTIONS:-}" == "true" ]] || fail "native workload Incus acceptance only runs inside GitHub Actions"
[[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || fail "native workload Incus acceptance requires a GitHub-hosted runner"
[[ "$(uname -s)" == "Linux" ]] || fail "native workload Incus acceptance requires Linux"
[[ -n "${RUNNER_TEMP:-}" ]] || fail "RUNNER_TEMP is required"

readonly CLIENT_CONF="${HACO_CI_INCUS_CONF:-${RUNNER_TEMP}/haco-incus-client}"
export INCUS_CONF="$CLIENT_CONF"
[[ -s "$CLIENT_CONF/config.yml" ]] || fail "trusted Incus TLS client is missing at $CLIENT_CONF; run tools/ci-incus.sh setup first"

# Privileged real-Incus enablement stays inside this repository-controlled CI
# helper. Normal PR workflow YAML must never carry HACO_E2E_INCUS=true/1.
export HACO_E2E_INCUS=1
export HACO_WORKLOAD_SHIM_BINARY="${RUNNER_TEMP}/haco-host-incus-native-e2e"
export HACO_CONTROL_SOCKET="${RUNNER_TEMP}/haco-native-e2e/control.sock"

mkdir -p "$(dirname "$HACO_CONTROL_SOCKET")"
go build -trimpath -o "$HACO_WORKLOAD_SHIM_BINARY" ./cmd/haco-host
chmod 0755 "$HACO_WORKLOAD_SHIM_BINARY"
go test -count=1 -run '^TestRealIncusNerdctlUsesHostOCIWorkloadE2E$' ./modules/runtime/incus

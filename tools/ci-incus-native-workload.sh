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

# Privileged real-Incus enablement stays inside this repository-controlled CI
# helper. Normal PR workflow YAML must never carry HACO_E2E_INCUS=true/1.
export HACO_E2E_INCUS=1
export HACO_WORKLOAD_SHIM_BINARY="${RUNNER_TEMP}/haco-host-incus-native-e2e"
export HACO_CONTROL_SOCKET="${RUNNER_TEMP}/haco-native-e2e/control.sock"

mkdir -p "$(dirname "$HACO_CONTROL_SOCKET")"
go build -trimpath -o "$HACO_WORKLOAD_SHIM_BINARY" ./cmd/haco-host
go test -count=1 -run '^TestRealIncusNerdctlUsesHostOCIWorkloadE2E$' ./modules/runtime/incus

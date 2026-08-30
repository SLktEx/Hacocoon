#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == "--managed-ci" ]]; then
  shift
  [[ "${GITHUB_ACTIONS:-}" == "true" && "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || {
    echo 'managed host E2E requires the GitHub-hosted CI substrate' >&2
    exit 2
  }
  [[ -n "${HACO_E2E_HACO_BIN:-}" && -n "${HACO_E2E_SHARED_ROOT:-}" ]] || {
    echo 'managed host E2E requires the prebuilt haco binary and shared root' >&2
    exit 2
  }
elif [[ "${HACO_E2E_INCUS:-}" != "1" ]]; then
  echo "SKIP: set HACO_E2E_INCUS=1 on a supported Incus host"
  exit 0
fi
[[ "$#" == "0" ]] || { echo "usage: $0 [--managed-ci]" >&2; exit 2; }

for command in go incus grep mktemp script timeout; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done
incus version >/dev/null

root="$(mktemp -d)"
haco="${HACO_E2E_HACO_BIN:-$root/haco}"
haco_host="$(dirname "$haco")/haco-host"
export HACO_ROOT="${HACO_E2E_SHARED_ROOT:-$root/haco-root}"
created=0
built_companion=0

cleanup() {
  set +e
  if incus list haco-host --project hacocoon --format csv -c n 2>/dev/null | grep -Fx haco-host >/dev/null 2>&1; then
    incus delete haco-host --project hacocoon --force >/dev/null 2>&1 || true
  fi
  if [[ "$built_companion" == "1" ]]; then
    rm -f -- "$haco_host"
  fi
  rm -rf "$root"
}
trap cleanup EXIT

if [[ -z "${HACO_E2E_HACO_BIN:-}" ]]; then
  go build -o "$haco" ./cmd/haco
fi
[[ -x "$haco" ]]

# `haco host ensure` provisions the companion logical Host binary from beside
# the main haco executable. Build that release-layout sibling explicitly when
# the managed storage substrate only provided the haco binary itself.
if [[ ! -x "$haco_host" ]]; then
  go build -o "$haco_host" ./cmd/haco-host
  built_companion=1
fi
[[ -x "$haco_host" ]]

"$haco" host ensure
created=1
state="$(incus list haco-host --project hacocoon --format csv -c s)"
[[ "$state" == "RUNNING" ]]
marker="$(incus config get haco-host user.hacocoon.role --project hacocoon)"
[[ "$marker" == "trusted-host" ]]

# Reconciliation is intentionally idempotent.
"$haco" host ensure
[[ "$(incus list haco-host --project hacocoon --format csv -c s)" == "RUNNING" ]]

# A persistent trusted Host that was stopped must be recovered rather than
# replaced or left unavailable.
incus stop haco-host --project hacocoon
[[ "$(incus list haco-host --project hacocoon --format csv -c s)" == "STOPPED" ]]
"$haco" host ensure
[[ "$(incus list haco-host --project hacocoon --format csv -c s)" == "RUNNING" ]]
[[ "$(incus config get haco-host user.hacocoon.role --project hacocoon)" == "trusted-host" ]]

# The trusted Host is privileged, but it must not gain Incus daemon authority
# through a mounted control socket.
incus config show haco-host --expanded --project hacocoon >"$root/host-config"
if grep -Fq '/var/lib/incus/unix.socket' "$root/host-config" || grep -Fq '/var/lib/incus/unix.socket.user' "$root/host-config"; then
  echo 'trusted host unexpectedly exposes Incus control authority' >&2
  exit 1
fi

# Use a pseudo-terminal so the real interactive shell path is exercised. The
# login shell receives `exit` immediately, keeping the acceptance deterministic.
printf 'exit\n' | LC_ALL=C timeout 30s script -q -e -c "$haco host shell" /dev/null >"$root/shell.out" 2>&1
grep -Fq 'privileged management environment' "$root/shell.out"
grep -Fq 'Physical Host' "$root/shell.out"

incus delete haco-host --project hacocoon --force
created=0
if incus list haco-host --project hacocoon --format csv -c n | grep -Fx haco-host >/dev/null 2>&1; then
  echo 'trusted host remained after E2E cleanup' >&2
  exit 1
fi

echo 'PASS: haco host ensure/shell -> real Incus E2E'
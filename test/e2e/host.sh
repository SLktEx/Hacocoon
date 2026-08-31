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

for command in go incus grep mktemp; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done
incus version >/dev/null

root="$(mktemp -d)"
haco="${HACO_E2E_HACO_BIN:-$root/haco}"
bin_dir="$(dirname "$haco")"
haco_host="$bin_dir/haco-host"
controller="$bin_dir/haco-controller"
controller_log="$root/controller.log"
control_socket="$root/control.sock"
export HACO_ROOT="${HACO_E2E_SHARED_ROOT:-$root/haco-root}"
export HACO_CONTROL_SOCKET="$control_socket"
created=0
built_host=0
built_controller=0
controller_pid=""

cleanup() {
  local code=$?
  set +e
  if [[ "$code" != "0" && -f "$controller_log" ]]; then
    echo '::group::haco-controller stderr' >&2
    cat "$controller_log" >&2 || true
    echo '::endgroup::' >&2
  fi
  if [[ -n "$controller_pid" ]]; then
    kill "$controller_pid" >/dev/null 2>&1 || true
    wait "$controller_pid" >/dev/null 2>&1 || true
  fi
  if incus list haco-host --project hacocoon --format csv -c n 2>/dev/null | grep -Fx haco-host >/dev/null 2>&1; then
    incus delete haco-host --project hacocoon --force >/dev/null 2>&1 || true
  fi
  if [[ "$built_host" == "1" ]]; then
    rm -f -- "$haco_host"
  fi
  if [[ "$built_controller" == "1" ]]; then
    rm -f -- "$controller"
  fi
  rm -rf "$root"
}
trap cleanup EXIT

if [[ -z "${HACO_E2E_HACO_BIN:-}" ]]; then
  go build -o "$haco" ./cmd/haco
fi
[[ -x "$haco" ]]

# `haco host ensure` provisions the logical Host client from beside the main
# executable. `haco host shell` is controller-routed, so reproduce the actual
# Physical Host topology rather than falling back to direct Incus access.
if [[ ! -x "$haco_host" ]]; then
  go build -o "$haco_host" ./cmd/haco-host
  built_host=1
fi
if [[ ! -x "$controller" ]]; then
  go build -o "$controller" ./cmd/haco-controller
  built_controller=1
fi
[[ -x "$haco_host" && -x "$controller" ]]

"$controller" >"$controller_log" 2>&1 &
controller_pid=$!
for _ in $(seq 1 100); do
  [[ -S "$control_socket" ]] && break
  if ! kill -0 "$controller_pid" >/dev/null 2>&1; then
    cat "$controller_log" >&2 || true
    echo 'haco-controller exited before creating its Unix socket' >&2
    exit 1
  fi
  sleep 0.05
done
[[ -S "$control_socket" ]] || {
  cat "$controller_log" >&2 || true
  echo "haco-controller did not create $control_socket" >&2
  exit 1
}

"$haco" host ensure
created=1
state="$(incus list haco-host --project hacocoon --format csv -c s)"
[[ "$state" == "RUNNING" ]]
marker="$(incus config get haco-host user.hacocoon.role --project hacocoon)"
[[ "$marker" == "trusted-host" ]]
[[ "$(incus config get haco-host environment.HACO_CLIENT_MODE --project hacocoon)" == "controller" ]]
[[ "$(incus config get haco-host environment.HACO_CONTROL_SOCKET --project hacocoon)" == "/var/lib/hacocoon-control.sock" ]]

# The trusted Host gets only a projected controller endpoint, never the Incus
# daemon socket itself.
[[ "$(incus config device get haco-host haco-control type --project hacocoon)" == "proxy" ]]
[[ "$(incus config device get haco-host haco-control listen --project hacocoon)" == "unix:/var/lib/hacocoon-control.sock" ]]
[[ "$(incus config device get haco-host haco-control connect --project hacocoon)" == "unix:$control_socket" ]]

# Reconciliation is intentionally idempotent.
"$haco" host ensure
[[ "$(incus list haco-host --project hacocoon --format csv -c s)" == "RUNNING" ]]

# A persistent trusted Host that was stopped must be recovered rather than
# replaced or left unavailable, while preserving controller-client mode.
incus stop haco-host --project hacocoon
[[ "$(incus list haco-host --project hacocoon --format csv -c s)" == "STOPPED" ]]
"$haco" host ensure
[[ "$(incus list haco-host --project hacocoon --format csv -c s)" == "RUNNING" ]]
[[ "$(incus config get haco-host user.hacocoon.role --project hacocoon)" == "trusted-host" ]]
[[ "$(incus config get haco-host environment.HACO_CLIENT_MODE --project hacocoon)" == "controller" ]]

# The trusted Host is privileged, but it must not gain Incus daemon authority
# through a mounted control socket.
incus config show haco-host --expanded --project hacocoon >"$root/host-config"
if grep -Fq '/var/lib/incus/unix.socket' "$root/host-config" || grep -Fq '/var/lib/incus/unix.socket.user' "$root/host-config"; then
  echo 'trusted host unexpectedly exposes Incus control authority' >&2
  exit 1
fi

# Exercise the production controller streaming path end-to-end. The sentinel
# proves stdin/stdout traverse haco -> controller -> trusted haco-host.
printf 'echo host-shell-controller-ok\nexit\n' | LC_ALL=C "$haco" host shell >"$root/shell.out" 2>"$root/shell.err"
grep -Fq 'host-shell-controller-ok' "$root/shell.out" || {
  echo 'haco host shell did not round-trip through the controller' >&2
  cat "$root/shell.out" >&2 || true
  cat "$root/shell.err" >&2 || true
  exit 1
}
grep -Fq 'haco-host' "$root/shell.err" || {
  echo 'haco host shell did not emit the privileged Host warning' >&2
  cat "$root/shell.err" >&2 || true
  exit 1
}

incus delete haco-host --project hacocoon --force
created=0
if incus list haco-host --project hacocoon --format csv -c n | grep -Fx haco-host >/dev/null 2>&1; then
  echo 'trusted host remained after E2E cleanup' >&2
  exit 1
fi

echo 'PASS: haco host ensure/shell -> controller -> real Incus E2E'

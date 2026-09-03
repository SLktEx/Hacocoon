#!/usr/bin/env bash
set -euo pipefail

if [[ "${HACO_E2E_INCUS:-}" != "1" ]]; then
  echo "SKIP: set HACO_E2E_INCUS=1 on a supported Incus host"
  exit 0
fi

for command in go incus mktemp python3; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

incus version >/dev/null

root="$(mktemp -d)"
workspace="$root/workspace"
haco="$root/haco"
haco_host="$root/haco-host"
controller="$root/haco-controller"
controller_log="$root/controller.log"
control_socket="$root/control.sock"
environment="pty-e2e-$$"
runtime_ref="haco-$environment"
trusted_host_ref="haco-host"
export HACO_ROOT="$root/haco-root"
export HACO_CONTROL_SOCKET="$control_socket"
created=0
trusted_host_created=0
controller_pid=""

cleanup() {
  set +e
  if [[ "$created" == "1" ]]; then
    "$haco" env delete "$environment" >/dev/null 2>&1 || \
      incus delete "$runtime_ref" --project hacocoon --force >/dev/null 2>&1 || true
  fi
  if [[ "$trusted_host_created" == "1" ]]; then
    marker="$(incus config get "$trusted_host_ref" user.hacocoon.role --project hacocoon 2>/dev/null || true)"
    if [[ "$marker" == "trusted-host" ]]; then
      incus delete "$trusted_host_ref" --project hacocoon --force >/dev/null 2>&1 || true
    fi
  fi
  if [[ -n "$controller_pid" ]]; then
    kill "$controller_pid" >/dev/null 2>&1 || true
    wait "$controller_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

mkdir -p "$workspace"
printf 'pty-e2e\n' >"$workspace/probe.txt"

go build -o "$haco" ./cmd/haco
go build -o "$haco_host" ./cmd/haco-host
go build -o "$controller" ./cmd/haco-controller

"$controller" >"$controller_log" 2>&1 &
controller_pid=$!
for _ in $(seq 1 100); do
  [[ -S "$control_socket" ]] && break
  if ! kill -0 "$controller_pid" >/dev/null 2>&1; then
    cat "$controller_log" >&2 || true
    echo "haco-controller exited before creating its Unix socket" >&2
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
trusted_host_created=1
"$haco" env create --workspace "$workspace" "$environment" >/dev/null
created=1

python3 test/e2e/interactive_pty.py \
  --prompt '[HACO-HOST]' \
  -- "$haco" host shell

python3 test/e2e/interactive_pty.py \
  --prompt "[HACO-ENV:$environment]" \
  -- "$haco" env shell "$environment"

# This path deliberately adds an outer Incus PTY. The immediate-Up assertion in
# interactive_pty.py still detects a canonical TTY inside haco-host: if the
# trusted-host client does not enter raw mode, readline cannot redraw the remote
# Environment command before Enter is sent.
python3 test/e2e/interactive_pty.py \
  --prompt "[HACO-ENV:$environment]" \
  -- incus exec "$trusted_host_ref" --project hacocoon --force-interactive -- \
     /usr/local/bin/haco-host env shell "$environment"

# Cooperative termination must restore the caller TTY rather than leaving it in
# raw/no-echo mode. These exercise the shared bridge used by both first-class
# Physical Host shell entry points.
python3 test/e2e/interactive_pty.py \
  --terminate \
  --prompt '[HACO-HOST]' \
  -- "$haco" host shell

python3 test/e2e/interactive_pty.py \
  --terminate \
  --prompt "[HACO-ENV:$environment]" \
  -- "$haco" env shell "$environment"

echo "PASS: real PTY history, Ctrl-C, exit, and terminal restoration across Hacocoon shell paths"

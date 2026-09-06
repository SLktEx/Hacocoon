#!/usr/bin/env bash
set -euo pipefail

for command in go grep mktemp sleep; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

source "$(dirname "$0")/controller.sh"

root="$(mktemp -d)"
notify_pid=""
cleanup() {
  set +e
  haco_stop_test_controller
  if [[ -n "$notify_pid" ]] && kill -0 "$notify_pid" >/dev/null 2>&1; then
    kill -TERM "$notify_pid" >/dev/null 2>&1 || true
    wait "$notify_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

bin="$root/bin"
mkdir -p "$bin" "$root/home" "$root/haco-root"
export HOME="$root/home"
export HACO_ROOT="$root/haco-root"
unset WSL_DISTRO_NAME || true

go build -o "$bin/haco" ./cmd/haco-product
go build -o "$bin/hacoq" ./cmd/haco
for name in haco-controller haco-vscode haco-agent-host haco-notify; do
  go build -o "$bin/$name" "./cmd/$name"
done
for name in haco hacoq haco-controller haco-vscode haco-agent-host haco-notify; do
  test -x "$bin/$name"
done

# Product identity/help must be available before any Incus/runtime/controller
# initialization. The new haco deliberately exposes no legacy namespaces yet.
"$bin/haco" --version >"$root/haco-version-short.out" 2>"$root/haco-version-short.err"
grep -Eq '^haco dev \(checkpoint v0\.[0-9]+, commit [^)]+\)$' "$root/haco-version-short.out"
[[ ! -s "$root/haco-version-short.err" ]]

"$bin/haco" version --json >"$root/haco-version.json" 2>"$root/haco-version-json.err"
grep -Eq '"checkpoint":"v0\.[0-9]+"' "$root/haco-version.json"
grep -Fq '"version":"dev"' "$root/haco-version.json"
grep -Fq '"commit":' "$root/haco-version.json"
grep -Fq '"build_date":"unknown"' "$root/haco-version.json"
[[ ! -s "$root/haco-version-json.err" ]]

"$bin/haco" help >"$root/haco-help.out" 2>"$root/haco-help.err"
grep -Fq 'Usage:' "$root/haco-help.out"
grep -Fq 'version' "$root/haco-help.out"
[[ ! -s "$root/haco-help.err" ]]

set +e
"$bin/haco" env >"$root/haco-env.out" 2>"$root/haco-env.err"
haco_env_code=$?
set -e
[[ "$haco_env_code" == "2" ]]
[[ ! -s "$root/haco-env.out" ]]
grep -Fq 'Usage: haco env create' "$root/haco-env.err"

set +e
"$bin/haco" definitely-not-a-command >"$root/haco-invalid.out" 2>"$root/haco-invalid.err"
haco_invalid_code=$?
set -e
[[ "$haco_invalid_code" == "2" ]]
[[ ! -s "$root/haco-invalid.out" ]]
grep -Fq 'command "definitely-not-a-command" is not available yet' "$root/haco-invalid.err"

# Existing controller-backed functionality stays reachable only through the
# temporary migration CLI. This is compatibility coverage, not a product API.
haco_start_test_controller \
  "$bin/haco-controller" \
  "$root/control.sock" \
  "$root/controller.out" \
  "$root/controller.err"

# Product development commands use the same controller without legacy fallback.
"$bin/haco" env list >"$root/product-env-list.out"
grep -Fq '[' "$root/product-env-list.out"
"$bin/haco" git pending >"$root/product-git-pending.out"
grep -Fq '[' "$root/product-git-pending.out"
set +e
HACO_ROOT="$root/product-missing-root" HACO_CONTROL_SOCKET="$root/missing-product.sock" \
  "$bin/haco" env list >"$root/product-missing.out" 2>"$root/product-missing.err"
product_missing_code=$?
set -e
[[ "$product_missing_code" == "1" ]]
[[ ! -e "$root/product-missing-root/state" ]]

"$bin/hacoq" base list >"$root/hacoq-base.out" 2>"$root/hacoq-base.err"
grep -Fxq 'haco/ubuntu-24.04' "$root/hacoq-base.out"
grep -Fxq 'haco/ubuntu-26.04' "$root/hacoq-base.out"
[[ ! -s "$root/hacoq-base.err" ]]

# Legacy controller-client mode must still fail closed rather than initialize
# local state while the migration surface exists.
client_mode_root="$root/client-mode-root"
missing_control="$root/missing-control.sock"
set +e
HACO_ROOT="$client_mode_root" HACO_CONTROL_SOCKET="$missing_control" \
  "$bin/hacoq" env list >"$root/env-client.out" 2>"$root/env-client.err"
env_client_code=$?
set -e
[[ "$env_client_code" == "1" ]]
[[ ! -s "$root/env-client.out" ]]
[[ ! -e "$client_mode_root/state" ]]

set +e
HACO_ROOT="$client_mode_root" HACO_CLIENT_MODE=controller HACO_CONTROL_SOCKET="$missing_control" \
  "$bin/hacoq" base list >"$root/client-mode.out" 2>"$root/client-mode.err"
client_mode_code=$?
set -e
[[ "$client_mode_code" == "1" ]]
[[ ! -s "$root/client-mode.out" ]]
grep -Fq 'control endpoint unavailable' "$root/client-mode.err"
[[ ! -e "$client_mode_root/state" ]]

set +e
"$bin/hacoq" definitely-not-a-command >"$root/hacoq-invalid.out" 2>"$root/hacoq-invalid.err"
hacoq_invalid_code=$?
set -e
[[ "$hacoq_invalid_code" == "1" ]]
[[ ! -s "$root/hacoq-invalid.out" ]]
grep -Fq 'unknown command "definitely-not-a-command"' "$root/hacoq-invalid.err"

# Agent Host: release is intentionally idempotent, so a never-created session
# gives us a deterministic successful process-level path without real Incus.
"$bin/haco-agent-host" release --session e2e-never-created >"$root/agent.out" 2>"$root/agent.err"
grep -Fq 'released: haco-agent-' "$root/agent.out"
[[ ! -s "$root/agent.err" ]]

# VS Code adapter: exercise the shipped process entrypoint and its usage/error
# routing. Real environment/SSH behavior stays in the Incus/client E2E layers.
set +e
"$bin/haco-vscode" definitely-not-a-command >"$root/vscode.out" 2>"$root/vscode.err"
vscode_code=$?
set -e
[[ "$vscode_code" == "1" ]]
[[ ! -s "$root/vscode.out" ]]
grep -Fq 'usage: haco-vscode <open|delete>' "$root/vscode.err"
grep -Fq 'unknown command "definitely-not-a-command"' "$root/vscode.err"

# Browser notifier: prove the real server process can start and terminate
# cleanly on SIGTERM without requiring a desktop session.
"$bin/haco-notify" web --listen 127.0.0.1:0 >"$root/notify.out" 2>"$root/notify.err" &
notify_pid=$!
notify_ready=0
for ((attempt = 0; attempt < 50; attempt++)); do
  if grep -Fq 'Hacocoon browser notifications: http://127.0.0.1:0/' "$root/notify.out"; then
    notify_ready=1
    break
  fi
  if ! kill -0 "$notify_pid" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done
[[ "$notify_ready" == "1" ]] || {
  echo 'haco-notify did not report a ready browser listener' >&2
  cat "$root/notify.err" >&2 || true
  exit 1
}
kill -TERM "$notify_pid"
wait "$notify_pid"
notify_pid=""
[[ ! -s "$root/notify.err" ]]

echo 'PASS: new haco product CLI + temporary hacoq compatibility black-box E2E'

#!/usr/bin/env bash
set -euo pipefail

for command in go grep mktemp sleep; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

root="$(mktemp -d)"
notify_pid=""
cleanup() {
  set +e
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
export HACO_STORAGE_PRIVILEGE_MODE=direct
unset WSL_DISTRO_NAME || true

for name in haco haco-vscode haco-agent-host haco-notify haco-storage-helper; do
  go build -o "$bin/$name" "./cmd/$name"
  test -x "$bin/$name"
done

# Build identity must be available before any Incus/runtime initialization.
# This empty HACO_ROOT has no prepared local runtime state, so successful
# process-level version output proves the early identity path is standalone.
"$bin/haco" --version >"$root/haco-version-short.out" 2>"$root/haco-version-short.err"
grep -Eq '^haco dev \(checkpoint v0\.[0-9]+, commit [^)]+\)$' "$root/haco-version-short.out"
[[ ! -s "$root/haco-version-short.err" ]]

"$bin/haco" version --json >"$root/haco-version.json" 2>"$root/haco-version-json.err"
grep -Eq '"checkpoint":"v0\.[0-9]+"' "$root/haco-version.json"
grep -Fq '"version":"dev"' "$root/haco-version.json"
grep -Fq '"commit":' "$root/haco-version.json"
grep -Fq '"build_date":"unknown"' "$root/haco-version.json"
[[ ! -s "$root/haco-version-json.err" ]]

# Main CLI: prove the final executable dispatches a successful command and
# preserves the user-visible error contract for an unknown command.
"$bin/haco" base list >"$root/haco-base.out" 2>"$root/haco-base.err"
grep -Fxq 'haco/ubuntu-24.04' "$root/haco-base.out"
grep -Fxq 'haco/ubuntu-26.04' "$root/haco-base.out"
[[ ! -s "$root/haco-base.err" ]]

set +e
"$bin/haco" definitely-not-a-command >"$root/haco-invalid.out" 2>"$root/haco-invalid.err"
haco_invalid_code=$?
set -e
[[ "$haco_invalid_code" == "1" ]]
[[ ! -s "$root/haco-invalid.out" ]]
grep -Fq 'usage: haco <' "$root/haco-invalid.err"
grep -Fq 'unknown command "definitely-not-a-command"' "$root/haco-invalid.err"

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

# Privileged storage helper: invoke the real binary and prove it fails closed.
# As a normal user it must reject the caller before parsing operations; when
# already root, the same empty request must reach the allowlisted usage guard.
set +e
"$bin/haco-storage-helper" >"$root/storage.out" 2>"$root/storage.err"
storage_code=$?
set -e
[[ "$storage_code" == "125" ]]
[[ ! -s "$root/storage.out" ]]
if [[ "$(id -u)" == "0" ]]; then
  grep -Fq 'usage: haco-storage-helper --root <haco-root> <operation> [arguments]' "$root/storage.err"
else
  grep -Fq 'storage helper must run with effective uid 0' "$root/storage.err"
fi

if command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then
  set +e
  sudo -n -- "$bin/haco-storage-helper" >"$root/storage-root.out" 2>"$root/storage-root.err"
  storage_root_code=$?
  set -e
  [[ "$storage_root_code" == "125" ]]
  [[ ! -s "$root/storage-root.out" ]]
  grep -Fq 'usage: haco-storage-helper --root <haco-root> <operation> [arguments]' "$root/storage-root.err"
fi

echo 'PASS: Hacocoon shipped command black-box E2E'

#!/usr/bin/env bash
set -euo pipefail

for command in go grep mktemp script sleep; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

root="$(mktemp -d)"
notify_pid=""
controller_pid=""
cleanup() {
  set +e
  if [[ -n "$controller_pid" ]] && kill -0 "$controller_pid" >/dev/null 2>&1; then
    kill -TERM "$controller_pid" >/dev/null 2>&1 || true
    wait "$controller_pid" >/dev/null 2>&1 || true
  fi
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

for name in haco haco-controller haco-host haco-vscode haco-agent-host haco-notify haco-storage-helper; do
  go build -o "$bin/$name" "./cmd/$name"
  test -x "$bin/$name"
done
ln -s "$bin/haco" "$bin/hacocoon-login"

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

# Controller + logical Host: exercise the newly shipped Unix-socket control
# plane as two real processes. Listing an empty Environment state is a useful
# success path that does not need privileged Incus.
export HACO_CONTROL_SOCKET="$root/control.sock"
"$bin/haco-controller" >"$root/controller.out" 2>"$root/controller.err" &
controller_pid=$!
controller_ready=0
for ((attempt = 0; attempt < 50; attempt++)); do
  if [[ -S "$HACO_CONTROL_SOCKET" ]]; then
    controller_ready=1
    break
  fi
  if ! kill -0 "$controller_pid" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done
[[ "$controller_ready" == "1" ]] || {
  echo 'haco-controller did not create its Unix socket' >&2
  cat "$root/controller.err" >&2 || true
  exit 1
}
[[ "$(stat -c '%a' "$HACO_CONTROL_SOCKET")" == "600" ]]
"$bin/haco-host" doctor >"$root/host-doctor.out" 2>"$root/host-doctor.err"
grep -Fq 'Hacocoon logical Host client' "$root/host-doctor.out"
grep -Fq "$HACO_CONTROL_SOCKET" "$root/host-doctor.out"
host_list_json="$("$bin/haco-host" env list --json)"
[[ "$host_list_json" == '[]' ]]
kill -TERM "$controller_pid"
wait "$controller_pid"
controller_pid=""
[[ ! -e "$HACO_CONTROL_SOCKET" ]]
unset HACO_CONTROL_SOCKET

# Doctor: both the healthy and unavailable runtime contracts are process-level
# behavior. A PATH-local Incus shim keeps these fast and deterministic.
mkdir -p "$root/doctor-ok" "$root/doctor-down"
cat >"$root/doctor-ok/incus" <<'SH'
#!/bin/sh
if [ "${1:-}" = version ]; then
  printf '%s\n' '6.12-e2e'
  exit 0
fi
exit 2
SH
cat >"$root/doctor-down/incus" <<'SH'
#!/bin/sh
exit 127
SH
chmod +x "$root/doctor-ok/incus" "$root/doctor-down/incus"
PATH="$root/doctor-ok:$PATH" "$bin/haco" doctor >"$root/doctor-ok.out" 2>"$root/doctor-ok.err"
grep -Fq 'Hacocoon Secure Workspace Runtime' "$root/doctor-ok.out"
grep -Fq 'Incus available: true' "$root/doctor-ok.out"
grep -Fq '6.12-e2e' "$root/doctor-ok.out"
set +e
PATH="$root/doctor-down:$PATH" "$bin/haco" doctor >"$root/doctor-down.out" 2>"$root/doctor-down.err"
doctor_down_code=$?
set -e
[[ "$doctor_down_code" != "0" ]]
grep -Fq 'Incus available: false' "$root/doctor-down.out"
grep -Fq 'runtime unavailable' "$root/doctor-down.err"

# Dedicated WSL login entrypoint: explicit commands and non-interactive stdin
# must stay on the Physical Host even though argv[0] is hacocoon-login.
explicit_output="$("$bin/hacocoon-login" -lc 'printf physical-host-explicit')"
[[ "$explicit_output" == 'physical-host-explicit' ]]
stdin_output="$(printf 'printf physical-host-stdin\n' | "$bin/hacocoon-login")"
[[ "$stdin_output" == 'physical-host-stdin' ]]

# Interactive login must dispatch through passwordless sudo to the privileged
# haco-host shell. A pseudo-terminal makes the real argv[0]/stdio branch run;
# the local sudo shim records the exact handoff without requiring real Incus.
login_bin="$root/login-bin"
mkdir -p "$login_bin"
cat >"$login_bin/sudo" <<'SH'
#!/bin/sh
printf '%s\n' "$*" >"$HACO_LOGIN_CAPTURE"
SH
chmod +x "$login_bin/sudo"
PATH="$login_bin:$PATH" HACO_LOGIN_CAPTURE="$root/login.capture" \
  script -q -e -c "$bin/hacocoon-login" /dev/null >/dev/null
[[ -s "$root/login.capture" ]]
grep -Fxq -- "-n $bin/haco host shell" "$root/login.capture"

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

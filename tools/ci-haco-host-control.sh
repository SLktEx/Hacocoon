#!/usr/bin/env bash
set -euo pipefail

readonly HELPER_PATH="/usr/local/libexec/hacocoon/haco-storage-helper"
readonly ROOT="${RUNNER_TEMP:-}/haco-host-control-e2e"
readonly WORKSPACE="${RUNNER_TEMP:-}/haco-host-control-workspace"
readonly HACO_BIN="$ROOT/bin/haco"
readonly CONTROLLER_BIN="$ROOT/bin/haco-controller"
readonly HOST_CLIENT_BIN="$ROOT/bin/haco-host"
readonly CONTROL_SOCKET="$ROOT/control.sock"
readonly CONTROLLER_PID_FILE="$ROOT/controller.pid"
readonly CONTROLLER_LOG="$ROOT/controller.log"
readonly PROJECT="hacocoon"
readonly POOL="haco-local-default"
readonly TRUSTED_HOST="haco-host"
readonly ENV_NAME="control-client-e2e"
readonly ENV_INSTANCE="haco-${ENV_NAME}"
readonly MOUNTPOINT="$ROOT/mounts/local-default"
readonly BACKING="$ROOT/images/local-default.raw"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_runner() {
  [[ "${GITHUB_ACTIONS:-}" == "true" ]] || fail "trusted-host control E2E only runs inside GitHub Actions"
  [[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || fail "trusted-host control E2E requires a GitHub-hosted runner"
  [[ -n "${RUNNER_TEMP:-}" ]] || fail "trusted-host control E2E requires RUNNER_TEMP"
  [[ "$ROOT" == "${RUNNER_TEMP%/}/haco-host-control-e2e" ]] || fail "invalid runner-local Hacocoon root"
  [[ "$WORKSPACE" == "${RUNNER_TEMP%/}/haco-host-control-workspace" ]] || fail "invalid runner-local workspace"
  [[ "$(uname -s)" == "Linux" ]] || fail "trusted-host control E2E requires Linux"
  . /etc/os-release
  [[ "${ID:-}" == "ubuntu" && "${VERSION_ID:-}" == "26.04" ]] || fail "trusted-host control E2E requires Ubuntu 26.04"
}

setup() {
  require_runner
  [[ "$(id -u)" != "0" ]] || fail "acceptance setup must run as the ordinary runner user"
  [[ -x "$HELPER_PATH" ]] || fail "storage helper is not installed"
  incus version >/dev/null || fail "Incus client is not ready"

  rm -rf -- "$ROOT" "$WORKSPACE"
  mkdir -m 0700 -p "$ROOT/bin" "$WORKSPACE"
  printf 'host-visible\n' > "$WORKSPACE/host.txt"

  go build -trimpath -o "$HACO_BIN" ./cmd/haco
  go build -trimpath -o "$CONTROLLER_BIN" ./cmd/haco-controller
  go build -trimpath -o "$HOST_CLIENT_BIN" ./cmd/haco-host
  chmod 0755 "$HACO_BIN" "$CONTROLLER_BIN" "$HOST_CLIENT_BIN"
}

start_controller() {
  export HACO_ROOT="$ROOT"
  export HACO_CONTROL_SOCKET="$CONTROL_SOCKET"
  unset HACO_PLUGIN_OCI
  unset HACO_STORAGE_PRIVILEGE_MODE

  "$CONTROLLER_BIN" >"$CONTROLLER_LOG" 2>&1 &
  controller_pid=$!
  printf '%s\n' "$controller_pid" > "$CONTROLLER_PID_FILE"

  for _ in $(seq 1 100); do
    if [[ -S "$CONTROL_SOCKET" ]] && "$HOST_CLIENT_BIN" doctor >/dev/null 2>&1; then
      return 0
    fi
    if ! kill -0 "$controller_pid" 2>/dev/null; then
      cat "$CONTROLLER_LOG" >&2 || true
      fail "controller exited before becoming ready"
    fi
    sleep 0.1
  done
  cat "$CONTROLLER_LOG" >&2 || true
  fail "controller did not become ready"
}

stop_controller() {
  if [[ -f "$CONTROLLER_PID_FILE" ]]; then
    pid="$(cat "$CONTROLLER_PID_FILE" 2>/dev/null || true)"
    if [[ "$pid" =~ ^[0-9]+$ ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
    rm -f -- "$CONTROLLER_PID_FILE"
  fi
}

assert_proxy_contract() {
  [[ "$(incus config device get "$TRUSTED_HOST" haco-control listen --project "$PROJECT")" == "unix:/run/hacocoon/control.sock" ]] || fail "unexpected guest control listen socket"
  [[ "$(incus config device get "$TRUSTED_HOST" haco-control connect --project "$PROJECT")" == "unix:$CONTROL_SOCKET" ]] || fail "unexpected host control connect socket"
  [[ "$(incus config device get "$TRUSTED_HOST" haco-control bind --project "$PROJECT")" == "instance" ]] || fail "control proxy must bind inside haco-host"

  config="$ROOT/haco-host-config.yaml"
  incus config show "$TRUSTED_HOST" --expanded --project "$PROJECT" > "$config"
  ! grep -Fq '/var/lib/incus' "$config" || fail "raw Incus state leaked into haco-host config"
  ! grep -Fq '/run/incus' "$config" || fail "raw Incus socket leaked into haco-host config"
}

run_test() {
  require_runner
  [[ "$(id -u)" != "0" ]] || fail "acceptance must run as the ordinary runner user"
  [[ -x "$HACO_BIN" && -x "$CONTROLLER_BIN" && -x "$HOST_CLIENT_BIN" ]] || fail "test binaries are missing"

  start_controller
  trap stop_controller EXIT

  export HACO_TRUSTED_HOST_CLIENT_BINARY="$HOST_CLIENT_BIN"
  "$HACO_BIN" host ensure

  role="$(incus config get "$TRUSTED_HOST" user.hacocoon.role --project "$PROJECT")"
  [[ "$role" == "trusted-host" ]] || fail "trusted host ownership marker mismatch: $role"
  row="$(incus list "$TRUSTED_HOST" --project "$PROJECT" --format csv -c n,s)"
  [[ "$row" == "$TRUSTED_HOST,RUNNING" ]] || fail "trusted host did not reach RUNNING: $row"
  assert_proxy_contract

  incus exec "$TRUSTED_HOST" --project "$PROJECT" -- /usr/local/bin/haco-host doctor >/dev/null
  guest_empty="$(incus exec "$TRUSTED_HOST" --project "$PROJECT" -- /usr/local/bin/haco-host env list --json)"
  python3 - "$guest_empty" <<'PY'
import json, sys
rows = json.loads(sys.argv[1])
assert rows == [], rows
PY

  "$HACO_BIN" create --base haco/ubuntu-26.04 --workspace "$WORKSPACE" "$ENV_NAME"

  guest_status="$(incus exec "$TRUSTED_HOST" --project "$PROJECT" -- /usr/local/bin/haco-host env status "$ENV_NAME" --json)"
  python3 - "$guest_status" <<'PY'
import json, sys
row = json.loads(sys.argv[1])
assert row["environment"]["name"] == "control-client-e2e", row
assert row["state"] == "running", row
PY

  guest_read="$(incus exec "$TRUSTED_HOST" --project "$PROJECT" -- /usr/local/bin/haco-host env exec "$ENV_NAME" -- cat /workspace/host.txt)"
  [[ "$guest_read" == "host-visible" ]] || fail "guest controller exec returned '$guest_read'"

  if incus config device get "$ENV_INSTANCE" haco-control listen --project "$PROJECT" >/dev/null 2>&1; then
    fail "ordinary Environment received trusted Host control device"
  fi
  env_config="$ROOT/environment-config.yaml"
  incus config show "$ENV_INSTANCE" --expanded --project "$PROJECT" > "$env_config"
  ! grep -Fq "$CONTROL_SOCKET" "$env_config" || fail "ordinary Environment references Physical Host control socket"
  ! grep -Fq '/run/hacocoon/control.sock' "$env_config" || fail "ordinary Environment references trusted control socket"

  incus exec "$TRUSTED_HOST" --project "$PROJECT" -- /usr/local/bin/haco-host env delete "$ENV_NAME"
  if incus list "$ENV_INSTANCE" --project "$PROJECT" --format csv -c n | grep -Fx "$ENV_INSTANCE" >/dev/null 2>&1; then
    fail "Environment remained after controller-mediated guest delete"
  fi

  stop_controller
  trap - EXIT
}

diagnostics() {
  require_runner
  set +e
  echo '::group::controller log'
  cat "$CONTROLLER_LOG"
  echo '::endgroup::'
  echo '::group::Incus instances'
  incus list --all-projects
  echo '::endgroup::'
  echo '::group::trusted host config'
  incus config show "$TRUSTED_HOST" --expanded --project "$PROJECT"
  echo '::endgroup::'
  echo '::group::managed storage'
  incus storage list --project "$PROJECT"
  incus storage show "$POOL" --project "$PROJECT"
  findmnt -rn -o SOURCE,TARGET,FSTYPE,OPTIONS --mountpoint "$MOUNTPOINT"
  sudo losetup --list --output NAME,BACK-FILE,BACK-INO
  echo '::endgroup::'
}

remove_owned_instances() {
  if incus list "$ENV_INSTANCE" --project "$PROJECT" --format csv -c n 2>/dev/null | grep -Fx "$ENV_INSTANCE" >/dev/null 2>&1; then
    incus delete "$ENV_INSTANCE" --project "$PROJECT" --force || return 1
  fi
  if incus list "$TRUSTED_HOST" --project "$PROJECT" --format csv -c n 2>/dev/null | grep -Fx "$TRUSTED_HOST" >/dev/null 2>&1; then
    role="$(incus config get "$TRUSTED_HOST" user.hacocoon.role --project "$PROJECT" 2>/dev/null || true)"
    [[ "$role" == "trusted-host" ]] || {
      echo "ERROR: refusing to delete unowned haco-host (role=$role)" >&2
      return 1
    }
    incus delete "$TRUSTED_HOST" --project "$PROJECT" --force || return 1
  fi
}

remove_owned_pool() {
  project="$1"
  incus storage show "$POOL" --project "$project" >/dev/null 2>&1 || return 0
  source="$(incus storage get "$POOL" source --project "$project" 2>/dev/null)" || return 1
  [[ "$source" == "$MOUNTPOINT" ]] || {
    echo "ERROR: refusing to delete pool with unexpected source: $source" >&2
    return 1
  }
  incus storage delete "$POOL" --project "$project"
}

cleanup() {
  require_runner
  set +e
  failed=0
  stop_controller

  if incus project show "$PROJECT" >/dev/null 2>&1; then
    remove_owned_instances || failed=1
    if [[ "$failed" == "0" ]]; then
      while IFS= read -r fingerprint; do
        [[ -n "$fingerprint" ]] || continue
        incus image delete "$fingerprint" --project "$PROJECT" || { failed=1; break; }
      done < <(incus image list --project "$PROJECT" --format csv -c f 2>/dev/null | sort -u)
    fi
    [[ "$failed" == "0" ]] && remove_owned_pool "$PROJECT" || true
    [[ "$failed" == "0" ]] && printf 'yes\n' | incus project delete "$PROJECT" --force || true
  fi

  if incus storage show "$POOL" --project default >/dev/null 2>&1; then
    remove_owned_pool default || failed=1
  fi

  if findmnt -rn --mountpoint "$MOUNTPOINT" >/dev/null 2>&1; then
    sudo -- "$HELPER_PATH" --root "$ROOT" unmount-btrfs "$MOUNTPOINT" || failed=1
  fi
  while read -r device backing; do
    [[ -n "$device" ]] || continue
    if [[ "$backing" == "$BACKING" ]]; then
      sudo -- "$HELPER_PATH" --root "$ROOT" detach-loop "$device" "$BACKING" || failed=1
    fi
  done < <(sudo losetup --list --noheadings --output NAME,BACK-FILE 2>/dev/null || true)
  [[ -e "$BACKING" ]] && sudo -- "$HELPER_PATH" --root "$ROOT" remove-backing "$BACKING" || true

  rm -rf -- "$ROOT" "$WORKSPACE"
  [[ "$failed" == "0" ]]
}

case "${1:-}" in
  setup) setup ;;
  test) run_test ;;
  diagnostics) diagnostics ;;
  cleanup) cleanup ;;
  *) echo "usage: $0 <setup|test|diagnostics|cleanup>" >&2; exit 2 ;;
esac

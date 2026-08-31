#!/usr/bin/env bash

# Shared process helper for E2E suites that exercise controller-backed `haco`
# commands. The caller owns HACO_ROOT/PATH and any fake runtime dependencies;
# this helper only starts the real haco-controller against that environment and
# waits until its Unix socket is ready.

HACO_TEST_CONTROLLER_PID=""

haco_start_test_controller() {
  local controller_bin="$1"
  local socket_path="$2"
  local stdout_path="$3"
  local stderr_path="$4"

  [[ -x "$controller_bin" ]] || {
    echo "controller binary is not executable: $controller_bin" >&2
    return 1
  }
  [[ -n "$socket_path" ]] || {
    echo "controller socket path is required" >&2
    return 1
  }
  [[ -z "${HACO_TEST_CONTROLLER_PID:-}" ]] || {
    echo "test controller is already running" >&2
    return 1
  }

  export HACO_CONTROL_SOCKET="$socket_path"
  rm -f -- "$socket_path"
  "$controller_bin" >"$stdout_path" 2>"$stderr_path" &
  HACO_TEST_CONTROLLER_PID=$!

  local attempt
  for ((attempt = 0; attempt < 100; attempt++)); do
    if [[ -S "$socket_path" ]]; then
      return 0
    fi
    if ! kill -0 "$HACO_TEST_CONTROLLER_PID" >/dev/null 2>&1; then
      wait "$HACO_TEST_CONTROLLER_PID" >/dev/null 2>&1 || true
      HACO_TEST_CONTROLLER_PID=""
      echo "haco-controller exited before creating $socket_path" >&2
      cat "$stderr_path" >&2 || true
      return 1
    fi
    sleep 0.05
  done

  echo "haco-controller did not create $socket_path" >&2
  cat "$stderr_path" >&2 || true
  haco_stop_test_controller
  return 1
}

haco_stop_test_controller() {
  local pid="${HACO_TEST_CONTROLLER_PID:-}"
  [[ -n "$pid" ]] || return 0

  if kill -0 "$pid" >/dev/null 2>&1; then
    kill -TERM "$pid" >/dev/null 2>&1 || true
  fi
  wait "$pid" >/dev/null 2>&1 || true
  HACO_TEST_CONTROLLER_PID=""
}

#!/usr/bin/env bash
set -euo pipefail

haco_bin="${HACO_BIN:-haco}"
workspace="${HACO_E2E_WORKSPACE:-$(mktemp -d)}"
environment="${HACO_E2E_ENVIRONMENT:-installer-e2e-${RANDOM}-$$}"
environment_created=0

fail() {
  printf 'installer user journey: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [[ "$environment_created" == "1" ]]; then
    "$haco_bin" env delete "$environment" >/dev/null 2>&1 || true
  fi
  rm -rf "$workspace"
}
trap cleanup EXIT

command -v "$haco_bin" >/dev/null 2>&1 || fail "haco is unavailable"
mkdir -p "$workspace"
printf 'from-host\n' > "$workspace/input.txt"

printf '==> User journey: controller and base catalog\n'
base_output="$($haco_bin base list)"
printf '%s\n' "$base_output"
printf '%s\n' "$base_output" | grep -Fxq 'haco/ubuntu-26.04' || fail "Ubuntu 26.04 base is unavailable"

printf '==> User journey: ephemeral haco run\n'
run_output="$($haco_bin run --workspace "$workspace" -- sh -eu -c '
  test "$(cat /workspace/input.txt)" = from-host
  printf "run-ok\\n"
  printf "from-run\\n" > /workspace/run-output.txt
')"
printf '%s\n' "$run_output"
printf '%s\n' "$run_output" | grep -qx 'run-ok' || fail "haco run did not return expected output"
grep -qx 'from-run' "$workspace/run-output.txt" || fail "haco run did not persist workspace output to the host"

printf '==> User journey: named environment lifecycle\n'
create_output="$($haco_bin env create --workspace "$workspace" "$environment")"
printf '%s\n' "$create_output"
environment_created=1
printf '%s\n' "$create_output" | grep -Fq "$environment" || fail "haco env create did not report the environment name"

status_output="$($haco_bin env status "$environment")"
printf '%s\n' "$status_output"
printf '%s\n' "$status_output" | grep -Fxq "name: $environment" || fail "haco env status did not report the environment"

exec_output="$($haco_bin env exec "$environment" -- sh -eu -c '
  test "$(cat /workspace/input.txt)" = from-host
  printf "exec-ok\\n"
  printf "from-exec\\n" > /workspace/exec-output.txt
')"
printf '%s\n' "$exec_output"
printf '%s\n' "$exec_output" | grep -qx 'exec-ok' || fail "haco env exec did not return expected output"
grep -qx 'from-exec' "$workspace/exec-output.txt" || fail "haco env exec did not persist workspace output to the host"

$haco_bin env delete "$environment"
environment_created=0
if $haco_bin env status "$environment" >/dev/null 2>&1; then
  fail "deleted environment is still addressable"
fi

printf 'installer user journey: PASS\n'

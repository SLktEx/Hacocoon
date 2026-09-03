#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/test/e2e/controller.sh"
trap haco_stop_test_controller EXIT

readonly CLI_ROOT="${RUNNER_TEMP:-}/haco-incus-storage-cli-e2e"
readonly WORKSPACE="${RUNNER_TEMP:-}/haco-incus-storage-workspace"
readonly RUN_WORKSPACE="${RUNNER_TEMP:-}/haco-incus-storage-run-workspace"
readonly HACO_BIN="${RUNNER_TEMP:-}/haco-incus-storage-cli-bin"
readonly CONTROLLER_BIN="${RUNNER_TEMP:-}/haco-incus-storage-controller-bin"
readonly PROJECT="hacocoon"
readonly POOL="haco-local-default"
readonly ENV_NAME="incus-storage-cli-e2e"
readonly INSTANCE="haco-${ENV_NAME}"
readonly INCUS_POOL_MOUNT="/var/lib/incus/storage-pools/${POOL}"
readonly INCUS_BACKING="/var/lib/incus/disks/${POOL}.img"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_github_hosted_runner() {
  [[ "${GITHUB_ACTIONS:-}" == "true" ]] || fail "Incus storage CLI E2E only runs inside GitHub Actions"
  [[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || fail "Incus storage CLI E2E requires a GitHub-hosted runner"
  [[ -n "${RUNNER_TEMP:-}" ]] || fail "Incus storage CLI E2E requires RUNNER_TEMP"
  [[ "$(uname -s)" == "Linux" ]] || fail "Incus storage CLI E2E requires Linux"

  . /etc/os-release
  [[ "${ID:-}" == "ubuntu" ]] || fail "Incus storage CLI E2E requires Ubuntu"
  [[ "${VERSION_ID:-}" == "26.04" ]] || fail "Incus storage CLI E2E requires Ubuntu 26.04, got ${VERSION_ID:-unknown}"
}

setup() {
  require_github_hosted_runner
  [[ "$(id -u)" != "0" ]] || fail "storage CLI acceptance must run as the ordinary runner user"
  incus version >/dev/null || fail "Incus client is not ready"

  rm -rf -- "$CLI_ROOT" "$WORKSPACE" "$RUN_WORKSPACE"
  rm -f -- "$HACO_BIN" "$CONTROLLER_BIN"
  mkdir -m 0700 "$CLI_ROOT" "$WORKSPACE" "$RUN_WORKSPACE"
  printf 'host-visible\n' > "$WORKSPACE/host.txt"

  go build -trimpath -o "$HACO_BIN" ./cmd/haco
  go build -trimpath -o "$CONTROLLER_BIN" ./cmd/haco-controller
  [[ -x "$HACO_BIN" ]] || fail "haco CLI build failed"
  [[ -x "$CONTROLLER_BIN" ]] || fail "haco-controller build failed"
}

assert_incus_managed_storage() {
  local source configured_size mount_options fstype live_options logical_bytes allocated_bytes

  source="$(incus storage get "$POOL" source --project "$PROJECT")"
  [[ "$source" == "$INCUS_BACKING" ]] || fail "Incus pool source is '$source', expected '$INCUS_BACKING'"

  configured_size="$(incus storage get "$POOL" size --project "$PROJECT")"
  [[ "$configured_size" == "128GiB" ]] || fail "Incus pool size is '$configured_size', expected 128GiB"

  mount_options="$(incus storage get "$POOL" btrfs.mount_options --project "$PROJECT")"
  [[ "$mount_options" == "compress=zstd:3" ]] || fail "Incus Btrfs mount options are '$mount_options'"
  [[ ",$mount_options," != *,autodefrag,* ]] || fail "autodefrag must remain disabled: $mount_options"

  [[ ! -e "$CLI_ROOT/images/local-default.raw" ]] || fail "default composition still created the legacy Hacocoon raw image"
  [[ ! -e "$CLI_ROOT/mounts/local-default" ]] || fail "default composition still created the legacy Hacocoon mountpoint"

  sudo test -f "$source" || fail "Incus loop backing image is missing"
  logical_bytes="$(sudo stat -Lc '%s' "$source")"
  allocated_bytes="$(( $(sudo stat -Lc '%b' "$source") * 512 ))"
  [[ "$logical_bytes" == "$((128 * 1024 * 1024 * 1024))" ]] || fail "loop image logical size is $logical_bytes"
  [[ "$allocated_bytes" -lt "$logical_bytes" ]] || fail "Incus loop image is not sparse: allocated=$allocated_bytes logical=$logical_bytes"

  sudo losetup --list --noheadings --output NAME,BACK-FILE | awk -v backing="$source" '
    $2 == backing && $1 ~ /^\/dev\/loop[0-9]+$/ { found = 1 }
    END { exit found ? 0 : 1 }
  ' || fail "Incus sparse backing image is not attached to a loop device"

  fstype="$(sudo findmnt -rn -o FSTYPE --mountpoint "$INCUS_POOL_MOUNT")"
  [[ "$fstype" == "btrfs" ]] || fail "Incus pool mount filesystem is '$fstype', expected btrfs"
  live_options="$(sudo findmnt -rn -o OPTIONS --mountpoint "$INCUS_POOL_MOUNT")"
  [[ ",$live_options," == *,compress=zstd:3,* || ",$live_options," == *,compress=zstd,* ]] || fail "Incus pool mount is missing zstd compression: $live_options"
  [[ ",$live_options," != *,autodefrag,* ]] || fail "live Incus Btrfs mount unexpectedly enables autodefrag: $live_options"
}

assert_instance_boundary() {
  local config_file="$CLI_ROOT/incus-config"
  incus config show "$INSTANCE" --expanded --project "$PROJECT" >"$config_file"
  grep -Fq "$WORKSPACE" "$config_file" || fail "requested workspace is not mounted in Incus config"

  local forbidden
  for forbidden in \
    "${HOME:-}/.ssh" \
    "${HOME:-}/.aws" \
    "${HOME:-}/.config/gh" \
    "/var/lib/incus/unix.socket" \
    "/var/lib/incus/unix.socket.user"; do
    [[ "$forbidden" == "/.ssh" || "$forbidden" == "/.aws" || "$forbidden" == "/.config/gh" ]] && continue
    if grep -Fq "$forbidden" "$config_file"; then
      fail "unexpected credential/authority exposure in Incus config: $forbidden"
    fi
  done
}

run_test() {
  require_github_hosted_runner
  [[ "$(id -u)" != "0" ]] || fail "storage CLI acceptance must execute haco as the ordinary runner user"

  export HACO_ROOT="$CLI_ROOT"
  unset HACO_PLUGIN_OCI
  unset HACO_STORAGE_PRIVILEGE_MODE
  unset HACO_BLOCK_BACKEND

  "$HACO_BIN" create --base haco/ubuntu-26.04 --workspace "$WORKSPACE" "$ENV_NAME"

  status_json="$("$HACO_BIN" status "$ENV_NAME" --json)"
  python3 - "$status_json" <<'PY'
import json, sys
row = json.loads(sys.argv[1])
env = row["environment"]
assert env["name"] == "incus-storage-cli-e2e", row
assert row["state"] == "running", row
PY

  instance_row="$(incus list "$INSTANCE" --project "$PROJECT" --format csv -c n,s)"
  [[ "$instance_row" == "$INSTANCE,RUNNING" ]] || fail "real Incus instance did not reach RUNNING: $instance_row"
  incus storage show "$POOL" --project "$PROJECT" >/dev/null
  assert_incus_managed_storage
  assert_instance_boundary

  read_back="$("$HACO_BIN" exec "$ENV_NAME" -- cat /workspace/host.txt)"
  [[ "$read_back" == "host-visible" ]] || fail "workspace host->environment read mismatch: $read_back"
  "$HACO_BIN" exec "$ENV_NAME" -- sh -c 'test -w /workspace && printf "from-environment\n" > /workspace/from-environment.txt'
  [[ "$(cat "$WORKSPACE/from-environment.txt")" == "from-environment" ]] || fail "Environment did not write through the real workspace mount"

  "$HACO_BIN" delete "$ENV_NAME"
  if incus list "$INSTANCE" --project "$PROJECT" --format csv -c n | grep -Fx "$INSTANCE" >/dev/null 2>&1; then
    fail "named Environment instance remained after haco delete"
  fi

  haco_start_test_controller \
    "$CONTROLLER_BIN" \
    "$CLI_ROOT/control.sock" \
    "$CLI_ROOT/controller.out" \
    "$CLI_ROOT/controller.err"

  run_json="$("$HACO_BIN" run --workspace "$RUN_WORKSPACE" --json -- sh -c 'printf "run-ok\n"; printf "from-run\n" > /workspace/from-run.txt')"
  python3 - "$run_json" <<'PY'
import json, sys
row = json.loads(sys.argv[1])
assert row["execution"]["exit_code"] == 0, row
assert row["execution"]["stdout"] == "run-ok\n", row
assert row["cleaned_up"] is True, row
PY
  [[ "$(cat "$RUN_WORKSPACE/from-run.txt")" == "from-run" ]] || fail "haco run did not write through the real workspace mount"
  incus storage show "$POOL" --project "$PROJECT" >/dev/null
  assert_incus_managed_storage
  haco_stop_test_controller
}

diagnostics() {
  require_github_hosted_runner
  set +e
  echo '::group::Hacocoon CLI storage root'
  find "$CLI_ROOT" -maxdepth 4 -ls
  echo '::endgroup::'
  echo '::group::Incus state'
  incus project list
  incus list --all-projects
  incus storage list --project "$PROJECT"
  incus storage show "$POOL" --project "$PROJECT"
  echo '::endgroup::'
  echo '::group::Incus Btrfs loop state'
  sudo stat "$INCUS_BACKING"
  sudo findmnt -rn -o SOURCE,TARGET,FSTYPE,OPTIONS --mountpoint "$INCUS_POOL_MOUNT"
  sudo losetup --list --output NAME,BACK-FILE,BACK-INO
  echo '::endgroup::'
}

delete_owned_instances() {
  local instance unexpected=0
  while IFS= read -r instance; do
    [[ -n "$instance" ]] || continue
    case "$instance" in
      "$INSTANCE"|haco-run-*) incus delete "$instance" --project "$PROJECT" --force || return 1 ;;
      *) echo "ERROR: refusing to delete unexpected instance '$instance'" >&2; unexpected=1 ;;
    esac
  done < <(incus list --project "$PROJECT" --format csv -c n 2>/dev/null || true)
  [[ "$unexpected" == "0" ]]
}

delete_project_images() {
  local fingerprint
  while IFS= read -r fingerprint; do
    [[ -n "$fingerprint" ]] || continue
    incus image delete "$fingerprint" --project "$PROJECT" || return 1
  done < <(incus image list --project "$PROJECT" --format csv -c f 2>/dev/null | sort -u)
}

cleanup() {
  require_github_hosted_runner
  local failed=0
  set +e
  haco_stop_test_controller

  if incus project show "$PROJECT" >/dev/null 2>&1; then
    delete_owned_instances || failed=1
    [[ "$failed" != "0" ]] || delete_project_images || failed=1
  fi

  if incus storage show "$POOL" --project "$PROJECT" >/dev/null 2>&1; then
    incus storage delete "$POOL" --project "$PROJECT" || failed=1
  elif incus storage show "$POOL" --project default >/dev/null 2>&1; then
    incus storage delete "$POOL" --project default || failed=1
  fi

  if incus project show "$PROJECT" >/dev/null 2>&1; then
    printf 'yes\n' | incus project delete "$PROJECT" --force || failed=1
  fi

  if [[ "$failed" == "0" ]]; then
    sudo test ! -e "$INCUS_BACKING" || fail "Incus backing image remained after storage delete"
    if sudo losetup --list --noheadings --output BACK-FILE | grep -Fx "$INCUS_BACKING" >/dev/null 2>&1; then
      fail "Incus loop attachment remained after storage delete"
    fi
    rm -rf -- "$CLI_ROOT" "$WORKSPACE" "$RUN_WORKSPACE"
  fi
  rm -f -- "$HACO_BIN" "$CONTROLLER_BIN"
  [[ "$failed" == "0" ]] || fail "Incus storage CLI E2E cleanup was incomplete"
}

case "${1:-}" in
  setup) setup ;;
  test) run_test ;;
  diagnostics) diagnostics ;;
  cleanup) cleanup ;;
  *) echo "usage: $0 <setup|test|diagnostics|cleanup>" >&2; exit 2 ;;
esac

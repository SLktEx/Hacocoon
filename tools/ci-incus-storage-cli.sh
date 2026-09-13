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
readonly BTRFS_MOUNT_OPTIONS="compress=zstd:3,noatime,nodiscard"

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
  go build -trimpath -o "$CLI_ROOT/haco-product" ./cmd/haco-product
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
  [[ "$mount_options" == "$BTRFS_MOUNT_OPTIONS" ]] || fail "Incus Btrfs mount options are '$mount_options', expected '$BTRFS_MOUNT_OPTIONS'"
  [[ ",$mount_options," != *,autodefrag,* ]] || fail "autodefrag must remain disabled: $mount_options"

  [[ ! -e "$CLI_ROOT/images/local-default.raw" ]] || fail "default composition still created the removed Hacocoon raw image"
  [[ ! -e "$CLI_ROOT/mounts/local-default" ]] || fail "default composition still created the removed Hacocoon mountpoint"

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
  [[ ",$live_options," == *,noatime,* ]] || fail "Incus pool mount is missing noatime: $live_options"
  # Negative defaults may be omitted from findmnt; reject every positive discard mode.
  [[ ",$live_options," != *,discard,* && ",$live_options," != *,discard=*,* ]] || fail "Incus pool mount unexpectedly enables discard: $live_options"
  [[ ",$live_options," != *,relatime,* && ",$live_options," != *,strictatime,* ]] || fail "Incus pool mount unexpectedly enables atime updates: $live_options"
  [[ ",$live_options," != *,compress-force* ]] || fail "Incus pool mount unexpectedly forces compression: $live_options"
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

  "$HACO_BIN" create --base haco/ubuntu-26.04 --workspace "$WORKSPACE" "$ENV_NAME"
  python3 tools/verify_ci_base_provenance.py "$CLI_ROOT/state/environments.json" "$ENV_NAME"

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

  "$HACO_BIN" exec "$ENV_NAME" -- sh -c 'printf "rootfs-retained\n" > /root/storage-reuse-sentinel'

  # The next rootfs operation must reconcile the existing pool through Incus.
  # Keep existing workspace data to catch destructive replacement on reuse.
  incus storage set "$POOL" btrfs.mount_options=compress=zstd:3 --project "$PROJECT"
  [[ "$(incus storage get "$POOL" btrfs.mount_options --project "$PROJECT")" == "compress=zstd:3" ]] || fail "failed to install stale mount policy"

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
  python3 tools/test_temporary_run.py "$CLI_ROOT/haco-product" "$RUN_WORKSPACE"
  incus storage show "$POOL" --project "$PROJECT" >/dev/null
  assert_incus_managed_storage
  [[ "$(cat "$WORKSPACE/from-environment.txt")" == "from-environment" ]] || fail "workspace data changed during pool reuse"
  [[ "$("$HACO_BIN" exec "$ENV_NAME" -- cat /root/storage-reuse-sentinel)" == "rootfs-retained" ]] || fail "existing rootfs data changed during policy reconciliation"
  haco_stop_test_controller
  "$HACO_BIN" delete "$ENV_NAME"
  local remaining
  remaining="$(incus list "$INSTANCE" --project "$PROJECT" --format csv -c n)" || fail "instance absence is unknown"
  if grep -Fxq -- "$INSTANCE" <<< "$remaining"; then
    fail "named Environment instance remained after hacoq delete"
  fi
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
  local instance instances
  instances="$(incus list --project "$PROJECT" --format csv -c n)" || return 1
  # Validate the entire inventory before the first deletion. Failure-retained
  # fixtures are evidence; their prefix alone does not authorize removal here.
  while IFS= read -r instance; do
    [[ -n "$instance" ]] || continue
    [[ "$instance" =~ ^haco-[a-zA-Z0-9-]+$ ]] || return 1
    case "$instance" in
      haco-base-*|"$INSTANCE"|haco-run-*) ;;
      *) echo "ERROR: refusing to delete unexpected instance '$instance'" >&2; return 1 ;;
    esac
  done <<< "$instances"
  while IFS= read -r instance; do
    [[ -n "$instance" ]] || continue
    case "$instance" in
      haco-base-*) python3 tools/cleanup_ci_base_asset.py "$instance" || return 1 ;;
      "$INSTANCE"|haco-run-*) incus delete "$instance" --project "$PROJECT" --force || return 1 ;;
    esac
  done <<< "$instances"
  instances="$(incus list --project "$PROJECT" --format csv -c n)" || return 1
  [[ -z "$instances" ]]
}

delete_project_images() {
  local fingerprint fingerprints
  fingerprints="$(incus image list --project "$PROJECT" --format csv -c F)" || return 1
  while IFS= read -r fingerprint; do
    [[ -n "$fingerprint" ]] || continue
    [[ "$fingerprint" =~ ^[a-f0-9]{64}$ ]] || return 1
  done <<< "$fingerprints"
  while IFS= read -r fingerprint; do
    [[ -n "$fingerprint" ]] || continue
    incus image delete "$fingerprint" --project "$PROJECT" || return 1
  done < <(sort -u <<< "$fingerprints")
  fingerprints="$(incus image list --project "$PROJECT" --format csv -c F)" || return 1
  [[ -z "$fingerprints" ]]
}

cleanup_inventory() {
  local observed
  CLEANUP_PROJECTS="$(incus project list --format csv -c n)" || return 1
  grep -Fxq default <<< "$CLEANUP_PROJECTS" || return 1
  CLEANUP_POOLS="$(incus storage list --format csv -c n)" || return 1
  while IFS= read -r observed; do
    [[ -n "$observed" ]] || continue
    [[ "$observed" =~ ^[a-zA-Z0-9][a-zA-Z0-9_.-]*$ ]] || return 1
  done <<< "$CLEANUP_PROJECTS"$'\n'"$CLEANUP_POOLS"
}

cleanup() {
  require_github_hosted_runner
  haco_stop_test_controller
  cleanup_inventory || fail "cleanup inventory is unknown"

  if grep -Fxq -- "$PROJECT" <<< "$CLEANUP_PROJECTS"; then
    delete_owned_instances || fail "cleanup stopped before deleting shared storage: instance ownership/absence unconfirmed"
    delete_project_images || fail "cleanup stopped: image absence unconfirmed"
  fi

  if grep -Fxq -- "$POOL" <<< "$CLEANUP_POOLS"; then
    incus storage delete "$POOL" --project default || fail "storage cleanup was incomplete"
  fi

  if grep -Fxq -- "$PROJECT" <<< "$CLEANUP_PROJECTS"; then
    incus project delete "$PROJECT" || fail "project cleanup was incomplete"
  fi

  cleanup_inventory || fail "cleanup absence is unknown"
  if grep -Fxq -- "$PROJECT" <<< "$CLEANUP_PROJECTS" || grep -Fxq -- "$POOL" <<< "$CLEANUP_POOLS"; then
    fail "Incus project or storage pool remained after cleanup"
  fi
  sudo test ! -e "$INCUS_BACKING" || fail "Incus backing image remained after storage delete"
  local backing_files
  backing_files="$(sudo losetup --list --noheadings --output BACK-FILE)" || fail "Incus loop attachment absence is unknown"
  if grep -Fxq -- "$INCUS_BACKING" <<< "$backing_files"; then
    fail "Incus loop attachment remained after storage delete"
  fi
  rm -rf -- "$CLI_ROOT" "$WORKSPACE" "$RUN_WORKSPACE"
  rm -f -- "$HACO_BIN" "$CONTROLLER_BIN"
}

case "${1:-}" in
  setup) setup ;;
  test) run_test ;;
  diagnostics) diagnostics ;;
  cleanup) cleanup ;;
  *) echo "usage: $0 <setup|test|diagnostics|cleanup>" >&2; exit 2 ;;
esac

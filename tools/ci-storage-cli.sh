#!/usr/bin/env bash
set -euo pipefail

readonly HELPER_PATH="/usr/local/libexec/hacocoon/haco-storage-helper"
readonly CLI_ROOT="${RUNNER_TEMP:-}/haco-storage-cli-e2e"
readonly WORKSPACE="${RUNNER_TEMP:-}/haco-storage-cli-workspace"
readonly RUN_WORKSPACE="${RUNNER_TEMP:-}/haco-storage-cli-run-workspace"
readonly HACO_BIN="${RUNNER_TEMP:-}/haco-storage-cli-bin"
readonly PROJECT="hacocoon"
readonly POOL="haco-local-default"
readonly ENV_NAME="storage-cli-e2e"
readonly INSTANCE="haco-${ENV_NAME}"
readonly MOUNTPOINT="${CLI_ROOT}/mounts/local-default"
readonly BACKING="${CLI_ROOT}/images/local-default.raw"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_github_hosted_runner() {
  [[ "${GITHUB_ACTIONS:-}" == "true" ]] || fail "storage CLI E2E only runs inside GitHub Actions"
  [[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || fail "storage CLI E2E requires a GitHub-hosted runner"
  [[ -n "${RUNNER_TEMP:-}" ]] || fail "storage CLI E2E requires RUNNER_TEMP"
  [[ "$CLI_ROOT" == "${RUNNER_TEMP%/}/haco-storage-cli-e2e" ]] || fail "invalid runner-local CLI storage root"
  [[ "$WORKSPACE" == "${RUNNER_TEMP%/}/haco-storage-cli-workspace" ]] || fail "invalid runner-local CLI workspace"
  [[ "$RUN_WORKSPACE" == "${RUNNER_TEMP%/}/haco-storage-cli-run-workspace" ]] || fail "invalid runner-local CLI run workspace"
  [[ "$(uname -s)" == "Linux" ]] || fail "storage CLI E2E requires Linux"

  . /etc/os-release
  [[ "${ID:-}" == "ubuntu" ]] || fail "storage CLI E2E requires Ubuntu"
  [[ "${VERSION_ID:-}" == "26.04" ]] || fail "storage CLI E2E requires Ubuntu 26.04, got ${VERSION_ID:-unknown}"
}

setup() {
  require_github_hosted_runner
  [[ "$(id -u)" != "0" ]] || fail "storage CLI acceptance must run setup as the ordinary runner user"
  [[ -x "$HELPER_PATH" ]] || fail "storage helper is not installed"
  incus version >/dev/null || fail "Incus client is not ready"

  rm -rf -- "$CLI_ROOT" "$WORKSPACE" "$RUN_WORKSPACE"
  rm -f -- "$HACO_BIN"
  mkdir -m 0700 "$CLI_ROOT" "$WORKSPACE" "$RUN_WORKSPACE"
  printf 'host-visible\n' > "$WORKSPACE/host.txt"

  go build -trimpath -o "$HACO_BIN" ./cmd/haco
  [[ -x "$HACO_BIN" ]] || fail "haco CLI build failed"
}

assert_managed_storage() {
  local source fstype options

  source="$(incus storage get "$POOL" source --project "$PROJECT")"
  [[ "$source" == "$MOUNTPOINT" ]] || fail "Incus pool source is '$source', expected '$MOUNTPOINT'"

  fstype="$(findmnt -rn -o FSTYPE --mountpoint "$MOUNTPOINT")"
  [[ "$fstype" == "btrfs" ]] || fail "managed mount filesystem is '$fstype', expected btrfs"

  options="$(findmnt -rn -o OPTIONS --mountpoint "$MOUNTPOINT")"
  [[ ",$options," == *,compress=zstd:3,* || ",$options," == *,compress=zstd,* ]] || fail "managed mount is missing zstd compression: $options"

  [[ -f "$BACKING" && ! -L "$BACKING" ]] || fail "managed sparse backing image is missing or not a regular file"
  [[ "$(stat -c '%u' "$BACKING")" == "$(id -u)" ]] || fail "managed sparse backing image is not owned by the runner user"
  [[ "$(stat -c '%h' "$BACKING")" == "1" ]] || fail "managed sparse backing image unexpectedly has hard links"

  sudo losetup --list --noheadings --output NAME,BACK-FILE,BACK-INO | awk -v backing="$BACKING" '
    $2 == backing && $1 ~ /^\/dev\/loop[0-9]+$/ && $3 ~ /^[0-9]+$/ { found = 1 }
    END { exit found ? 0 : 1 }
  ' || fail "managed sparse backing image is not attached to a real loop device with BACK-INO"
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
  [[ -x "$HACO_BIN" ]] || fail "haco CLI test binary is missing"
  [[ -x "$HELPER_PATH" ]] || fail "storage helper is missing"

  export HACO_ROOT="$CLI_ROOT"
  unset HACO_PLUGIN_OCI
  unset HACO_STORAGE_PRIVILEGE_MODE

  "$HACO_BIN" create --base haco/ubuntu-26.04 --workspace "$WORKSPACE" "$ENV_NAME"

  status_json="$("$HACO_BIN" status "$ENV_NAME" --json)"
  python3 - "$status_json" <<'PY'
import json, sys
row = json.loads(sys.argv[1])
env = row["environment"]
assert env["name"] == "storage-cli-e2e", row
assert row["state"] == "running", row
PY

  instance_row="$(incus list "$INSTANCE" --project "$PROJECT" --format csv -c n,s)"
  [[ "$instance_row" == "$INSTANCE,RUNNING" ]] || fail "real Incus instance did not reach RUNNING: $instance_row"
  incus storage show "$POOL" --project "$PROJECT" >/dev/null
  assert_managed_storage
  assert_instance_boundary

  read_back="$("$HACO_BIN" exec "$ENV_NAME" -- cat /workspace/host.txt)"
  [[ "$read_back" == "host-visible" ]] || fail "workspace host->environment read mismatch: $read_back"

  "$HACO_BIN" exec "$ENV_NAME" -- sh -c 'test -w /workspace && printf "from-environment\n" > /workspace/from-environment.txt'
  [[ "$(cat "$WORKSPACE/from-environment.txt")" == "from-environment" ]] || fail "named Environment did not write through the real workspace mount"

  stdout_file="$CLI_ROOT/exec-stdout"
  stderr_file="$CLI_ROOT/exec-stderr"
  set +e
  "$HACO_BIN" exec "$ENV_NAME" -- sh -c "printf 'stdout-ok'; printf 'stderr-ok' >&2; exit 17" >"$stdout_file" 2>"$stderr_file"
  remote_exit=$?
  set -e
  [[ "$remote_exit" == "17" ]] || fail "expected remote exit 17, got $remote_exit"
  [[ "$(cat "$stdout_file")" == "stdout-ok" ]] || fail "stdout propagation mismatch"
  grep -Fq 'stderr-ok' "$stderr_file" || fail "stderr propagation mismatch"

  printf 'exit\n' | "$HACO_BIN" shell "$ENV_NAME" >/dev/null

  "$HACO_BIN" delete "$ENV_NAME"
  if incus list "$INSTANCE" --project "$PROJECT" --format csv -c n | grep -Fx "$INSTANCE" >/dev/null 2>&1; then
    fail "named Environment instance remained after haco delete"
  fi
  if [[ -e "$HACO_ROOT/state/environments.json" ]] && grep -Fq "\"$ENV_NAME\"" "$HACO_ROOT/state/environments.json"; then
    fail "named Environment metadata remained after haco delete"
  fi

  # Reuse the already-provisioned managed pool through the normal ephemeral CLI
  # path. This is the real composition-level idempotency check for lazy storage.
  run_json="$("$HACO_BIN" run --workspace "$RUN_WORKSPACE" --json -- sh -c 'printf "run-ok\n"; printf "from-run\n" > /workspace/from-run.txt')"
  python3 - "$run_json" <<'PY'
import json, sys
row = json.loads(sys.argv[1])
assert row["execution"]["exit_code"] == 0, row
assert row["execution"]["stdout"] == "run-ok\n", row
assert row["cleaned_up"] is True, row
PY
  [[ "$(cat "$RUN_WORKSPACE/from-run.txt")" == "from-run" ]] || fail "haco run did not write through the real workspace mount"

  if incus list --project "$PROJECT" --format csv -c n | grep -E '^haco-run-' >/dev/null 2>&1; then
    fail "ephemeral haco run instance leaked"
  fi
  incus storage show "$POOL" --project "$PROJECT" >/dev/null
  assert_managed_storage
}

diagnostics() {
  require_github_hosted_runner
  set +e

  echo '::group::Hacocoon CLI storage root'
  find "$CLI_ROOT" -maxdepth 4 -ls
  echo '::endgroup::'

  echo '::group::Incus project and instances'
  incus project list
  incus list --all-projects
  echo '::endgroup::'

  echo '::group::Managed Incus storage'
  incus storage list --project "$PROJECT"
  incus storage show "$POOL" --project "$PROJECT"
  incus storage volume list "$POOL" --project "$PROJECT"
  incus image list --project "$PROJECT"
  echo '::endgroup::'

  echo '::group::Managed mount and loop'
  findmnt -rn -o SOURCE,TARGET,FSTYPE,OPTIONS --mountpoint "$MOUNTPOINT"
  sudo losetup --list --output NAME,BACK-FILE,BACK-INO
  echo '::endgroup::'
}

delete_owned_instances() {
  local instance unexpected=0

  while IFS= read -r instance; do
    [[ -n "$instance" ]] || continue
    case "$instance" in
      "$INSTANCE"|haco-run-*) incus delete "$instance" --project "$PROJECT" --force || return 1 ;;
      *)
        echo "ERROR: refusing to delete unexpected instance '$instance' from project '$PROJECT'" >&2
        unexpected=1
        ;;
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

delete_owned_pool() {
  local project="$1"
  local source

  incus storage show "$POOL" --project "$project" >/dev/null 2>&1 || return 0
  source="$(incus storage get "$POOL" source --project "$project" 2>/dev/null)" || return 1
  if [[ "$source" != "$MOUNTPOINT" ]]; then
    echo "ERROR: refusing to delete pool '$POOL' with unexpected source '$source'" >&2
    return 1
  fi
  incus storage delete "$POOL" --project "$project"
}

cleanup() {
  require_github_hosted_runner
  local device backing failed=0
  set +e

  if incus project show "$PROJECT" >/dev/null 2>&1; then
    delete_owned_instances || failed=1
    if [[ "$failed" == "0" ]]; then
      delete_project_images || failed=1
    fi
    if [[ "$failed" == "0" ]]; then
      delete_owned_pool "$PROJECT" || failed=1
    fi
    if [[ "$failed" == "0" ]]; then
      printf 'yes\n' | incus project delete "$PROJECT" --force || failed=1
    fi
  fi

  # Incus storage pools may remain visible from the default project after the
  # owning project disappears. Delete only the exact pool/source created here.
  if incus storage show "$POOL" --project default >/dev/null 2>&1; then
    delete_owned_pool default || failed=1
  fi

  if findmnt -rn --mountpoint "$MOUNTPOINT" >/dev/null 2>&1; then
    if [[ -x "$HELPER_PATH" ]]; then
      sudo -- "$HELPER_PATH" --root "$CLI_ROOT" unmount-btrfs "$MOUNTPOINT" || failed=1
    else
      echo "ERROR: storage helper missing while managed mount still exists" >&2
      failed=1
    fi
  fi

  while read -r device backing; do
    [[ -n "${device:-}" && -n "${backing:-}" ]] || continue
    [[ "$backing" == "$BACKING" ]] || continue
    if [[ ! "$device" =~ ^/dev/loop[0-9]+$ ]]; then
      echo "ERROR: refusing unexpected loop device '$device' for '$backing'" >&2
      failed=1
      continue
    fi
    if [[ -x "$HELPER_PATH" ]]; then
      sudo -- "$HELPER_PATH" --root "$CLI_ROOT" loop-detach "$device" || failed=1
    else
      echo "ERROR: storage helper missing while managed loop still exists" >&2
      failed=1
    fi
  done < <(sudo losetup --list --noheadings --output NAME,BACK-FILE 2>/dev/null || true)

  if [[ "$failed" == "0" ]]; then
    rm -rf -- "$CLI_ROOT" "$WORKSPACE" "$RUN_WORKSPACE"
  fi
  rm -f -- "$HACO_BIN"

  [[ "$failed" == "0" ]] || fail "storage CLI E2E cleanup was incomplete"
}

case "${1:-}" in
  setup) setup ;;
  test) run_test ;;
  diagnostics) diagnostics ;;
  cleanup) cleanup ;;
  *) echo "usage: $0 <setup|test|diagnostics|cleanup>" >&2; exit 2 ;;
esac

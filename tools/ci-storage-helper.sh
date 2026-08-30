#!/usr/bin/env bash
set -euo pipefail

readonly HELPER_PATH="/usr/local/libexec/hacocoon/haco-storage-helper"
readonly TEST_ROOT="${RUNNER_TEMP:-}/haco-storage-helper-e2e"
readonly BUILD_HELPER="${RUNNER_TEMP:-}/haco-storage-helper-build"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_github_hosted_runner() {
  [[ "${GITHUB_ACTIONS:-}" == "true" ]] || fail "storage helper E2E only runs inside GitHub Actions"
  [[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || fail "storage helper E2E requires a GitHub-hosted runner"
  [[ -n "${RUNNER_TEMP:-}" && "$TEST_ROOT" == "${RUNNER_TEMP%/}/haco-storage-helper-e2e" ]] || fail "invalid runner-local storage test root"
  [[ "$(uname -s)" == "Linux" ]] || fail "storage helper E2E requires Linux"

  . /etc/os-release
  [[ "${ID:-}" == "ubuntu" ]] || fail "storage helper E2E requires Ubuntu"
  [[ "${VERSION_ID:-}" == "26.04" ]] || fail "storage helper E2E requires Ubuntu 26.04, got ${VERSION_ID:-unknown}"
}

setup() {
  require_github_hosted_runner
  rm -rf -- "$TEST_ROOT"
  mkdir -m 0700 "$TEST_ROOT"

  sudo env DEBIAN_FRONTEND=noninteractive apt-get update
  sudo env DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends \
    btrfs-progs \
    util-linux

  go build -trimpath -o "$BUILD_HELPER" ./cmd/haco-storage-helper
  sudo install -d -o root -g root -m 0755 "$(dirname "$HELPER_PATH")"
  sudo install -o root -g root -m 0755 "$BUILD_HELPER" "$HELPER_PATH"

  [[ "$(stat -c '%u:%g:%a' "$HELPER_PATH")" == "0:0:755" ]] || fail "storage helper installation is not root-owned mode 0755"
}

run_test() {
  require_github_hosted_runner
  [[ "$(id -u)" != "0" ]] || fail "storage helper acceptance must execute Go tests as the ordinary runner user"
  [[ -x "$HELPER_PATH" ]] || fail "storage helper is not installed"

  export HACO_E2E_STORAGE_HELPER=1
  export HACO_E2E_STORAGE_ROOT="$TEST_ROOT"
  go test -count=1 -run '^TestRealPrivilegedStorageHelperE2E$' ./internal/storagepriv

  if findmnt -rn -o TARGET | grep -Fx "$TEST_ROOT/mounts/helper-e2e" >/dev/null 2>&1; then
    fail "managed Btrfs mount leaked after successful acceptance"
  fi
  if sudo losetup --list --noheadings --output BACK-FILE | grep -Fx "$TEST_ROOT/images/helper-e2e.raw" >/dev/null 2>&1; then
    fail "managed loop device leaked after successful acceptance"
  fi
  [[ ! -e "$TEST_ROOT/images/helper-e2e.raw" ]] || fail "managed sparse image leaked after successful acceptance"
}

diagnostics() {
  require_github_hosted_runner
  set +e
  echo '::group::storage helper identity'
  ls -ld "$(dirname "$HELPER_PATH")" "$HELPER_PATH"
  stat "$HELPER_PATH"
  echo '::endgroup::'

  echo '::group::loop devices'
  sudo losetup --list --output NAME,BACK-FILE,BACK-INO
  echo '::endgroup::'

  echo '::group::mounts under test root'
  findmnt -R "$TEST_ROOT"
  echo '::endgroup::'

  echo '::group::test root'
  find "$TEST_ROOT" -maxdepth 3 -ls
  echo '::endgroup::'
}

cleanup() {
  require_github_hosted_runner
  local target device backing failed=0
  set +e

  while IFS= read -r target; do
    [[ -n "$target" ]] || continue
    case "$target" in
      "$TEST_ROOT"/mounts/*) sudo umount -- "$target" || failed=1 ;;
    esac
  done < <(findmnt -rn -o TARGET 2>/dev/null || true)

  while read -r device backing; do
    [[ -n "${device:-}" && -n "${backing:-}" ]] || continue
    if [[ "$device" =~ ^/dev/loop[0-9]+$ && "$backing" == "$TEST_ROOT"/images/*.raw ]]; then
      sudo losetup -d "$device" || failed=1
    fi
  done < <(sudo losetup --list --noheadings --output NAME,BACK-FILE 2>/dev/null || true)

  sudo rm -f -- "$HELPER_PATH"
  rm -f -- "$BUILD_HELPER"
  rm -rf -- "$TEST_ROOT"

  [[ "$failed" == "0" ]] || fail "storage helper E2E cleanup was incomplete"
}

case "${1:-}" in
  setup) setup ;;
  test) run_test ;;
  diagnostics) diagnostics ;;
  cleanup) cleanup ;;
  *) echo "usage: $0 <setup|test|diagnostics|cleanup>" >&2; exit 2 ;;
esac

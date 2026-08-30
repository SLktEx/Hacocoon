#!/usr/bin/env bash
set -euo pipefail

readonly INSTANCE="haco-ci-incus-smoke"
readonly IMAGE="images:ubuntu/26.04"

fail() { echo "ERROR: $*" >&2; exit 1; }

require_github_hosted_runner() {
  [[ "${GITHUB_ACTIONS:-}" == "true" ]] || fail "Incus smoke test only runs inside GitHub Actions"
  [[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || fail "Incus smoke test requires a GitHub-hosted runner"
  [[ "$(uname -s)" == "Linux" ]] || fail "Incus smoke test requires Linux"
  . /etc/os-release
  [[ "${ID:-}" == "ubuntu" ]] || fail "Incus smoke test requires Ubuntu"
  [[ "${VERSION_ID:-}" == "26.04" ]] || fail "Incus smoke test requires Ubuntu 26.04, got ${VERSION_ID:-unknown}"
}

setup() {
  require_github_hosted_runner
  sudo env DEBIAN_FRONTEND=noninteractive apt-get update
  sudo env DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends incus-base dnsmasq-base
  sudo incus admin init --minimal
  sudo incus version
}

wait_for_systemd() {
  local pid1="" state=""
  for _ in $(seq 1 30); do
    pid1="$(sudo incus exec "$INSTANCE" -- ps -p 1 -o comm= 2>/dev/null | tr -d '[:space:]' || true)"
    if [[ "$pid1" == "systemd" ]]; then
      state="$(sudo incus exec "$INSTANCE" -- systemctl is-system-running 2>/dev/null || true)"
      case "$state" in
        running|degraded|starting|initializing)
          echo "systemd is responsive (state: $state)"
          return 0
          ;;
      esac
    fi
    sleep 2
  done
  fail "systemd did not become responsive in $INSTANCE (pid1: ${pid1:-unknown}, state: ${state:-unknown})"
}

smoke() {
  require_github_hosted_runner
  sudo incus info "$INSTANCE" >/dev/null 2>&1 && fail "refusing to reuse pre-existing instance $INSTANCE"
  sudo incus launch "$IMAGE" "$INSTANCE"
  wait_for_systemd
  [[ "$(sudo incus exec "$INSTANCE" -- ps -p 1 -o comm= | tr -d '[:space:]')" == "systemd" ]] || fail "PID 1 is not systemd"
  [[ "$(sudo incus exec "$INSTANCE" -- sh -c "printf hacocoon-incus-smoke")" == "hacocoon-incus-smoke" ]] || fail "incus exec output mismatch"
  sudo incus stop "$INSTANCE" --force
  sudo incus delete "$INSTANCE"
  ! sudo incus info "$INSTANCE" >/dev/null 2>&1 || fail "test instance still exists after deletion"
  echo "PASS: standalone Incus system-container smoke test"
}

diagnostics() {
  require_github_hosted_runner
  set +e
  echo '::group::Incus version and server state'; sudo incus version; sudo incus info; echo '::endgroup::'
  echo '::group::Incus instances'; sudo incus list; sudo incus info "$INSTANCE" --show-log 2>/dev/null || true; echo '::endgroup::'
  echo '::group::Incus storage and networks'; sudo incus storage list; sudo incus network list; echo '::endgroup::'
  echo '::group::Incus daemon journal'; sudo journalctl -u incus --no-pager -n 300; echo '::endgroup::'
}

cleanup() {
  require_github_hosted_runner
  set +e
  sudo incus info "$INSTANCE" >/dev/null 2>&1 && sudo incus delete "$INSTANCE" --force
  if sudo incus info "$INSTANCE" >/dev/null 2>&1; then echo "ERROR: cleanup left $INSTANCE behind" >&2; return 1; fi
}

case "${1:-}" in
  setup) setup ;;
  smoke) smoke ;;
  diagnostics) diagnostics ;;
  cleanup) cleanup ;;
  *) echo "usage: $0 <setup|smoke|diagnostics|cleanup>" >&2; exit 2 ;;
esac

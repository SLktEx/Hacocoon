#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/ci-incus-cleanup-library.sh"

readonly SANDBOX_PROFILE="haco-sandbox"
readonly SANDBOX_NETWORK="haco-sandbox0"
readonly SANDBOX_ACL="haco-sandbox-egress"
readonly CI_REMOTE="haco-ci"
readonly CLIENT_CONF="${HACO_CI_INCUS_CONF:-${RUNNER_TEMP:-/tmp}/haco-incus-client}"
export INCUS_CONF="$CLIENT_CONF"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_github_hosted_runner() {
  [[ "${GITHUB_ACTIONS:-}" == "true" ]] || fail "Incus Core E2E only runs inside GitHub Actions"
  [[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || fail "Incus Core E2E requires a GitHub-hosted runner"
  [[ "$(uname -s)" == "Linux" ]] || fail "Incus Core E2E requires Linux"

  . /etc/os-release
  [[ "${ID:-}" == "ubuntu" ]] || fail "Incus Core E2E requires Ubuntu"
  [[ "${VERSION_ID:-}" == "26.04" ]] || fail "Incus Core E2E requires Ubuntu 26.04, got ${VERSION_ID:-unknown}"
}

root_subid_contains() {
  local file="$1"
  local id="$2"

  [[ -r "$file" ]] || return 1
  awk -F: -v id="$id" '
    $1 == "root" && id >= $2 && id - $2 < $3 { found = 1 }
    END { exit found ? 0 : 1 }
  ' "$file"
}

allow_root_subid() {
  local file="$1"
  local id="$2"

  [[ "$id" != "0" ]] || return 0
  root_subid_contains "$file" "$id" && return 0
  printf 'root:%s:1\n' "$id" | sudo tee -a "$file" >/dev/null
}

configure_workspace_owner_idmap() {
  local uid gid
  uid="$(id -u)"
  gid="$(id -g)"

  # Hacocoon keeps the system container unprivileged. Grant Incus only the
  # single host UID/GID that owns the leased runner workspace so raw.idmap can
  # map that identity to container root without granting a broad host range.
  allow_root_subid /etc/subuid "$uid"
  allow_root_subid /etc/subgid "$gid"
}

setup() {
  require_github_hosted_runner
  # Every independent runner uses the same reviewed LTS source and readiness.
  bash "$(dirname "${BASH_SOURCE[0]}")/ci-incus.sh" setup
  configure_workspace_owner_idmap
}

run_test() {
  require_github_hosted_runner
  export HACO_E2E_INCUS=1
  python3 tools/ci_required_tests.py --expect TestRealIncusWorkspaceLifecycleE2E -- go test -v -timeout=10m -count=1 -run '^TestRealIncusWorkspaceLifecycleE2E$' ./modules/runtime/incus
}

run_egress_test() {
  require_github_hosted_runner
  [[ -s "$CLIENT_CONF/config.yml" ]] || fail "trusted Incus TLS client is missing at $CLIENT_CONF; run tools/ci-incus.sh setup first"
  export HACO_E2E_INCUS=1
  python3 tools/ci_required_tests.py --expect TestRealIncusEgressProxyE2E -- go test -v -timeout=10m -count=1 -run '^TestRealIncusEgressProxyE2E$' ./modules/runtime/incus
}

diagnostics() {
  require_github_hosted_runner
  python3 tools/ci_diagnostics.py --output "${GITHUB_WORKSPACE:-$PWD}/ci-failure-substrate.json"
}

cleanup_project() {
  ci_delete_project "$1"
}

cleanup() {
  require_github_hosted_runner
  local project
  local failed=0

  local projects
  projects="$(incus project list --format csv -c n)" || return 1
  while IFS= read -r project; do
    [[ -n "$project" ]] || continue
    case "$project" in
      haco-e2e-*) cleanup_project "$project" || failed=1 ;;
    esac
  done <<< "$projects"

  if incus profile show "$SANDBOX_PROFILE" --project default >/dev/null 2>&1; then
    incus profile delete "$SANDBOX_PROFILE" --project default || failed=1
  fi
  if incus network show "$SANDBOX_NETWORK" --project default >/dev/null 2>&1; then
    incus network delete "$SANDBOX_NETWORK" --project default || failed=1
  fi
  if incus network acl show "$SANDBOX_ACL" --project default >/dev/null 2>&1; then
    incus network acl delete "$SANDBOX_ACL" --project default || failed=1
  fi

  [[ "$failed" == "0" ]] || fail "Incus Core E2E cleanup was incomplete"
}

case "${1:-}" in
  setup) setup ;;
  test) run_test ;;
  egress) run_egress_test ;;
  diagnostics) diagnostics ;;
  cleanup) cleanup ;;
  *) echo "usage: $0 <setup|test|egress|diagnostics|cleanup>" >&2; exit 2 ;;
esac

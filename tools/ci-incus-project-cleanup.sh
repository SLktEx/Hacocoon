#!/usr/bin/env bash
set -euo pipefail

readonly TEST_ROOT="${RUNNER_TEMP:-}/haco-storage-helper-e2e"
readonly CLIENT_CONF="${HACO_CI_INCUS_CONF:-${RUNNER_TEMP:-/tmp}/haco-incus-client}"
export INCUS_CONF="$CLIENT_CONF"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_github_hosted_runner() {
  [[ "${GITHUB_ACTIONS:-}" == "true" ]] || fail "Incus project cleanup only runs inside GitHub Actions"
  [[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || fail "Incus project cleanup requires a GitHub-hosted runner"
  [[ -n "${RUNNER_TEMP:-}" && "$TEST_ROOT" == "${RUNNER_TEMP%/}/haco-storage-helper-e2e" ]] || fail "invalid runner-local storage test root"
  [[ -n "$CLIENT_CONF" ]] || fail "Incus client configuration path is empty"
  [[ "$(uname -s)" == "Linux" ]] || fail "Incus project cleanup requires Linux"
}

cleanup_project() {
  local project="$1"
  local instance unexpected=0

  while IFS= read -r instance; do
    [[ -n "$instance" ]] || continue
    case "$instance" in
      haco-*) ;;
      *)
        echo "ERROR: refusing to force-delete CI-owned project '$project' with unexpected instance '$instance'" >&2
        unexpected=1
        ;;
    esac
  done < <(incus list --project "$project" --format csv -c n 2>/dev/null || true)

  [[ "$unexpected" == "0" ]] || return 1
  printf 'yes\n' | incus project delete "$project" --force
}

cleanup_test_owned_pools() {
  local pool source failed=0

  while IFS= read -r pool; do
    [[ -n "$pool" ]] || continue
    source="$(incus storage get "$pool" source --project default 2>/dev/null || true)"
    [[ -n "$source" ]] || continue
    case "$source" in
      "$TEST_ROOT"/*)
        incus storage delete "$pool" --project default || failed=1
        ;;
    esac
  done < <(incus storage list --project default --format csv -c n 2>/dev/null || true)

  [[ "$failed" == "0" ]]
}

main() {
  require_github_hosted_runner
  local project failed=0

  while IFS= read -r project; do
    [[ -n "$project" ]] || continue
    case "$project" in
      hacocoon|haco-e2e-*) cleanup_project "$project" || failed=1 ;;
    esac
  done < <(incus project list --format csv -c n 2>/dev/null || true)

  cleanup_test_owned_pools || failed=1
  [[ "$failed" == "0" ]] || fail "Incus project cleanup was incomplete"
}

main "$@"

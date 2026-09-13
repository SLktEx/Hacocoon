#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/ci-incus-cleanup-library.sh"

readonly CLIENT_CONF="${HACO_CI_INCUS_CONF:-${RUNNER_TEMP:-/tmp}/haco-incus-client}"
export INCUS_CONF="$CLIENT_CONF"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_github_hosted_runner() {
  [[ "${GITHUB_ACTIONS:-}" == "true" ]] || fail "Incus project cleanup only runs inside GitHub Actions"
  [[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || fail "Incus project cleanup requires a GitHub-hosted runner"
  [[ -n "$CLIENT_CONF" ]] || fail "Incus client configuration path is empty"
  [[ "$(uname -s)" == "Linux" ]] || fail "Incus project cleanup requires Linux"
}

cleanup_project() {
  ci_delete_project "$1"
}

main() {
  require_github_hosted_runner
  local project failed=0

  local projects
  projects="$(incus project list --format csv -c n)" || return 1
  while IFS= read -r project; do
    [[ -n "$project" ]] || continue
    case "$project" in
      hacocoon|haco-e2e-*) cleanup_project "$project" || failed=1 ;;
    esac
  done <<< "$projects"

  [[ "$failed" == "0" ]] || fail "Incus project cleanup was incomplete"
}

main "$@"

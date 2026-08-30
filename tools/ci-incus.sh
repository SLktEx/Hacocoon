#!/usr/bin/env bash
set -euo pipefail

readonly CI_REMOTE="haco-ci"
readonly SANDBOX_PROFILE="haco-sandbox"
readonly SANDBOX_NETWORK="haco-sandbox0"
readonly SANDBOX_ACL="haco-sandbox-egress"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_github_hosted_runner() {
  [[ "${GITHUB_ACTIONS:-}" == "true" ]] || fail "Incus CI helper only runs inside GitHub Actions"
  [[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || fail "Incus CI helper requires a GitHub-hosted runner"
  [[ "$(uname -s)" == "Linux" ]] || fail "Incus CI helper requires Linux"
}

setup_incus() {
  require_github_hosted_runner

  sudo env DEBIAN_FRONTEND=noninteractive apt-get update
  sudo env DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends incus dnsmasq-base

  # Hacocoon deliberately keeps bridge IP filtering enabled. Incus 6.0
  # requires the host's bridge netfilter hooks for that policy, while the
  # GitHub-hosted Ubuntu image does not load br_netfilter by default.
  sudo modprobe br_netfilter
  [[ -e /proc/sys/net/bridge/bridge-nf-call-iptables ]] || fail "br_netfilter did not expose bridge netfilter sysctls"
  [[ -e /proc/sys/net/bridge/bridge-nf-call-ip6tables ]] || fail "br_netfilter did not expose IPv6 bridge netfilter sysctls"

  sudo incus admin init --minimal

  incus remote generate-certificate
  sudo incus config set core.https_address 127.0.0.1:8443
  sudo incus config trust add-certificate "${HOME}/.config/incus/client.crt"
  incus remote add "${CI_REMOTE}" https://127.0.0.1:8443 --accept-certificate
  incus remote switch "${CI_REMOTE}"

  incus version
  incus profile show default --project default >/dev/null
}

run_e2e() {
  require_github_hosted_runner
  export HACO_E2E_INCUS=1

  go test -count=1 -run '^TestRealIncusWorkspaceLifecycleE2E$' ./modules/runtime/incus
  bash test/e2e/incus.sh
}

diagnostics() {
  require_github_hosted_runner
  set +e

  echo '::group::Incus version'
  incus version
  echo '::endgroup::'

  echo '::group::Incus projects and instances'
  incus project list
  incus list --all-projects
  echo '::endgroup::'

  echo '::group::Incus default-project network state'
  incus network list --project default
  incus network acl list --project default
  incus profile list --project default
  echo '::endgroup::'

  echo '::group::Incus daemon journal'
  sudo journalctl -u incus --no-pager -n 300
  echo '::endgroup::'
}

cleanup_project() {
  local project="$1"
  local instance
  local unexpected=0

  while IFS= read -r instance; do
    [[ -n "$instance" ]] || continue
    case "$instance" in
      haco-*)
        incus delete "$instance" --project "$project" --force || return 1
        ;;
      *)
        echo "ERROR: refusing to delete unexpected instance '$instance' from CI-owned project '$project'" >&2
        unexpected=1
        ;;
    esac
  done < <(incus list --project "$project" --format csv -c n 2>/dev/null || true)

  [[ "$unexpected" == "0" ]] || return 1
  incus project delete "$project"
}

cleanup() {
  require_github_hosted_runner
  local project
  local failed=0

  while IFS= read -r project; do
    [[ -n "$project" ]] || continue
    case "$project" in
      hacocoon|haco-e2e-*)
        cleanup_project "$project" || failed=1
        ;;
    esac
  done < <(incus project list --format csv -c n 2>/dev/null || true)

  if incus profile show "$SANDBOX_PROFILE" --project default >/dev/null 2>&1; then
    incus profile delete "$SANDBOX_PROFILE" --project default || failed=1
  fi
  if incus network show "$SANDBOX_NETWORK" --project default >/dev/null 2>&1; then
    incus network delete "$SANDBOX_NETWORK" --project default || failed=1
  fi
  if incus network acl show "$SANDBOX_ACL" --project default >/dev/null 2>&1; then
    incus network acl delete "$SANDBOX_ACL" --project default || failed=1
  fi

  if [[ "$failed" != "0" ]]; then
    echo "ERROR: Incus CI cleanup was incomplete" >&2
    return 1
  fi
}

usage() {
  echo "usage: $0 <setup|test|diagnostics|cleanup>" >&2
}

case "${1:-}" in
  setup)
    setup_incus
    ;;
  test)
    run_e2e
    ;;
  diagnostics)
    diagnostics
    ;;
  cleanup)
    cleanup
    ;;
  *)
    usage
    exit 2
    ;;
esac

#!/usr/bin/env bash
set -euo pipefail

readonly SANDBOX_PROFILE="haco-sandbox"
readonly SANDBOX_NETWORK="haco-sandbox0"
readonly SANDBOX_ACL="haco-sandbox-egress"
readonly CI_REMOTE="haco-ci"

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
  local server_version

  require_github_hosted_runner
  sudo env DEBIAN_FRONTEND=noninteractive apt-get update
  sudo env DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends \
    incus-base \
    dnsmasq-base

  configure_workspace_owner_idmap

  # The Hacocoon sandbox keeps Incus bridge filtering enabled. GitHub-hosted
  # Ubuntu does not guarantee that br_netfilter is loaded before the job.
  sudo modprobe br_netfilter
  [[ -e /proc/sys/net/bridge/bridge-nf-call-iptables ]] || fail "br_netfilter IPv4 hooks unavailable"
  [[ -e /proc/sys/net/bridge/bridge-nf-call-ip6tables ]] || fail "br_netfilter IPv6 hooks unavailable"

  sudo incus admin init --minimal

  # Run Hacocoon as the ordinary runner user, not root. Give that user a
  # runner-local TLS client trusted by the disposable daemon rather than
  # exposing Incus' root-owned Unix socket.
  incus remote generate-certificate
  sudo incus config set core.https_address 127.0.0.1:8443
  sudo incus config trust add-certificate "${HOME}/.config/incus/client.crt"
  incus remote add "$CI_REMOTE" https://127.0.0.1:8443 --accept-certificate
  incus remote switch "$CI_REMOTE"

  incus version
  server_version="$(incus version | awk -F': ' '$1 == "Server version" {print $2; exit}')"
  [[ -n "$server_version" ]] || fail "could not determine Incus server version"
  dpkg --compare-versions "$server_version" ge 6.0.5 || fail "Incus $server_version is too old; 6.0.5+ is required"
  incus profile show default --project default >/dev/null
}

run_test() {
  require_github_hosted_runner
  export HACO_E2E_INCUS=1
  go test -count=1 -run '^TestRealIncusWorkspaceLifecycleE2E$' ./modules/runtime/incus
}

run_egress_test() {
  require_github_hosted_runner
  export HACO_E2E_INCUS=1
  go test -count=1 -run '^TestRealIncusEgressProxyE2E$' ./modules/runtime/incus
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

  echo '::group::Hacocoon shared Incus resources'
  incus network list --project default
  incus network acl list --project default
  incus profile list --project default
  echo '::endgroup::'

  echo '::group::Incus instance logs'
  while IFS=, read -r project instance; do
    [[ -n "$project" && -n "$instance" ]] || continue
    case "$project:$instance" in
      haco-e2e-*:haco-*) incus info "$instance" --project "$project" --show-log || true ;;
    esac
  done < <(incus list --all-projects --format csv -c p,n 2>/dev/null || true)
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
      haco-*) ;;
      *)
        echo "ERROR: refusing to force-delete project '$project' with unexpected instance '$instance'" >&2
        unexpected=1
        ;;
    esac
  done < <(incus list --project "$project" --format csv -c n 2>/dev/null || true)

  [[ "$unexpected" == "0" ]] || return 1
  printf 'yes\n' | incus project delete "$project" --force
}

cleanup() {
  require_github_hosted_runner
  local project
  local failed=0

  while IFS= read -r project; do
    [[ -n "$project" ]] || continue
    case "$project" in
      haco-e2e-*) cleanup_project "$project" || failed=1 ;;
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

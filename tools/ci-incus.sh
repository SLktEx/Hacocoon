#!/usr/bin/env bash
set -euo pipefail

readonly CI_REMOTE="haco-ci"
readonly SANDBOX_PROFILE="haco-sandbox"
readonly SANDBOX_NETWORK="haco-sandbox0"
readonly SANDBOX_ACL="haco-sandbox-egress"
readonly ZABBLY_SIGNING_FPR="4EFC590696CB15B87C73A3AD""82CC8797C838DCFD"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_github_hosted_runner() {
  [[ "${GITHUB_ACTIONS:-}" == "true" ]] || fail "Incus CI helper only runs inside GitHub Actions"
  [[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || fail "Incus CI helper requires a GitHub-hosted runner"
  [[ "$(uname -s)" == "Linux" ]] || fail "Incus CI helper requires Linux"
}

install_zabbly_incus_lts() {
  local codename="$1"
  local key_file source_file fingerprint architecture

  sudo env DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends ca-certificates curl gnupg

  key_file="$(mktemp)"
  curl -fsSL https://pkgs.zabbly.com/key.asc -o "$key_file"
  fingerprint="$(gpg --batch --show-keys --with-colons --fingerprint "$key_file" | awk -F: '$1 == "fpr" {print $10; exit}')"
  [[ "$fingerprint" == "$ZABBLY_SIGNING_FPR" ]] || fail "unexpected Zabbly signing fingerprint: $fingerprint"

  sudo install -d -m 0755 /etc/apt/keyrings
  sudo install -m 0644 "$key_file" /etc/apt/keyrings/zabbly.asc
  rm -f "$key_file"

  architecture="$(dpkg --print-architecture)"
  source_file="$(mktemp)"
  cat >"$source_file" <<EOF
Enabled: yes
Types: deb
URIs: https://pkgs.zabbly.com/incus/lts-6.0
Suites: $codename
Components: main
Architectures: $architecture
Signed-By: /etc/apt/keyrings/zabbly.asc
EOF
  sudo install -m 0644 "$source_file" /etc/apt/sources.list.d/zabbly-incus-lts-6.0.sources
  rm -f "$source_file"

  sudo env DEBIAN_FRONTEND=noninteractive apt-get update
  sudo env DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends incus-base
}

install_incus() {
  local codename

  sudo env DEBIAN_FRONTEND=noninteractive apt-get update
  codename="$(. /etc/os-release && printf '%s' "$VERSION_CODENAME")"

  if [[ "$codename" == "noble" ]]; then
    # Ubuntu 24.04's archive currently carries Incus 6.0.0, which predates
    # the Linux 6.9+ idmapped-mount fix. Use the upstream 6.0 LTS packages
    # on noble only; newer Ubuntu runners use their native package instead.
    install_zabbly_incus_lts "$codename"
    return
  fi

  sudo env DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends incus
}

setup_incus() {
  local server_version

  require_github_hosted_runner
  install_incus

  # Hacocoon deliberately keeps bridge IP filtering enabled. Incus requires
  # the host's bridge netfilter hooks for that policy, while GitHub-hosted
  # Ubuntu images do not necessarily load br_netfilter by default.
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
  server_version="$(incus version | awk -F': ' '$1 == "Server version" {print $2; exit}')"
  [[ -n "$server_version" ]] || fail "could not determine Incus server version"
  dpkg --compare-versions "$server_version" ge 6.0.5 || fail "Incus $server_version is too old; 6.0.5+ is required for Linux 6.9+ idmapped mounts"
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

  # Force project deletion is safe only after proving every instance in this
  # exact CI-owned project has a Hacocoon test name. The force flag also
  # removes project-scoped cached images/volumes left by a failed init.
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
  incus project delete "$project" --force
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

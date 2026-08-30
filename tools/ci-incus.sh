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

ci_prefix() {
  local run_id="${GITHUB_RUN_ID:-0}"
  local attempt="${GITHUB_RUN_ATTEMPT:-0}"
  local tail="${run_id: -6}"
  printf 'hci-%s-%s\n' "$tail" "$attempt"
}

readonly CI_PREFIX="${HACO_CI_PREFIX:-$(ci_prefix)}"
readonly DIAGNOSTICS_DIR="${HACO_CI_DIAGNOSTICS_DIR:-${RUNNER_TEMP:-/tmp}/incus-diagnostics}"

require_github_hosted_runner() {
  [[ "${GITHUB_ACTIONS:-}" == "true" ]] || fail "Incus CI helper only runs inside GitHub Actions"
  [[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || fail "Incus CI helper requires a GitHub-hosted runner"
  [[ "$(uname -s)" == "Linux" ]] || fail "Incus CI helper requires Linux"
  [[ "$CI_PREFIX" =~ ^hci-[0-9]+-[0-9]+$ ]] || fail "unsafe Incus CI prefix: $CI_PREFIX"
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
  cat >"$source_file" <<EOF_ZABBLY
Enabled: yes
Types: deb
URIs: https://pkgs.zabbly.com/incus/lts-6.0
Suites: $codename
Components: main
Architectures: $architecture
Signed-By: /etc/apt/keyrings/zabbly.asc
EOF_ZABBLY
  sudo install -m 0644 "$source_file" /etc/apt/sources.list.d/zabbly-incus-lts-6.0.sources
  rm -f "$source_file"

  sudo env DEBIAN_FRONTEND=noninteractive apt-get update
  # Managed Incus bridges require the dnsmasq executable. Keep dependencies
  # explicit because CI intentionally disables apt Recommends.
  sudo env DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends incus-base dnsmasq-base
}

install_incus() {
  local codename

  sudo env DEBIAN_FRONTEND=noninteractive apt-get update
  codename="$(. /etc/os-release && printf '%s' "$VERSION_CODENAME")"

  if [[ "$codename" == "noble" ]]; then
    # Ubuntu 24.04's archive carries an old Incus 6.0 point release. Retain
    # this compatibility path for local/manual use, but normal CI is 26.04.
    install_zabbly_incus_lts "$codename"
    return
  fi

  # dnsmasq-base is only Recommended by the Ubuntu Incus package, but
  # `incus admin init --minimal` creates a managed bridge and needs it.
  sudo env DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends incus dnsmasq-base
}

record_environment() {
  echo '::group::GitHub runner / Incus environment'
  cat /etc/os-release
  uname -a
  systemctl --version | head -n 1 || true
  incus version
  incus info
  incus storage list
  incus network list --project default
  ip -4 address show
  ip -4 route show
  sysctl net.ipv4.ip_forward || true
  echo '::endgroup::'
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

  server_version="$(incus version | awk -F': ' '$1 == "Server version" {print $2; exit}')"
  [[ -n "$server_version" ]] || fail "could not determine Incus server version"
  dpkg --compare-versions "$server_version" ge 6.0.5 || fail "Incus $server_version is too old; 6.0.5+ is required for Linux 6.9+ idmapped mounts"
  incus profile show default --project default >/dev/null
  record_environment
}

run_standalone_e2e() {
  require_github_hosted_runner
  export HACO_CI_PREFIX="$CI_PREFIX"
  export HACO_CI_INCUS_STANDALONE=1
  bash test/e2e/incus_standalone.sh
}

run_core_e2e() {
  require_github_hosted_runner
  export HACO_E2E_INCUS=1

  go test -count=1 -run '^TestRealIncusWorkspaceLifecycleE2E$' ./modules/runtime/incus
  bash test/e2e/incus.sh
}

incus_diag() {
  if incus info >/dev/null 2>&1; then
    incus "$@"
  else
    sudo incus "$@"
  fi
}

capture_instance_diagnostics() {
  local project="$1"
  local instance

  while IFS= read -r instance; do
    [[ -n "$instance" ]] || continue
    case "$project:$instance" in
      "default:${CI_PREFIX}"*|hacocoon:haco-*) ;;
      *) continue ;;
    esac

    echo "--- instance $project/$instance config ---"
    incus_diag config show "$instance" --expanded --project "$project" || true
    echo "--- instance $project/$instance info ---"
    incus_diag info "$instance" --project "$project" || true
    echo "--- guest $project/$instance addresses/routes ---"
    incus_diag exec "$instance" --project "$project" -- ip address || true
    incus_diag exec "$instance" --project "$project" -- ip route || true
    echo "--- guest $project/$instance systemd ---"
    incus_diag exec "$instance" --project "$project" -- systemctl status --no-pager || true
    incus_diag exec "$instance" --project "$project" -- journalctl -b --no-pager -n 250 || true
  done < <(incus_diag list --project "$project" --format csv -c n 2>/dev/null || true)
}

diagnostics() {
  require_github_hosted_runner
  set +e
  mkdir -p "$DIAGNOSTICS_DIR"

  {
    echo "diagnostics timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "CI prefix: $CI_PREFIX"

    echo '=== runner ==='
    cat /etc/os-release
    uname -a
    systemctl --version | head -n 3

    if command -v incus >/dev/null 2>&1; then
      echo '=== Incus info/version ==='
      incus_diag version
      incus_diag info
      echo '=== Incus projects/instances ==='
      incus_diag project list
      incus_diag list --all-projects
      echo '=== Incus networks ==='
      incus_diag network list --project default
      while IFS= read -r network; do
        [[ -n "$network" ]] || continue
        echo "--- network $network ---"
        incus_diag network show "$network" --project default || true
      done < <(incus_diag network list --project default --format csv -c n 2>/dev/null || true)
      echo '=== Incus ACLs/profiles ==='
      incus_diag network acl list --project default
      incus_diag profile list --project default
      echo '=== Incus storage ==='
      incus_diag storage list
      while IFS= read -r pool; do
        [[ -n "$pool" ]] || continue
        echo "--- storage $pool ---"
        incus_diag storage show "$pool" || true
      done < <(incus_diag storage list --format csv -c n 2>/dev/null || true)
      capture_instance_diagnostics default
      capture_instance_diagnostics hacocoon
    else
      echo 'incus executable is unavailable'
    fi

    echo '=== host networking/routing ==='
    ip -details address show || true
    ip route show table all || true
    ip rule show || true
    bridge link show || true
    sysctl net.ipv4.ip_forward || true
    sysctl net.bridge.bridge-nf-call-iptables || true
    sysctl net.bridge.bridge-nf-call-ip6tables || true

    echo '=== host firewall ==='
    sudo nft list ruleset || sudo iptables-save || true

    echo '=== Incus daemon journal ==='
    sudo journalctl -u incus --no-pager -n 500 || true
  } 2>&1 | tee "$DIAGNOSTICS_DIR/diagnostics.log"
}

default_root_pool() {
  incus profile device get default root pool --project default 2>/dev/null || true
}

cleanup_standalone() {
  require_github_hosted_runner
  local failed=0
  local instance network profile volume pool
  local instances=("${CI_PREFIX}a" "${CI_PREFIX}b")
  local networks=("${CI_PREFIX}x" "${CI_PREFIX}n")
  profile="${CI_PREFIX}p"
  volume="${CI_PREFIX}v"

  for instance in "${instances[@]}"; do
    if incus info "$instance" --project default >/dev/null 2>&1; then
      incus delete "$instance" --project default --force || failed=1
    fi
  done

  if incus profile show "$profile" --project default >/dev/null 2>&1; then
    incus profile delete "$profile" --project default || failed=1
  fi

  for network in "${networks[@]}"; do
    if incus network show "$network" --project default >/dev/null 2>&1; then
      incus network delete "$network" --project default || failed=1
    fi
  done

  pool="$(default_root_pool)"
  if [[ -n "$pool" ]] && incus storage volume show "$pool" "$volume" --project default >/dev/null 2>&1; then
    incus storage volume delete "$pool" "$volume" --project default || failed=1
  fi

  [[ "$failed" == "0" ]] || {
    echo "ERROR: standalone Incus CI cleanup was incomplete" >&2
    return 1
  }
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

cleanup_core() {
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

  [[ "$failed" == "0" ]] || {
    echo "ERROR: Hacocoon Core Incus CI cleanup was incomplete" >&2
    return 1
  }
}

usage() {
  echo "usage: $0 <setup|standalone|core|diagnostics|cleanup-standalone|cleanup-core>" >&2
}

case "${1:-}" in
  setup)
    setup_incus
    ;;
  standalone)
    run_standalone_e2e
    ;;
  core)
    run_core_e2e
    ;;
  diagnostics)
    diagnostics
    ;;
  cleanup-standalone)
    cleanup_standalone
    ;;
  cleanup-core)
    cleanup_core
    ;;
  *)
    usage
    exit 2
    ;;
esac

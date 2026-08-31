#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$repo_root"
export GOTOOLCHAIN=local

usage() {
  cat <<'USAGE'
Usage: bash tools/ci-local.sh [all|docs|workflow-policy|release-config|systemd|test|race|e2e]

Mirrors the checks in .github/workflows/test.yml using the local machine.
The release-config job intentionally fails if dist/ already exists because
GoReleaser --clean would otherwise delete an existing local directory.
USAGE
}

fail() { printf 'local CI: %s\n' "$*" >&2; exit 2; }
need() { command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"; }
section() { printf '\n==> %s\n' "$*"; }

check_go() {
  need go
  section "Go toolchain"
  go version
}

run_docs() {
  need python3
  section "docs"
  python3 tools/check_docs.py
  python3 tools/test_check_docs.py
}

run_workflow_policy() {
  need python3
  section "workflow-policy"
  python3 tools/check_workflow_policy.py
  python3 tools/test_workflow_policy.py
  python3 tools/test_public_release_readiness.py
  python3 tools/check_renovate_policy.py
  python3 tools/test_renovate_policy.py
}

validate_install_boundary() {
  grep -q -- '--name' scripts/install-windows.ps1
  grep -q -- '--set-version' scripts/install-windows.ps1
  grep -q -- '--terminate' scripts/install-windows.ps1
  grep -q 'systemd=true' scripts/install-windows.ps1
  grep -q 'Running common Ubuntu install.sh' scripts/install-windows.ps1
  grep -q 'HACO_BUNDLE_ROOT' scripts/install-windows.ps1
  grep -q 'this package is for native Ubuntu' scripts/install-ubuntu.sh
  grep -q 'HACO_BUNDLE_ROOT' scripts/install-ubuntu.sh
  grep -q 'Hacocoon common Ubuntu installation complete' scripts/install.sh
  ! grep -q 'WSL_DISTRO_NAME' scripts/install.sh
  ! grep -q 'systemd=true' scripts/install.sh
  ! grep -q 'hacocoon-login' scripts/install.sh
  ! grep -q -- '--set-default-version' scripts/install-windows.ps1
  ! grep -q -- '--shutdown' scripts/install-windows.ps1
}

run_systemd() {
  need chmod
  need cp
  need mktemp
  need systemd-analyze
  section "systemd packaging"
  local verify_root
  verify_root="$(mktemp -d)"
  (
    trap 'rm -rf -- "$verify_root"' EXIT
    mkdir -p "$verify_root/etc/systemd/system" "$verify_root/usr/bin" "$verify_root/bin"
    cp modules/plugin/oci/packaging/systemd/hacocoon-docker.service "$verify_root/etc/systemd/system/"
    cp modules/plugin/oci/packaging/systemd/hacocoon-docker.socket "$verify_root/etc/systemd/system/"
    local target
    for target in sysinit.target basic.target sockets.target network-online.target shutdown.target; do
      cat >"$verify_root/etc/systemd/system/$target" <<EOF
[Unit]
Description=local CI stub $target
DefaultDependencies=no
EOF
    done
    cat >"$verify_root/etc/systemd/system/containerd.service" <<'EOF'
[Unit]
Description=local CI containerd stub
DefaultDependencies=no
[Service]
ExecStart=/usr/bin/containerd
EOF
    printf '#!/bin/sh\nexit 0\n' >"$verify_root/usr/bin/dockerd"
    printf '#!/bin/sh\nexit 0\n' >"$verify_root/usr/bin/containerd"
    printf '#!/bin/sh\nexit 0\n' >"$verify_root/bin/kill"
    chmod 0755 "$verify_root/usr/bin/dockerd" "$verify_root/usr/bin/containerd" "$verify_root/bin/kill"
    systemd-analyze verify --root="$verify_root" hacocoon-docker.socket hacocoon-docker.service
  )
}

validate_release_artifacts() {
  test -f dist/haco_linux_amd64.tar.gz
  test -f dist/haco_linux_arm64.tar.gz
  test -f dist/checksums.txt

  local archive listing
  for archive in dist/haco_linux_amd64.tar.gz dist/haco_linux_arm64.tar.gz; do
    listing="$(tar -tzf "$archive")"
    for binary in haco haco-controller haco-host haco-vscode haco-agent-host haco-notify haco-storage-helper; do
      grep -Fx "$binary" <<<"$listing" >/dev/null
    done
  done

  rm -rf release-payload-test
  python3 tools/package_installers.py --dist dist --output release-payload-test --version v0.0.0-test
  (
    cd release-payload-test
    sha256sum -c checksums.txt
    test -f hacocoon-windows-amd64.zip
    test -f hacocoon-windows-arm64.zip
    test -f hacocoon-ubuntu-amd64.tar.gz
    test -f hacocoon-ubuntu-arm64.tar.gz
  )
}

run_release_config() {
  need bash
  need git
  need grep
  need python3
  need pwsh
  need goreleaser
  need tar
  check_go

  section "release-config: trust, provenance, and package contracts"
  bash tools/test_release_tag_trust.sh
  python3 tools/check_release_provenance.py
  bash tools/test_install_archive_safety.sh
  python3 tools/test_installer_packages.py

  section "release-config: GoReleaser config"
  goreleaser check

  section "release-config: shell syntax"
  bash -n scripts/install.sh scripts/install-ubuntu.sh tools/ci-local.sh tools/check_release_tag_trust.sh tools/test_release_tag_trust.sh tools/test_install_archive_safety.sh

  section "release-config: Windows installer syntax"
  pwsh -NoLogo -NoProfile -NonInteractive -Command '
    $ErrorActionPreference = "Stop"
    [scriptblock]::Create((Get-Content -Raw "scripts/install-windows.ps1")) | Out-Null
  '

  section "release-config: pre/main/post boundary"
  validate_install_boundary
  run_systemd

  if [[ -e dist ]]; then
    fail "dist/ already exists; refusing to run 'goreleaser release --clean'. Move or remove dist/ explicitly first."
  fi

  section "release-config: snapshot packaging"
  goreleaser release --snapshot --clean --skip=publish
  validate_release_artifacts
}

run_test() {
  check_go
  need node
  section "test"
  go test -count=1 -shuffle=on ./...
  go vet ./...
  section "notification clients"
  node --check pkg/interactionhttp/web/app.js
  node --check clients/vscode-notify/extension.js
  node --test test/js/notification_clients.test.js
}

run_race() {
  check_go
  section "race"
  go test -race -count=1 ./...
}

run_e2e() {
  need bash
  check_go
  section "e2e: shell syntax"
  bash -n test/e2e/*.sh
  section "e2e: shipped commands"
  bash test/e2e/commands.sh
  section "e2e: capability"
  bash test/e2e/capability.sh
  section "e2e: git/github"
  bash test/e2e/git_github.sh
  section "e2e: orchestrator"
  HACO_STORAGE_PRIVILEGE_MODE=direct bash test/e2e/orchestrator.sh
}

run_all() {
  run_docs
  run_workflow_policy
  run_release_config
  run_test
  run_race
  run_e2e
}

if (( $# > 1 )); then usage >&2; exit 2; fi
case "${1:-all}" in
  all) run_all ;;
  docs) run_docs ;;
  workflow-policy) run_workflow_policy ;;
  release-config) run_release_config ;;
  systemd) run_systemd ;;
  test) run_test ;;
  race) run_race ;;
  e2e) run_e2e ;;
  -h|--help|help) usage ;;
  *) usage >&2; exit 2 ;;
esac

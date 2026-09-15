#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$repo_root"
export GOTOOLCHAIN=local

usage() {
  cat <<'USAGE'
Usage: bash tools/ci-local.sh [all|docs|workflow-policy|release-config|test|race|e2e|forwarding|aws]

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
  python3 tools/check_ci_contracts.py
  python3 tools/test_ci_diagnostics.py
  python3 tools/test_ci_cleanup.py
  python3 tools/test_ci_required_tests.py
  python3 tools/test_ci_history.py
  python3 tools/test_ci_contracts.py
  python3 tools/check_workflow_policy.py
  python3 tools/test_workflow_policy.py
  python3 tools/test_public_release_readiness.py
  python3 tools/check_renovate_policy.py
  python3 tools/test_renovate_policy.py
}

validate_install_boundary() {
  grep -q -- '--name' install/install-windows.ps1
  grep -q -- '--set-version' install/install-windows.ps1
  grep -q -- '--terminate' install/install-windows.ps1
  grep -q 'systemd=true' install/install-windows.ps1
  grep -q 'Running common Ubuntu install.sh' install/install-windows.ps1
  grep -q 'HACO_BUNDLE_ROOT' install/install-windows.ps1
  grep -q 'UseCachedWslImage' install/install-windows.ps1
  grep -q 'Get-CachedUbuntuWslImage' install/install-windows.ps1
  grep -q 'DistributionInfo.json' install/install-windows.ps1
  grep -q -- '--from-file' install/install-windows.ps1
  grep -q 'Get-Sha256Hex' install/install-windows.ps1
  grep -q 'Security.Cryptography.SHA256' install/install-windows.ps1
  grep -q 'ubuntu.wsl' install/install-windows.ps1
  grep -q 'this package is for native Ubuntu' install/install-ubuntu.sh
  grep -q 'HACO_BUNDLE_ROOT' install/install-ubuntu.sh
  grep -q 'Hacocoon common Ubuntu installation complete' install/install.sh
  ! grep -q 'WSL_DISTRO_NAME' install/install.sh
  ! grep -q 'systemd=true' install/install.sh
  ! grep -q 'hacocoon-login' install/install.sh
  ! grep -q -- '--set-default-version' install/install-windows.ps1
  ! grep -q -- '--shutdown' install/install-windows.ps1
}

validate_release_artifacts() {
  test -f dist/haco_linux_amd64.tar.gz
  test -f dist/haco_linux_arm64.tar.gz
  test -f dist/checksums.txt

  local archive listing
  for archive in dist/haco_linux_amd64.tar.gz dist/haco_linux_arm64.tar.gz; do
    listing="$(tar -tzf "$archive")"
    for binary in haco haco-controller haco-host haco-vscode haco-agent-host haco-notify; do
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

  section "release-config: Actions syntax and expressions"
  go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -shellcheck= -pyflakes= -ignore '^label "ubuntu-26.04" is unknown'

  section "release-config: trust, provenance, and package contracts"
  bash tools/test_release_tag_trust.sh
  python3 tools/check_release_provenance.py
  bash tools/test_install_archive_safety.sh
  python3 tools/test_installer_packages.py
  python3 tools/test_install_identity.py
  python3 tools/test_install_network.py
  python3 tools/test_incus_lts.py
  python3 tools/test_incus_boot_guard.py
  python3 tools/test_windows_user_path.py
  python3 tools/test_wsl_oobe_config.py

  section "release-config: GoReleaser config"
  goreleaser check

  section "release-config: shell syntax"
  bash -n install/incus-lts.sh install/install.sh install/install-ubuntu.sh tools/ci-local.sh tools/check_release_tag_trust.sh tools/test_release_tag_trust.sh tools/test_install_archive_safety.sh

  section "release-config: Windows installer syntax"
  pwsh -NoLogo -NoProfile -NonInteractive -Command '
    $ErrorActionPreference = "Stop"
    [scriptblock]::Create((Get-Content -Raw "install/install-windows.ps1")) | Out-Null
  '

  section "release-config: pre/main/post boundary"
  pwsh -NoLogo -NoProfile -NonInteractive -File tools/test_windows_installer.ps1
  pwsh -NoLogo -NoProfile -NonInteractive -File tools/test_wsl_stop_readiness.ps1
  validate_install_boundary

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
  need python3
  python3 tools/test_wsl_host_interop.py
  python3 tools/test_pending_approvals_test.py
  python3 tools/test_windows_transfer_bundle_copy.py
  python3 tools/test_forwarding_fixture.py
  python3 tools/test_evacuation_inventory.py
  python3 tools/test_evacuation_associations.py
  python3 tools/test_evacuation_current_data.py
  python3 tools/test_evacuation_capture.py
  python3 tools/test_evacuation_files.py
  python3 tools/test_cleanup_ci_base_asset.py
  section "test"
  go test -count=1 -shuffle=615 ./...
  go vet ./...
  section "notification clients"
  node --check pkg/interactionhttp/web/app.js
  node --check clients/vscode-notify/extension.js
  node --check clients/vscode-notify/review.js
  node --test test/js/notification_clients.test.js test/js/vscode_acceptance.test.js test/js/approval_review.test.js test/js/approval_panel.test.js
  python3 tools/test_vscode_packaging.py
}

run_race() {
  check_go
  section "race"
  go test -race -count=1 ./...
}

run_forwarding() {
  check_go
  section "trusted-host forwarding: isolated Linux kernel regression"
  bash tools/test_trusted_host_forwarding.sh
}

run_e2e() {
  need bash
  check_go
  section "e2e: shell syntax"
  bash -n test/e2e/*.sh
  section "e2e: shipped commands"
  bash test/e2e/commands.sh
  section "e2e: orchestrator"
  bash test/e2e/orchestrator.sh
}

run_all() {
  run_docs
  run_workflow_policy
  run_release_config
  run_test
  run_race
  run_e2e
  run_forwarding
}

if (( $# > 1 )); then usage >&2; exit 2; fi
case "${1:-all}" in
  all) run_all ;;
  docs) run_docs ;;
  workflow-policy) run_workflow_policy ;;
  release-config) run_release_config ;;
  aws) "${HACO_AWS_TEST_PYTHON:-python3}" internal/adapters/aws/test_host_agent.py ;;
  test) run_test ;;
  race) run_race ;;
  e2e) run_e2e ;;
  forwarding) run_forwarding ;;
  -h|--help|help) usage ;;
  *) usage >&2; exit 2 ;;
esac

#!/usr/bin/env bash
# Optional capability discovery on a fresh runner, separate from product E2E.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$repo_root"
[[ "${GITHUB_ACTIONS:-}" == true && "${HACO_CI_RUNNER_ENVIRONMENT:-}" == github-hosted ]] || {
  echo 'Incus VM bootstrap requires a disposable GitHub-hosted runner' >&2; exit 1;
}
[[ "${GITHUB_RUN_ID:-}" =~ ^[0-9]{1,20}$ && "${GITHUB_RUN_ATTEMPT:-}" =~ ^[0-9]{1,10}$ ]] || exit 1
export INCUS_CONF="${RUNNER_TEMP:?}/haco-incus-client"
output="$RUNNER_TEMP/incus-vm-probe.json"
case "${1:-}" in
  setup)
    bash tools/ci-incus.sh setup
    # The shared pinned-key LTS setup stays container-only. Only this fresh job
    # adds the matching full package (QEMU/firmware dependencies) from that source.
    version="$(dpkg-query -W -f='${Version}' incus-base)"
    [[ "$version" =~ ^1:7\.0\.[0-9]+-[0-9A-Za-z.+~_-]+$ ]] || exit 1
    sudo env DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends "incus=$version"
    # Driver availability is cached at daemon startup, before QEMU was installed.
    sudo systemctl restart incus.service
    timeout 65 sudo incus admin waitready --timeout=60
    sh install/incus-lts.sh verify-server
    ;;
  probe)
    python3 tools/incus_vm_probe.py probe --name "hci-$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT-vm" --output "$output"
    ;;
  cleanup)
    python3 tools/incus_vm_probe.py cleanup --output "$output"
    ;;
  *) echo 'usage: ci-incus-vm.sh <setup|probe|cleanup>' >&2; exit 2 ;;
esac

#!/usr/bin/env bash
set -euo pipefail

readonly NERDCTL_VERSION="2.3.5"
readonly NERDCTL_AMD64_SHA256="de3206aeb7cbd5f20f5fb1f55c1e3bf2db1be567812a8a3f5e65eba2488347ee"

install_nerdctl() {
  for command in curl sha256sum sudo tar; do
    command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 1; }
  done
  [[ "$(uname -m)" == "x86_64" ]] || { echo 'pinned CI nerdctl installer currently supports amd64 only' >&2; exit 1; }

  archive="$(mktemp -t nerdctl-${NERDCTL_VERSION}.XXXXXX.tar.gz)"
  trap 'rm -f "$archive"' EXIT
  curl --fail --silent --show-error --location \
    --proto '=https' --tlsv1.2 \
    "https://github.com/containerd/nerdctl/releases/download/v${NERDCTL_VERSION}/nerdctl-${NERDCTL_VERSION}-linux-amd64.tar.gz" \
    --output "$archive"
  printf '%s  %s\n' "$NERDCTL_AMD64_SHA256" "$archive" | sha256sum --check --strict
  sudo tar -xzf "$archive" -C /usr/local/bin nerdctl
  nerdctl --version | grep -F "${NERDCTL_VERSION}" >/dev/null
}

case "${1:-}" in
  install) install_nerdctl ;;
  *) echo "usage: $0 install" >&2; exit 2 ;;
esac

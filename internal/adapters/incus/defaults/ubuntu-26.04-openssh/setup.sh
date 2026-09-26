#!/bin/sh
set -eu

export DEBIAN_FRONTEND=noninteractive

NERDCTL_VERSION="2.3.5"
case "$(dpkg --print-architecture)" in
  amd64)
    NERDCTL_ARCH="amd64"
    NERDCTL_SHA256="b697295c623639734aaab737523c808fd3cc8d3046039fd94fff1744e4c317aa"
    ;;
  arm64)
    NERDCTL_ARCH="arm64"
    NERDCTL_SHA256="6e4b687f1d138e750a3c8372abc0f81d3d7490b6359c48c0562fc7dfe98859b2"
    ;;
  *)
    echo "unsupported architecture for nerdctl-full: $(dpkg --print-architecture)" >&2
    exit 1
    ;;
esac

apt-get update
apt-get install -y --no-install-recommends ca-certificates curl iptables openssh-server

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' 0 HUP INT TERM
archive="$tmp_dir/nerdctl-full.tar.gz"

curl --fail --location --silent --show-error --retry 3 \
  --output "$archive" \
  "https://github.com/containerd/nerdctl/releases/download/v${NERDCTL_VERSION}/nerdctl-full-${NERDCTL_VERSION}-linux-${NERDCTL_ARCH}.tar.gz"
printf '%s  %s\n' "$NERDCTL_SHA256" "$archive" | sha256sum -c -
tar -xzf "$archive" -C /usr/local

/usr/local/bin/nerdctl --version
/usr/local/bin/containerd --version
/usr/local/bin/runc --version
/usr/local/bin/buildkitd --version

systemctl enable containerd.service buildkit.service

rm -rf /var/lib/apt/lists/*

#!/bin/sh
set -eu

export DEBIAN_FRONTEND=noninteractive

NERDCTL_VERSION="2.3.5"
case "$(dpkg --print-architecture)" in
  amd64)
    NERDCTL_ARCH="amd64"
    NERDCTL_SHA256="de3206aeb7cbd5f20f5fb1f55c1e3bf2db1be567812a8a3f5e65eba2488347ee"
    ;;
  arm64)
    NERDCTL_ARCH="arm64"
    NERDCTL_SHA256="76ced9bd0d03f6140f9cf7b927958b654cb8d5ecd3c58af585d096c8bdf9d6c2"
    ;;
  *)
    echo "unsupported architecture for nerdctl: $(dpkg --print-architecture)" >&2
    exit 1
    ;;
esac

apt-get update
apt-get install -y --no-install-recommends ca-certificates curl openssh-server

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' 0 HUP INT TERM
archive="$tmp_dir/nerdctl.tar.gz"

curl --fail --location --silent --show-error --retry 3 \
  --output "$archive" \
  "https://github.com/containerd/nerdctl/releases/download/v${NERDCTL_VERSION}/nerdctl-${NERDCTL_VERSION}-linux-${NERDCTL_ARCH}.tar.gz"
printf '%s  %s\n' "$NERDCTL_SHA256" "$archive" | sha256sum -c -
tar -xzf "$archive" -C /usr/local/bin nerdctl
chmod 0755 /usr/local/bin/nerdctl
/usr/local/bin/nerdctl --version

rm -rf /var/lib/apt/lists/*

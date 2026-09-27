#!/bin/sh
set -eu

export DEBIAN_FRONTEND=noninteractive

NERDCTL_VERSION="2.3.5"
DOCKER_VERSION="28.5.2"
case "$(dpkg --print-architecture)" in
  amd64)
    NERDCTL_ARCH="amd64"
    NERDCTL_SHA256="b697295c623639734aaab737523c808fd3cc8d3046039fd94fff1744e4c317aa"
    DOCKER_ARCH="x86_64"
    DOCKER_SHA256="ea90cfd12e1eeb12aa1c971741adb8bd4ed88e2a574eaac13f5029a1dbc6300d"
    ;;
  arm64)
    NERDCTL_ARCH="arm64"
    NERDCTL_SHA256="6e4b687f1d138e750a3c8372abc0f81d3d7490b6359c48c0562fc7dfe98859b2"
    DOCKER_ARCH="aarch64"
    DOCKER_SHA256="9e4f82996ab790724094475ebed33a736434bfe5d45231b676fef22ffb80044d"
    ;;
  *)
    echo "unsupported architecture for nerdctl-full/Docker: $(dpkg --print-architecture)" >&2
    exit 1
    ;;
esac

apt-get update
apt-get install -y --no-install-recommends ca-certificates curl iptables openssh-server

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' 0 HUP INT TERM

nerdctl_archive="$tmp_dir/nerdctl-full.tar.gz"
curl --fail --location --silent --show-error --retry 3 \
  --output "$nerdctl_archive" \
  "https://github.com/containerd/nerdctl/releases/download/v${NERDCTL_VERSION}/nerdctl-full-${NERDCTL_VERSION}-linux-${NERDCTL_ARCH}.tar.gz"
printf '%s  %s\n' "$NERDCTL_SHA256" "$nerdctl_archive" | sha256sum -c -
tar -xzf "$nerdctl_archive" -C /usr/local

docker_archive="$tmp_dir/docker.tgz"
curl --fail --location --silent --show-error --retry 3 \
  --output "$docker_archive" \
  "https://download.docker.com/linux/static/stable/${DOCKER_ARCH}/docker-${DOCKER_VERSION}.tgz"
printf '%s  %s\n' "$DOCKER_SHA256" "$docker_archive" | sha256sum -c -
tar -xzf "$docker_archive" -C "$tmp_dir"

install -d -m 0755 /usr/local/lib/hacocoon/docker
install -m 0755 "$tmp_dir/docker/docker" /usr/local/bin/docker
for binary in containerd containerd-shim-runc-v2 ctr docker-init docker-proxy dockerd runc; do
  install -m 0755 "$tmp_dir/docker/$binary" "/usr/local/lib/hacocoon/docker/$binary"
done

/usr/local/bin/nerdctl --version
/usr/local/bin/containerd --version
/usr/local/bin/runc --version
/usr/local/bin/buildkitd --version
/usr/local/bin/docker --version
/usr/local/lib/hacocoon/docker/dockerd --version
/usr/local/lib/hacocoon/docker/containerd --version
/usr/local/lib/hacocoon/docker/runc --version

cat >/etc/systemd/system/docker.service <<'EOF'
[Unit]
Description=Hacocoon Docker Engine
After=network.target

[Service]
Type=notify
ExecStart=/usr/local/lib/hacocoon/docker/dockerd --storage-driver=vfs
Environment=PATH=/usr/local/lib/hacocoon/docker:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
Delegate=yes
KillMode=process
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

systemctl enable containerd.service buildkit.service docker.service

rm -rf /var/lib/apt/lists/*

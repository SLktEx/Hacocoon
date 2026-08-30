#!/bin/sh
set -eu

INSTALLER="${1:-}"
VERSION="${2:-latest}"
SKIP_INCUS="${HACO_BOOTSTRAP_SKIP_INCUS:-0}"
GRANT_INCUS_ADMIN="${HACO_BOOTSTRAP_GRANT_INCUS_ADMIN:-0}"
SYSTEMD_RESTART_REQUIRED=42
ZABBLY_INCUS_REPOSITORY="${HACO_BOOTSTRAP_INCUS_REPOSITORY:-lts-7.0}"
ZABBLY_KEY_FINGERPRINT="4EFC590696CB15B87C73A3AD82CC8797C838DCFD"

if [ -z "$INSTALLER" ] || [ ! -f "$INSTALLER" ]; then
  printf 'haco bootstrap: install script not found: %s\n' "$INSTALLER" >&2
  exit 1
fi

if ! command -v apt-get >/dev/null 2>&1; then
  printf 'haco bootstrap: automatic WSL dependency setup currently supports apt-based distributions only\n' >&2
  exit 1
fi

if [ "$(id -u)" -eq 0 ]; then
  SUDO=""
else
  command -v sudo >/dev/null 2>&1 || {
    printf 'haco bootstrap: sudo is required for dependency installation\n' >&2
    exit 1
  }
  SUDO="sudo"
fi

configure_wsl_systemd() {
  tmp="$(mktemp)"
  if [ -f /etc/wsl.conf ]; then
    awk '
      BEGIN {
        in_boot = 0
        boot_seen = 0
        systemd_seen = 0
      }
      function flush_boot() {
        if (in_boot && !systemd_seen) {
          print "systemd=true"
          systemd_seen = 1
        }
      }
      /^[[:space:]]*\[[^]]+\][[:space:]]*$/ {
        flush_boot()
        in_boot = ($0 ~ /^[[:space:]]*\[boot\][[:space:]]*$/)
        if (in_boot) {
          boot_seen = 1
          systemd_seen = 0
        }
        print
        next
      }
      {
        if (in_boot && $0 ~ /^[[:space:]]*systemd[[:space:]]*=/) {
          if (!systemd_seen) {
            print "systemd=true"
            systemd_seen = 1
          }
          next
        }
        print
      }
      END {
        flush_boot()
        if (!boot_seen) {
          if (NR > 0) {
            print ""
          }
          print "[boot]"
          print "systemd=true"
        }
      }
    ' /etc/wsl.conf > "$tmp"
  else
    printf '[boot]\nsystemd=true\n' > "$tmp"
  fi
  $SUDO install -m 0644 "$tmp" /etc/wsl.conf
  rm -f "$tmp"
}

configure_zabbly_incus_repository() {
  case "$ZABBLY_INCUS_REPOSITORY" in
    lts-7.0|stable)
      ;;
    *)
      printf 'haco bootstrap: unsupported Incus repository %s (expected lts-7.0 or stable)\n' "$ZABBLY_INCUS_REPOSITORY" >&2
      exit 1
      ;;
  esac

  key="$(mktemp)"
  curl --fail --silent --show-error --location \
    --proto '=https' --tlsv1.2 \
    https://pkgs.zabbly.com/key.asc \
    --output "$key"
  actual_fingerprint="$(gpg --show-keys --with-colons "$key" 2>/dev/null | awk -F: '$1 == "fpr" { print $10; exit }')"
  if [ "$actual_fingerprint" != "$ZABBLY_KEY_FINGERPRINT" ]; then
    rm -f "$key"
    printf 'haco bootstrap: Zabbly repository signing-key fingerprint mismatch\n' >&2
    exit 1
  fi

  $SUDO install -d -m 0755 /etc/apt/keyrings
  $SUDO install -m 0644 "$key" /etc/apt/keyrings/zabbly-incus.asc
  rm -f "$key"

  codename="$(. /etc/os-release && printf '%s' "${VERSION_CODENAME:-}")"
  architecture="$(dpkg --print-architecture)"
  if [ -z "$codename" ] || [ -z "$architecture" ]; then
    printf 'haco bootstrap: could not determine apt distribution codename/architecture for Incus\n' >&2
    exit 1
  fi

  source_tmp="$(mktemp)"
  cat >"$source_tmp" <<EOF
Enabled: yes
Types: deb
URIs: https://pkgs.zabbly.com/incus/$ZABBLY_INCUS_REPOSITORY
Suites: $codename
Components: main
Architectures: $architecture
Signed-By: /etc/apt/keyrings/zabbly-incus.asc
EOF
  $SUDO install -m 0644 "$source_tmp" /etc/apt/sources.list.d/zabbly-incus-hacocoon.sources
  rm -f "$source_tmp"
}

ensure_bridge_netfilter() {
  if [ ! -e /proc/sys/net/bridge/bridge-nf-call-iptables ] || [ ! -e /proc/sys/net/bridge/bridge-nf-call-ip6tables ]; then
    printf '==> Loading bridge netfilter required by the Hacocoon sandbox\n'
    $SUDO modprobe br_netfilter || {
      printf 'haco bootstrap: failed to load br_netfilter required by Incus bridge filtering\n' >&2
      exit 1
    }
  fi

  if [ ! -e /proc/sys/net/bridge/bridge-nf-call-iptables ] || [ ! -e /proc/sys/net/bridge/bridge-nf-call-ip6tables ]; then
    printf 'haco bootstrap: bridge netfilter hooks are unavailable after loading br_netfilter\n' >&2
    exit 1
  fi
}

printf '==> Installing base dependencies and systemd support\n'
$SUDO apt-get update
$SUDO apt-get install -y ca-certificates curl gnupg kmod tar git systemd systemd-sysv

printf '==> Enabling systemd for this WSL distribution\n'
configure_wsl_systemd

pid1="$(ps -p 1 -o comm= 2>/dev/null | tr -d '[:space:]' || true)"
if [ "$pid1" != "systemd" ]; then
  printf 'haco bootstrap: systemd configuration is ready; the dedicated WSL distribution must be restarted\n' >&2
  exit "$SYSTEMD_RESTART_REQUIRED"
fi

printf '==> systemd is active as PID 1\n'

if [ "$SKIP_INCUS" != "1" ]; then
  printf '==> Configuring verified Incus %s repository\n' "$ZABBLY_INCUS_REPOSITORY"
  configure_zabbly_incus_repository
  $SUDO apt-get update

  printf '==> Installing Incus\n'
  $SUDO apt-get install -y incus

  ensure_bridge_netfilter

  if ! systemctl is-system-running >/dev/null 2>&1; then
    state="$(systemctl is-system-running 2>/dev/null || true)"
    if [ "$state" != "degraded" ]; then
      printf 'haco bootstrap: systemd is PID 1 but not operational (state: %s)\n' "$state" >&2
      exit 1
    fi
  fi

  $SUDO systemctl enable --now incus.service 2>/dev/null || $SUDO systemctl enable --now incus 2>/dev/null || {
    printf 'haco bootstrap: failed to enable/start Incus with systemd\n' >&2
    exit 1
  }

  if [ "$GRANT_INCUS_ADMIN" = "1" ] && [ "$(id -u)" -ne 0 ]; then
    if getent group incus-admin >/dev/null 2>&1; then
      printf '==> Granting current Linux user incus-admin access\n'
      printf 'haco bootstrap: warning: incus-admin is root-equivalent local authority\n' >&2
      $SUDO usermod -aG incus-admin "$(id -un)"
    else
      printf 'haco bootstrap: warning: incus-admin group does not exist after package installation\n' >&2
    fi
  fi

  if command -v incus >/dev/null 2>&1 && $SUDO incus info >/dev/null 2>&1; then
    if ! $SUDO incus storage list --format csv -c n 2>/dev/null | grep -q .; then
      printf '==> Initializing Incus with a minimal configuration\n'
      $SUDO incus admin init --minimal
    fi
  else
    printf 'haco bootstrap: Incus daemon is not ready after systemd startup\n' >&2
    exit 1
  fi
fi

printf '==> Installing Hacocoon release\n'
sh "$INSTALLER" "$VERSION"

printf '==> Installed binaries\n'
command -v haco || true
command -v haco-vscode || true

if [ "$SKIP_INCUS" != "1" ] && [ "$GRANT_INCUS_ADMIN" = "1" ] && [ "$(id -u)" -ne 0 ]; then
  printf '%s\n' 'haco bootstrap: restart the WSL shell (or use newgrp incus-admin) before relying on the new group membership.'
fi

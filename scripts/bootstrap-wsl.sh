#!/bin/sh
set -eu

INSTALLER="${1:-}"
VERSION="${2:-latest}"
SKIP_INCUS="${HACO_BOOTSTRAP_SKIP_INCUS:-0}"
GRANT_INCUS_ADMIN="${HACO_BOOTSTRAP_GRANT_INCUS_ADMIN:-0}"
LOGIN_USER="${HACO_BOOTSTRAP_LOGIN_USER:-$(id -un)}"
SYSTEMD_RESTART_REQUIRED=42
HACOCOON_LOGIN_SHELL="/usr/local/libexec/hacocoon-login"
HACOCOON_CONTROLLER_SERVICE="haco-controller.service"
HACOCOON_CONTROLLER_SOCKET="/run/hacocoon/control.sock"

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

configure_hacocoon_controller() {
  controller_bin="$1"

  case "$controller_bin" in
    /usr/local/bin/haco-controller|/usr/bin/haco-controller) ;;
    *)
      printf 'haco bootstrap: controller service requires a system-owned haco-controller at /usr/local/bin or /usr/bin (got %s)\n' "$controller_bin" >&2
      return 1
      ;;
  esac
  owner="$($SUDO stat -Lc '%u' "$controller_bin")"
  if [ "$owner" != "0" ]; then
    printf 'haco bootstrap: refusing controller service through non-root-owned binary: %s\n' "$controller_bin" >&2
    return 1
  fi
  if $SUDO find "$controller_bin" -perm /022 -print -quit | grep -q .; then
    printf 'haco bootstrap: refusing controller service through group/world-writable binary: %s\n' "$controller_bin" >&2
    return 1
  fi

  printf '==> Configuring Physical Host Hacocoon controller service\n'
  unit_tmp="$(mktemp)"
  cat > "$unit_tmp" <<EOF_UNIT
[Unit]
Description=Hacocoon Physical Host controller
Requires=incus.service
After=incus.service

[Service]
Type=simple
ExecStart=$controller_bin
Restart=on-failure
RestartSec=1s
RuntimeDirectory=hacocoon
RuntimeDirectoryMode=0700
UMask=0077
Environment=HACO_ROOT=/var/lib/hacocoon

[Install]
WantedBy=multi-user.target
EOF_UNIT
  $SUDO install -o root -g root -m 0644 "$unit_tmp" "/etc/systemd/system/$HACOCOON_CONTROLLER_SERVICE"
  rm -f "$unit_tmp"

  $SUDO systemctl daemon-reload
  $SUDO systemctl enable "$HACOCOON_CONTROLLER_SERVICE" >/dev/null
  $SUDO systemctl restart "$HACOCOON_CONTROLLER_SERVICE"

  attempts=0
  while [ "$attempts" -lt 100 ]; do
    if $SUDO test -S "$HACOCOON_CONTROLLER_SOCKET"; then
      break
    fi
    if ! $SUDO systemctl is-active --quiet "$HACOCOON_CONTROLLER_SERVICE"; then
      $SUDO systemctl status "$HACOCOON_CONTROLLER_SERVICE" --no-pager >&2 || true
      printf 'haco bootstrap: Physical Host controller service exited before creating its socket\n' >&2
      return 1
    fi
    attempts=$((attempts + 1))
    sleep 0.05
  done
  if ! $SUDO test -S "$HACOCOON_CONTROLLER_SOCKET"; then
    $SUDO systemctl status "$HACOCOON_CONTROLLER_SERVICE" --no-pager >&2 || true
    printf 'haco bootstrap: controller did not create %s\n' "$HACOCOON_CONTROLLER_SOCKET" >&2
    return 1
  fi

  socket_state="$($SUDO stat -Lc '%u:%g:%a' "$HACOCOON_CONTROLLER_SOCKET")"
  if [ "$socket_state" != "0:0:600" ]; then
    printf 'haco bootstrap: unsafe controller socket ownership/mode: %s (want 0:0:600)\n' "$socket_state" >&2
    return 1
  fi
}

configure_hacocoon_login() {
  haco_bin="$1"

  if [ "$LOGIN_USER" = "root" ]; then
    printf 'haco bootstrap: refusing to replace root login shell; configure a non-root WSL default user first\n' >&2
    return 1
  fi
  case "$LOGIN_USER" in
    ''|*[!A-Za-z0-9._-]*)
      printf 'haco bootstrap: unsupported WSL login user name: %s\n' "$LOGIN_USER" >&2
      return 1
      ;;
  esac
  if ! id "$LOGIN_USER" >/dev/null 2>&1; then
    printf 'haco bootstrap: WSL login user does not exist: %s\n' "$LOGIN_USER" >&2
    return 1
  fi

  case "$haco_bin" in
    /usr/local/bin/haco|/usr/bin/haco) ;;
    *)
      printf 'haco bootstrap: automatic WSL login requires a system-owned haco at /usr/local/bin/haco or /usr/bin/haco (got %s)\n' "$haco_bin" >&2
      return 1
      ;;
  esac
  owner="$($SUDO stat -Lc '%u' "$haco_bin")"
  if [ "$owner" != "0" ]; then
    printf 'haco bootstrap: refusing passwordless host entry through non-root-owned haco binary: %s\n' "$haco_bin" >&2
    return 1
  fi
  if $SUDO find "$haco_bin" -perm /022 -print -quit | grep -q .; then
    printf 'haco bootstrap: refusing passwordless host entry through group/world-writable haco binary: %s\n' "$haco_bin" >&2
    return 1
  fi

  printf '==> Configuring default WSL login to enter haco-host\n'
  $SUDO mkdir -p /usr/local/libexec
  $SUDO ln -sfn "$haco_bin" "$HACOCOON_LOGIN_SHELL"
  if ! grep -Fx "$HACOCOON_LOGIN_SHELL" /etc/shells >/dev/null 2>&1; then
    printf '%s\n' "$HACOCOON_LOGIN_SHELL" | $SUDO tee -a /etc/shells >/dev/null
  fi

  sudoers_tmp="$(mktemp)"
  printf '%s ALL=(root) NOPASSWD: %s host ensure, %s host shell\n' "$LOGIN_USER" "$haco_bin" "$haco_bin" > "$sudoers_tmp"
  $SUDO visudo -cf "$sudoers_tmp" >/dev/null
  $SUDO install -m 0440 "$sudoers_tmp" /etc/sudoers.d/hacocoon-login
  rm -f "$sudoers_tmp"

  $SUDO usermod -s "$HACOCOON_LOGIN_SHELL" "$LOGIN_USER"
}

printf '==> Installing base dependencies, managed Btrfs tools, and systemd support\n'
$SUDO apt-get update
$SUDO apt-get install -y ca-certificates curl tar git sudo systemd systemd-sysv btrfs-progs util-linux

printf '==> Enabling systemd for this WSL distribution\n'
configure_wsl_systemd

pid1="$(ps -p 1 -o comm= 2>/dev/null | tr -d '[:space:]' || true)"
if [ "$pid1" != "systemd" ]; then
  printf 'haco bootstrap: systemd configuration is ready; the dedicated WSL distribution must be restarted\n' >&2
  exit "$SYSTEMD_RESTART_REQUIRED"
fi

printf '==> systemd is active as PID 1\n'

if [ "$SKIP_INCUS" != "1" ]; then
  printf '==> Installing Incus\n'
  $SUDO apt-get install -y incus

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
command -v haco-controller || true
command -v haco-host || true
command -v haco-vscode || true
printf '%s\n' '/usr/local/libexec/hacocoon/haco-storage-helper'

if [ "$SKIP_INCUS" != "1" ]; then
  haco_bin="$(command -v haco || true)"
  controller_bin="$(command -v haco-controller || true)"
  if [ -z "$haco_bin" ] || [ -z "$controller_bin" ]; then
    printf 'haco bootstrap: haco or haco-controller binary is unavailable after installation\n' >&2
    exit 1
  fi
  haco_bin="$(readlink -f "$haco_bin")"
  controller_bin="$(readlink -f "$controller_bin")"

  configure_hacocoon_controller "$controller_bin" || {
    distro="${WSL_DISTRO_NAME:-Hacocoon}"
    printf 'haco bootstrap: failed to prepare Physical Host controller; default WSL login was not changed\n' >&2
    printf 'haco bootstrap: recover on the Physical Host with: wsl -d %s -u root\n' "$distro" >&2
    exit 1
  }

  printf '==> Reconciling trusted haco-host and controller endpoint\n'
  $SUDO "$haco_bin" host ensure || {
    distro="${WSL_DISTRO_NAME:-Hacocoon}"
    printf 'haco bootstrap: failed to prepare haco-host; default WSL login was not changed\n' >&2
    printf 'haco bootstrap: recover on the Physical Host with: wsl -d %s -u root\n' "$distro" >&2
    exit 1
  }

  printf '==> Verifying trusted haco-host controller round trip\n'
  $SUDO incus exec haco-host --project hacocoon -- /usr/local/bin/haco-host doctor >/dev/null || {
    distro="${WSL_DISTRO_NAME:-Hacocoon}"
    printf 'haco bootstrap: haco-host cannot reach the Physical Host controller\n' >&2
    printf 'haco bootstrap: recover on the Physical Host with: wsl -d %s -u root\n' "$distro" >&2
    exit 1
  }

  configure_hacocoon_login "$haco_bin"
else
  printf '%s\n' 'haco bootstrap: -SkipIncus leaves the Physical Host login unchanged; haco-host auto-entry requires a ready Incus backend.'
fi

if [ "$SKIP_INCUS" != "1" ] && [ "$GRANT_INCUS_ADMIN" = "1" ] && [ "$(id -u)" -ne 0 ]; then
  printf '%s\n' 'haco bootstrap: restart the WSL shell (or use newgrp incus-admin) before relying on the new group membership.'
fi

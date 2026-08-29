#!/bin/sh
set -eu

INSTALLER="${1:-}"
VERSION="${2:-latest}"
SKIP_INCUS="${HACO_BOOTSTRAP_SKIP_INCUS:-0}"
GRANT_INCUS_ADMIN="${HACO_BOOTSTRAP_GRANT_INCUS_ADMIN:-0}"

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

printf '==> Installing base dependencies\n'
$SUDO apt-get update
$SUDO apt-get install -y ca-certificates curl tar git

if [ "$SKIP_INCUS" != "1" ]; then
  printf '==> Installing Incus\n'
  $SUDO apt-get install -y incus

  if command -v systemctl >/dev/null 2>&1; then
    if systemctl is-system-running >/dev/null 2>&1 || systemctl is-system-running 2>/dev/null | grep -Eq 'degraded|running'; then
      $SUDO systemctl enable --now incus.service 2>/dev/null || $SUDO systemctl enable --now incus 2>/dev/null || true
    else
      printf 'haco bootstrap: warning: systemd is not active in this WSL distribution; Incus may not start until systemd is enabled\n' >&2
    fi
  fi

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
    printf 'haco bootstrap: warning: Incus daemon is not ready yet; Hacocoon can still be installed, but local runtime use requires a working Incus daemon\n' >&2
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

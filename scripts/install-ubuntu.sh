#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
VERSION="${1:-${HACO_VERSION:-latest}}"

fail() {
  printf 'haco ubuntu installer: %s\n' "$*" >&2
  exit 1
}

is_wsl() {
  [ -n "${WSL_DISTRO_NAME:-}" ] && return 0
  kernel_release="$(uname -r 2>/dev/null || true)"
  case "$kernel_release" in
    *[Mm]icrosoft*|*[Ww][Ss][Ll]*) return 0 ;;
  esac
  return 1
}

# pre: native Ubuntu only. WSL enters through install-windows.ps1 so its WSL
# lifecycle and post-install login integration remain environment-specific.
is_wsl && fail "this package is for native Ubuntu; use the Windows installer for WSL"
[ -r /etc/os-release ] || fail "/etc/os-release is unavailable"
# shellcheck disable=SC1091
. /etc/os-release
[ "${ID:-}" = "ubuntu" ] || fail "only Ubuntu is supported (got ${ID:-unknown})"
command -v dpkg >/dev/null 2>&1 || fail "dpkg is required"
dpkg --compare-versions "${VERSION_ID:-0}" ge 26.04 ||
  fail "Ubuntu 26.04 or newer is required (got ${VERSION_ID:-unknown})"

pid1="$(ps -p 1 -o comm= 2>/dev/null | tr -d '[:space:]' || true)"
[ "$pid1" = "systemd" ] || fail "systemd must be active as PID 1"

if [ "$(id -u)" -ne 0 ]; then
  command -v sudo >/dev/null 2>&1 || fail "sudo is required"
  sudo -v || fail "sudo authorization is required"
fi

# main: exactly the same Ubuntu installation logic used inside WSL.
sh "$SCRIPT_DIR/install.sh" "$VERSION"

# post: native Ubuntu deliberately keeps the user's login shell unchanged.
printf '%s\n' 'Hacocoon native Ubuntu setup complete.'
if [ "${HACO_BOOTSTRAP_SKIP_INCUS:-0}" = "1" ]; then
  printf '%s\n' 'Incus setup was skipped; trusted haco-host entry is not available.'
else
  command -v haco >/dev/null 2>&1 || fail "haco is unavailable after common installation"
  if [ "$(id -u)" -eq 0 ]; then
    systemctl is-active --quiet haco-controller.service || fail "haco-controller.service is not active"
  else
    sudo systemctl is-active --quiet haco-controller.service || fail "haco-controller.service is not active"
  fi
  printf '%s\n' 'Enter the trusted Host explicitly with: sudo haco host shell'
fi

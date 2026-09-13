#!/bin/sh
# Shared Ubuntu/WSL installer and disposable CI package contract.
set -eu

INCUS_LTS_SERIES=7.0
INCUS_LTS_MINIMUM=7.0.1
INCUS_LTS_LIMIT=7.1
INCUS_PACKAGE_URI=https://pkgs.zabbly.com/incus/lts-7.0
INCUS_SIGNING_FPR=4EFC590696CB15B87C73A3AD82CC8797C838DCFD

fail() { printf 'Incus LTS: %s\n' "$*" >&2; exit 1; }

verify_version() {
  # Never echo an arbitrary backend value into the user's terminal.
  case "$1" in ''|*[!0-9.]*) fail 'cannot verify the Incus server version; expected a numeric release version' ;; esac
  [ "${#1}" -le 32 ] && printf '%s\n' "$1" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' ||
    fail 'cannot verify the Incus server version; expected a numeric release version'
  dpkg --compare-versions "$1" ge "$INCUS_LTS_MINIMUM" &&
    dpkg --compare-versions "$1" lt "$INCUS_LTS_LIMIT" ||
    fail "unsupported Incus $1; install the current 7.0 LTS package (>= 7.0.1, < 7.1)"
  printf 'Incus %s: supported 7.0 LTS server\n' "$1"
}

install_lts() (
  [ "$(id -u)" = 0 ] || fail 'package installation requires root'
  . /etc/os-release
  [ "${ID:-}" = ubuntu ] || fail 'the supported package path requires Ubuntu'
  dpkg --compare-versions "${VERSION_ID:-0}" ge 26.04 || fail 'Ubuntu 26.04 or newer is required'
  case "${VERSION_CODENAME:-}" in ''|*[!a-z0-9]*) fail 'invalid Ubuntu release codename' ;; esac
  architecture="$(dpkg --print-architecture)"
  case "$architecture" in amd64|arm64) ;; *) fail 'unsupported package architecture' ;; esac

  # A newer existing series needs an explicit migration, never an automatic downgrade.
  installed="$(dpkg-query -W -f='${Version}' incus-base 2>/dev/null || true)"
  # Packaging epochs order package replacements, not Incus data compatibility.
  # A distro's unepoched 7.1 is newer than 7.0 even though dpkg orders it below 1:7.0.
  if [ -n "$installed" ] && dpkg --compare-versions "${installed#*:}" ge 7.1; then
    fail 'an Incus series newer than 7.0 is installed; preserve its data and plan migration explicitly'
  fi

  temporary="$(mktemp -d)"
  trap 'rm -f -- "$temporary/key.asc" "$temporary/sources" "$temporary/preferences"; rmdir -- "$temporary"' EXIT
  trap 'exit 1' HUP INT TERM
  curl -fsSL --proto '=https' --tlsv1.2 --connect-timeout 15 --max-time 60 \
    https://pkgs.zabbly.com/key.asc -o "$temporary/key.asc"
  # Every primary key must be the pinned key; checking only the first key can
  # accidentally trust extra keys from a malformed download.
  keys="$(gpg --batch --show-keys --with-colons --fingerprint "$temporary/key.asc" 2>/dev/null)" ||
    fail 'cannot read the downloaded signing key'
  fingerprints="$(printf '%s\n' "$keys" | awk -F: '
    $1 == "pub" { primary = 1; next }
    primary && $1 == "fpr" { print $10; primary = 0 }
  ')"
  [ "$fingerprints" = "$INCUS_SIGNING_FPR" ] || fail 'the downloaded signing key does not match the trusted Zabbly key'

  cat >"$temporary/sources" <<EOF
Enabled: yes
Types: deb
URIs: $INCUS_PACKAGE_URI
Suites: $VERSION_CODENAME
Components: main
Architectures: $architecture
Signed-By: /etc/apt/keyrings/zabbly.asc
EOF
  # Track all 7.0 patches. Other series are excluded from ordinary apt updates.
  cat >"$temporary/preferences" <<'EOF'
Package: incus incus-base incus-client
Pin: version 1:7.0.*
Pin-Priority: 990

Package: incus incus-base incus-client
Pin: version *
Pin-Priority: -1
EOF
  install -d -m 0755 /etc/apt/keyrings /etc/apt/sources.list.d /etc/apt/preferences.d
  install -o root -g root -m 0644 "$temporary/key.asc" /etc/apt/keyrings/zabbly.asc
  install -o root -g root -m 0644 "$temporary/sources" /etc/apt/sources.list.d/zabbly-incus-lts-7.0.sources
  install -o root -g root -m 0644 "$temporary/preferences" /etc/apt/preferences.d/hacocoon-incus-lts
  apt-get update

  # Select the greatest currently available version from the exact signed
  # source. This per-install selection is not a persisted patch-level pin.
  packages="$(apt-cache madison incus-base)"
  versions="$(printf '%s\n' "$packages" | awk -F'|' -v source="$INCUS_PACKAGE_URI" '
    { gsub(/^[ \t]+|[ \t]+$/, "", $2); gsub(/^[ \t]+/, "", $3) }
    index($3, source " ") == 1 { print $2 }
  ')"
  selected=''
  for version in $versions; do
    printf '%s\n' "$version" | grep -Eq '^1:7\.0\.[0-9]+-[0-9A-Za-z.+~_-]+$' || continue
    dpkg --compare-versions "$version" ge 1:7.0.1 || continue
    dpkg --compare-versions "$version" lt 1:7.1 || continue
    if [ -z "$selected" ] || dpkg --compare-versions "$version" gt "$selected"; then selected="$version"; fi
  done
  [ -n "$selected" ] || fail 'no supported 7.0 LTS package is available from the signed source'
  # Container-only substrate; do not pull the optional VM dependency stack.
  apt-get install --yes --no-install-recommends "incus-base=$selected" "incus-client=$selected"
)

case "${1:-}" in
  install) [ "$#" = 1 ] || fail 'unexpected install arguments'; install_lts ;;
  verify-version) [ "$#" = 2 ] || fail 'one server version is required'; verify_version "$2" ;;
  *) fail 'usage: incus-lts.sh install | verify-version <server-version>' ;;
esac

#!/bin/sh
set -eu

REPOSITORY="SLktEx/Hacocoon"
INSTALL_DIR="${HACO_INSTALL_DIR:-/usr/local/bin}"
VERSION="${1:-${HACO_VERSION:-latest}}"

die() {
  printf 'haco installer: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

need uname
need tar
need sha256sum
need awk
need mktemp
need cp
need chmod

case "$(uname -s)" in
  Linux) os="linux" ;;
  *) die "unsupported operating system: $(uname -s) (Hacocoon releases currently target Linux only)" ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac

if [ "$VERSION" != "latest" ]; then
  need grep
  printf '%s\n' "$VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$' ||
    die "invalid version: $VERSION"
fi

archive="haco_${os}_${arch}.tar.gz"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

download_with_gh() {
  tag="$1"
  if [ "$tag" = "latest" ]; then
    gh release download \
      --repo "$REPOSITORY" \
      --pattern "$archive" \
      --pattern checksums.txt \
      --dir "$tmpdir"
  else
    gh release download "$tag" \
      --repo "$REPOSITORY" \
      --pattern "$archive" \
      --pattern checksums.txt \
      --dir "$tmpdir"
  fi
}

download_with_curl() {
  tag="$1"
  if [ "$tag" = "latest" ]; then
    base="https://github.com/$REPOSITORY/releases/latest/download"
  else
    base="https://github.com/$REPOSITORY/releases/download/$tag"
  fi

  token="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
  if [ -n "$token" ]; then
    curl -fL --proto '=https' --tlsv1.2 \
      -H "Authorization: Bearer $token" \
      -o "$tmpdir/$archive" "$base/$archive"
    curl -fL --proto '=https' --tlsv1.2 \
      -H "Authorization: Bearer $token" \
      -o "$tmpdir/checksums.txt" "$base/checksums.txt"
  else
    curl -fL --proto '=https' --tlsv1.2 \
      -o "$tmpdir/$archive" "$base/$archive"
    curl -fL --proto '=https' --tlsv1.2 \
      -o "$tmpdir/checksums.txt" "$base/checksums.txt"
  fi
}

if command -v gh >/dev/null 2>&1 && { [ -n "${GH_TOKEN:-}" ] || [ -n "${GITHUB_TOKEN:-}" ] || gh auth status >/dev/null 2>&1; }; then
  download_with_gh "$VERSION" || die "failed to download release assets with gh"
else
  need curl
  download_with_curl "$VERSION" ||
    die "failed to download release assets; private repositories require authenticated GitHub access"
fi

[ -s "$tmpdir/$archive" ] || die "downloaded archive is empty"
[ -s "$tmpdir/checksums.txt" ] || die "downloaded checksums file is empty"

expected="$(
  awk -v name="$archive" '
    $2 == name || $2 == "*" name {
      print $1
      exit
    }
  ' "$tmpdir/checksums.txt"
)"
[ -n "$expected" ] || die "checksum for $archive not found"

actual="$(sha256sum "$tmpdir/$archive" | awk '{print $1}')"
[ "$actual" = "$expected" ] || die "checksum verification failed for $archive"

tar -xzf "$tmpdir/$archive" -C "$tmpdir"
[ -f "$tmpdir/haco" ] || die "release archive does not contain haco"
[ -x "$tmpdir/haco" ] || chmod 0755 "$tmpdir/haco"

install_target="$INSTALL_DIR/haco"
if [ -d "$INSTALL_DIR" ] && [ -w "$INSTALL_DIR" ]; then
  cp "$tmpdir/haco" "$install_target"
  chmod 0755 "$install_target"
elif command -v sudo >/dev/null 2>&1; then
  sudo mkdir -p "$INSTALL_DIR"
  sudo cp "$tmpdir/haco" "$install_target"
  sudo chmod 0755 "$install_target"
else
  die "cannot write to $INSTALL_DIR; set HACO_INSTALL_DIR to a writable directory or install sudo"
fi

printf 'Installed haco to %s\n' "$install_target"

#!/bin/sh
set -eu

REPOSITORY="SLktEx/Hacocoon"
SIGNER_WORKFLOW="$REPOSITORY/.github/workflows/release.yml"
SOURCE_REF="refs/heads/main"
INSTALL_DIR="${HACO_INSTALL_DIR:-/usr/local/bin}"
VERSION="${1:-${HACO_VERSION:-latest}}"
REQUIRE_PROVENANCE="${HACO_REQUIRE_PROVENANCE:-0}"

die() {
  printf 'haco installer: %s\n' "$*" >&2
  exit 1
}

warn() {
  printf 'haco installer: WARNING: %s\n' "$*" >&2
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

case "$REQUIRE_PROVENANCE" in
  0|1) ;;
  *) die "HACO_REQUIRE_PROVENANCE must be 0 or 1" ;;
esac

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

has_authenticated_gh() {
  command -v gh >/dev/null 2>&1 &&
    { [ -n "${GH_TOKEN:-}" ] || [ -n "${GITHUB_TOKEN:-}" ] || gh auth status >/dev/null 2>&1; }
}

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

verify_provenance() {
  if ! command -v gh >/dev/null 2>&1 || ! gh attestation verify --help >/dev/null 2>&1; then
    if [ "$REQUIRE_PROVENANCE" = "1" ]; then
      die "HACO_REQUIRE_PROVENANCE=1 requires a GitHub CLI version with 'gh attestation verify' support"
    fi
    warn "SHA-256 integrity verified, but provenance was not verified; install a current GitHub CLI and run 'gh attestation verify'"
    return 0
  fi

  release_tag="$VERSION"
  if [ "$release_tag" = "latest" ]; then
    if ! release_tag="$(gh release view --repo "$REPOSITORY" --json tagName --jq .tagName 2>/dev/null)" || [ -z "$release_tag" ]; then
      if [ "$REQUIRE_PROVENANCE" = "1" ]; then
        die "unable to resolve the latest release tag for provenance verification"
      fi
      warn "SHA-256 integrity verified, but the latest release tag could not be resolved for provenance verification"
      return 0
    fi
  fi

  if ! source_sha="$(gh api "repos/$REPOSITORY/commits/$release_tag" --jq .sha 2>/dev/null)" || [ -z "$source_sha" ]; then
    if [ "$REQUIRE_PROVENANCE" = "1" ]; then
      die "unable to resolve source commit for release $release_tag"
    fi
    warn "SHA-256 integrity verified, but release source commit could not be resolved for provenance verification"
    return 0
  fi

  if gh attestation verify "$tmpdir/$archive" \
    --repo "$REPOSITORY" \
    --signer-workflow "$SIGNER_WORKFLOW" \
    --source-ref "$SOURCE_REF" \
    --source-digest "$source_sha" \
    --deny-self-hosted-runners >/dev/null; then
    printf 'Verified GitHub/Sigstore provenance for %s from %s at %s.\n' "$archive" "$SOURCE_REF" "$source_sha"
    return 0
  fi

  if [ "$REQUIRE_PROVENANCE" = "1" ]; then
    die "artifact provenance verification failed for $archive"
  fi
  warn "SHA-256 integrity verified, but GitHub/Sigstore provenance verification failed or is unavailable for this release"
}

if has_authenticated_gh; then
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
printf 'Verified SHA-256 integrity for %s against checksums.txt.\n' "$archive"

# checksums.txt and the archive share GitHub Release authority. The attestation
# check below is the independent publisher/workflow provenance layer.
verify_provenance

tar -xzf "$tmpdir/$archive" -C "$tmpdir"
for binary in haco haco-vscode; do
  [ -f "$tmpdir/$binary" ] || die "release archive does not contain $binary"
  [ -x "$tmpdir/$binary" ] || chmod 0755 "$tmpdir/$binary"
done

install_binary() {
  binary="$1"
  install_target="$INSTALL_DIR/$binary"
  if [ -d "$INSTALL_DIR" ] && [ -w "$INSTALL_DIR" ]; then
    cp "$tmpdir/$binary" "$install_target"
    chmod 0755 "$install_target"
  elif command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "$INSTALL_DIR"
    sudo cp "$tmpdir/$binary" "$install_target"
    sudo chmod 0755 "$install_target"
  else
    die "cannot write to $INSTALL_DIR; set HACO_INSTALL_DIR to a writable directory or install sudo"
  fi
  printf 'Installed %s to %s\n' "$binary" "$install_target"
}

install_binary haco
install_binary haco-vscode

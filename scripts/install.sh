#!/bin/sh
set -eu

REPOSITORY="SLktEx/Hacocoon"
SIGNER_WORKFLOW="$REPOSITORY/.github/workflows/release.yml"
SIGNER_SOURCE_REF="refs/heads/main"
RELEASE_PREDICATE_TYPE="https://hacocoon.dev/attestations/release/v1"
INSTALL_DIR="${HACO_INSTALL_DIR:-/usr/local/bin}"
STORAGE_HELPER_DIR="${HACO_STORAGE_HELPER_INSTALL_DIR:-/usr/local/libexec/hacocoon}"
STORAGE_HELPER_PATH="$STORAGE_HELPER_DIR/haco-storage-helper"
DEFAULT_HACO_ROOT="/var/lib/hacocoon"
VERSION="${1:-${HACO_VERSION:-latest}}"
REQUIRE_PROVENANCE="${HACO_REQUIRE_PROVENANCE:-1}"

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

validate_version() {
  candidate="$1"
  need grep
  printf '%s\n' "$candidate" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$' ||
    die "invalid version: $candidate"
}

case "$REQUIRE_PROVENANCE" in
  0|1) ;;
  *) die "HACO_REQUIRE_PROVENANCE must be 0 or 1" ;;
esac

case "$STORAGE_HELPER_DIR" in
  /*) ;;
  *) die "HACO_STORAGE_HELPER_INSTALL_DIR must be an absolute path" ;;
esac

need uname
need tar
need sha256sum
need awk
need mktemp
need mkdir
need cp
need chmod
need chown
need id

case "$(uname -s)" in
  Linux) os="linux" ;;
  *) die "unsupported operating system: $(uname -s) (Hacocoon releases currently target Linux only)" ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac

has_authenticated_gh() {
  command -v gh >/dev/null 2>&1 &&
    { [ -n "${GH_TOKEN:-}" ] || [ -n "${GITHUB_TOKEN:-}" ] || gh auth status >/dev/null 2>&1; }
}

resolve_latest_version() {
  need curl
  latest_url="$(
    curl -fsSL --proto '=https' --tlsv1.2 \
      -o /dev/null \
      -w '%{url_effective}' \
      "https://github.com/$REPOSITORY/releases/latest"
  )" || die "failed to resolve latest release"
  latest_tag="${latest_url##*/}"
  validate_version "$latest_tag"
  printf '%s\n' "$latest_tag"
}

if [ "$VERSION" = "latest" ]; then
  VERSION="$(resolve_latest_version)"
  printf 'Resolved latest Hacocoon release to %s.\n' "$VERSION"
else
  validate_version "$VERSION"
fi

archive="haco_${os}_${arch}.tar.gz"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

download_with_gh() {
  tag="$1"
  gh release download "$tag" \
    --repo "$REPOSITORY" \
    --pattern "$archive" \
    --pattern checksums.txt \
    --dir "$tmpdir"
}

download_with_curl() {
  tag="$1"
  base="https://github.com/$REPOSITORY/releases/download/$tag"

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
      die "trusted provenance verification requires a GitHub CLI version with 'gh attestation verify' support"
    fi
    warn "provenance verification was explicitly disabled with HACO_REQUIRE_PROVENANCE=0"
    return 0
  fi

  if ! gh attestation verify "$tmpdir/$archive" \
    --repo "$REPOSITORY" \
    --signer-workflow "$SIGNER_WORKFLOW" \
    --source-ref "$SIGNER_SOURCE_REF" \
    --deny-self-hosted-runners >/dev/null; then
    if [ "$REQUIRE_PROVENANCE" = "1" ]; then
      die "trusted build provenance verification failed for $archive"
    fi
    warn "trusted build provenance verification failed, but HACO_REQUIRE_PROVENANCE=0 explicitly allows continuing"
    return 0
  fi

  printf 'Verified GitHub/Sigstore provenance for %s from trusted main release workflow.\n' "$archive"

  binding_tags="$(
    gh attestation verify "$tmpdir/$archive" \
      --repo "$REPOSITORY" \
      --signer-workflow "$SIGNER_WORKFLOW" \
      --source-ref "$SIGNER_SOURCE_REF" \
      --predicate-type "$RELEASE_PREDICATE_TYPE" \
      --deny-self-hosted-runners \
      --format json \
      --jq '.[].verificationResult.statement.predicate.tag' 2>/dev/null || true
  )"

  if printf '%s\n' "$binding_tags" | grep -Fx "$VERSION" >/dev/null 2>&1; then
    printf 'Verified signed release binding for %s.\n' "$VERSION"
    return 0
  fi

  if [ "$REQUIRE_PROVENANCE" = "1" ]; then
    die "signed release binding verification failed for $VERSION"
  fi
  warn "signed release binding verification failed, but HACO_REQUIRE_PROVENANCE=0 explicitly allows continuing"
}

validate_release_archive() {
  archive_path="$1"

  archive_names="$(tar -tzf "$archive_path")" || die "release archive cannot be listed safely"
  if ! printf '%s\n' "$archive_names" | awk '
    $0 == "haco" { haco++ }
    $0 == "haco-controller" { controller++ }
    $0 == "haco-host" { hacohost++ }
    $0 == "haco-vscode" { vscode++ }
    $0 == "haco-agent-host" { agenthost++ }
    $0 == "haco-notify" { notify++ }
    $0 == "haco-storage-helper" { storagehelper++ }
    { count++ }
    END { exit !(count == 7 && haco == 1 && controller == 1 && hacohost == 1 && vscode == 1 && agenthost == 1 && notify == 1 && storagehelper == 1) }
  '; then
    die "release archive must contain exactly haco, haco-controller, haco-host, haco-vscode, haco-agent-host, haco-notify, and haco-storage-helper"
  fi

  archive_verbose="$(LC_ALL=C tar -tvzf "$archive_path")" || die "release archive entry types cannot be inspected"
  if ! printf '%s\n' "$archive_verbose" | awk '
    NF {
      count++
      if (substr($1, 1, 1) != "-") bad = 1
    }
    END { exit !(count == 7 && bad != 1) }
  '; then
    die "release archive contains a non-regular entry"
  fi
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
# checks below independently bind the artifact to the trusted main workflow and
# to the explicit release tag authorized by that workflow. Public installs fail
# closed by default; HACO_REQUIRE_PROVENANCE=0 is an explicit private/dev escape
# hatch and must never be used as the documented public installation path.
verify_provenance

# Treat the archive itself as untrusted input even after integrity/provenance
# checks. A release compromise must not turn extraction into a path/link/device
# write primitive on the installer's host.
validate_release_archive "$tmpdir/$archive"
staging="$tmpdir/staging"
mkdir -m 0700 "$staging"
tar -xzf "$tmpdir/$archive" -C "$staging"
for binary in haco haco-controller haco-host haco-vscode haco-agent-host haco-notify haco-storage-helper; do
  [ -f "$staging/$binary" ] || die "release archive does not contain regular file $binary"
  [ ! -L "$staging/$binary" ] || die "release archive extracted symbolic link for $binary"
  chmod 0755 "$staging/$binary"
done

install_binary() {
  binary="$1"
  install_target="$INSTALL_DIR/$binary"
  if [ -d "$INSTALL_DIR" ] && [ -w "$INSTALL_DIR" ]; then
    cp "$staging/$binary" "$install_target"
    chmod 0755 "$install_target"
  elif command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "$INSTALL_DIR"
    sudo cp "$staging/$binary" "$install_target"
    sudo chmod 0755 "$install_target"
  else
    die "cannot write to $INSTALL_DIR; set HACO_INSTALL_DIR to a writable directory or install sudo"
  fi
  printf 'Installed %s to %s\n' "$binary" "$install_target"
}

install_storage_helper() {
  if [ "$(id -u)" -eq 0 ]; then
    mkdir -p "$STORAGE_HELPER_DIR"
    cp "$staging/haco-storage-helper" "$STORAGE_HELPER_PATH"
    chown root:root "$STORAGE_HELPER_PATH"
    chmod 0755 "$STORAGE_HELPER_PATH"
  elif command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "$STORAGE_HELPER_DIR"
    sudo cp "$staging/haco-storage-helper" "$STORAGE_HELPER_PATH"
    sudo chown root:root "$STORAGE_HELPER_PATH"
    sudo chmod 0755 "$STORAGE_HELPER_PATH"
  else
    die "sudo is required to install the root-owned storage helper"
  fi
  printf 'Installed haco-storage-helper to %s (root-owned, no passwordless sudo rule added)\n' "$STORAGE_HELPER_PATH"
}

prepare_default_haco_root() {
  # A custom HACO_ROOT remains operator-owned configuration. For the default
  # location, create the directory for the invoking user on a fresh install so
  # state and sparse backing images can remain ordinary-user-owned.
  if [ -n "${HACO_ROOT:-}" ] || [ -e "$DEFAULT_HACO_ROOT" ]; then
    return 0
  fi
  uid="$(id -u)"
  gid="$(id -g)"
  if [ "$uid" -eq 0 ]; then
    mkdir -p "$DEFAULT_HACO_ROOT"
    chmod 0700 "$DEFAULT_HACO_ROOT"
  elif command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "$DEFAULT_HACO_ROOT"
    sudo chown "$uid:$gid" "$DEFAULT_HACO_ROOT"
    sudo chmod 0700 "$DEFAULT_HACO_ROOT"
  else
    die "sudo is required to prepare $DEFAULT_HACO_ROOT for the ordinary-user CLI"
  fi
  printf 'Prepared %s for uid %s\n' "$DEFAULT_HACO_ROOT" "$uid"
}

install_binary haco
install_binary haco-controller
install_binary haco-host
install_binary haco-vscode
install_binary haco-agent-host
install_binary haco-notify
install_storage_helper
prepare_default_haco_root

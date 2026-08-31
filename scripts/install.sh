#!/bin/sh
set -eu

REPOSITORY="SLktEx/Hacocoon"
SIGNER_WORKFLOW="$REPOSITORY/.github/workflows/release.yml"
SIGNER_SOURCE_REF="refs/heads/main"
RELEASE_PREDICATE_TYPE="https://hacocoon.dev/attestations/release/v1"
GITHUB_API_VERSION="2026-03-10"
INSTALL_DIR="${HACO_INSTALL_DIR:-/usr/local/bin}"
STORAGE_HELPER_DIR="${HACO_STORAGE_HELPER_INSTALL_DIR:-/usr/local/libexec/hacocoon}"
STORAGE_HELPER_PATH="$STORAGE_HELPER_DIR/haco-storage-helper"
DEFAULT_HACO_ROOT="/var/lib/hacocoon"
VERSION="${1:-${HACO_VERSION:-latest}}"
REQUIRE_PROVENANCE="${HACO_REQUIRE_PROVENANCE:-1}"
BINARIES_ONLY="${HACO_INSTALL_BINARIES_ONLY:-0}"
SKIP_INCUS="${HACO_BOOTSTRAP_SKIP_INCUS:-0}"
GRANT_INCUS_ADMIN="${HACO_BOOTSTRAP_GRANT_INCUS_ADMIN:-0}"
HACOCOON_CONTROLLER_SERVICE="haco-controller.service"
HACOCOON_CONTROLLER_SOCKET="/run/hacocoon/control.sock"
GITHUB_CLI_KEYRING_URL="https://cli.github.com/packages/githubcli-archive-keyring.gpg"
GITHUB_CLI_OLD_KEY_FINGERPRINT_TEXT="2C61 0620 1985 B60E 6C7A C873 23F3 D4EA 7571 6059"
GITHUB_CLI_CURRENT_KEY_FINGERPRINT_TEXT="7F38 BBB5 9D06 4DBC B3D8 4D72 5612 B364 6231 3325"
GITHUB_CLI_KEYRING_PATH="/etc/apt/keyrings/githubcli-archive-keyring.gpg"
GITHUB_CLI_SOURCE_PATH="/etc/apt/sources.list.d/github-cli.list"

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

for bool_name in REQUIRE_PROVENANCE BINARIES_ONLY SKIP_INCUS GRANT_INCUS_ADMIN; do
  eval "bool_value=\${$bool_name}"
  case "$bool_value" in
    0|1) ;;
    *) die "$bool_name must be 0 or 1" ;;
  esac
done

case "$STORAGE_HELPER_DIR" in
  /*) ;;
  *) die "HACO_STORAGE_HELPER_INSTALL_DIR must be an absolute path" ;;
esac

need uname
need id
case "$(uname -s)" in
  Linux) os="linux" ;;
  *) die "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac

SUDO=""
prepare_privilege() {
  if [ "$(id -u)" -eq 0 ]; then
    SUDO=""
    return 0
  fi
  command -v sudo >/dev/null 2>&1 || die "sudo is required for Ubuntu host installation"
  SUDO="sudo"
}

assert_supported_ubuntu() {
  [ -r /etc/os-release ] || die "/etc/os-release is required to verify the supported Ubuntu host"
  . /etc/os-release
  [ "${ID:-}" = "ubuntu" ] || die "install.sh supports Ubuntu only (found ${ID:-unknown})"
  need dpkg
  dpkg --compare-versions "${VERSION_ID:-0}" ge 26.04 ||
    die "Ubuntu 26.04 or newer is required (found ${VERSION_ID:-unknown})"
}

assert_systemd_active() {
  pid1="$(ps -p 1 -o comm= 2>/dev/null | tr -d '[:space:]' || true)"
  [ "$pid1" = "systemd" ] ||
    die "systemd must already be active as PID 1 before install.sh; run the environment-specific pre installer first"
  printf '==> systemd is active as PID 1\n'
}

wait_for_systemd_operational() {
  attempts=0
  state=""
  while [ "$attempts" -lt 80 ]; do
    state="$(systemctl is-system-running 2>/dev/null || true)"
    case "$state" in
      running|degraded)
        return 0
        ;;
      starting|initializing)
        attempts=$((attempts + 1))
        sleep 0.25
        ;;
      *)
        die "systemd entered a non-operational state after package installation (state: ${state:-unknown})"
        ;;
    esac
  done
  die "systemd did not finish starting after package installation (state: ${state:-unknown})"
}

has_gh_attestation_verify() {
  command -v gh >/dev/null 2>&1 && gh attestation verify --help >/dev/null 2>&1
}

validate_github_cli_keyring() {
  keyring="$1"
  old_fingerprint="$(printf '%s' "$GITHUB_CLI_OLD_KEY_FINGERPRINT_TEXT" | tr -d ' ')"
  current_fingerprint="$(printf '%s' "$GITHUB_CLI_CURRENT_KEY_FINGERPRINT_TEXT" | tr -d ' ')"
  primary_fingerprints="$(
    gpg --batch --show-keys --with-colons --fingerprint "$keyring" 2>/dev/null | awk -F: '
      $1 == "pub" { want_primary = 1; next }
      want_primary && $1 == "fpr" { print $10; want_primary = 0 }
    '
  )"

  [ -n "$primary_fingerprints" ] || {
    printf 'haco installer: downloaded GitHub CLI package keyring contains no primary signing keys\n' >&2
    return 1
  }

  current_seen=0
  for fingerprint in $primary_fingerprints; do
    case "$fingerprint" in
      "$old_fingerprint") ;;
      "$current_fingerprint") current_seen=1 ;;
      *)
        printf 'haco installer: GitHub CLI package keyring contains an untrusted primary key: %s\n' "$fingerprint" >&2
        return 1
        ;;
    esac
  done

  [ "$current_seen" = "1" ] || {
    printf 'haco installer: GitHub CLI package keyring does not contain the pinned current signing key\n' >&2
    return 1
  }
}

ensure_gh_attestation_verify() {
  if has_gh_attestation_verify; then
    return 0
  fi

  printf '==> Installing attestation-capable GitHub CLI from the official signed repository\n'
  need curl
  need gpg
  need dpkg

  keyring_tmp="$(mktemp)"
  if ! curl -fsSL --proto '=https' --tlsv1.2 -o "$keyring_tmp" "$GITHUB_CLI_KEYRING_URL"; then
    rm -f "$keyring_tmp"
    die "failed to download the official GitHub CLI package keyring"
  fi

  if ! validate_github_cli_keyring "$keyring_tmp"; then
    rm -f "$keyring_tmp"
    die "refusing to trust changed GitHub CLI signing material"
  fi

  $SUDO mkdir -p -m 0755 /etc/apt/keyrings /etc/apt/sources.list.d
  $SUDO install -o root -g root -m 0644 "$keyring_tmp" "$GITHUB_CLI_KEYRING_PATH"
  rm -f "$keyring_tmp"

  architecture="$(dpkg --print-architecture)"
  printf 'deb [arch=%s signed-by=%s] https://cli.github.com/packages stable main\n' \
    "$architecture" "$GITHUB_CLI_KEYRING_PATH" | $SUDO tee "$GITHUB_CLI_SOURCE_PATH" >/dev/null
  $SUDO chmod 0644 "$GITHUB_CLI_SOURCE_PATH"
  $SUDO apt-get update
  $SUDO apt-get install -y gh

  has_gh_attestation_verify ||
    die "installed GitHub CLI still lacks gh attestation verify; refusing to disable provenance verification"
  gh --version | head -n 1
}

prepare_ubuntu_main() {
  assert_supported_ubuntu
  prepare_privilege
  command -v apt-get >/dev/null 2>&1 || die "apt-get is required on the supported Ubuntu host"

  printf '==> Installing shared Ubuntu host dependencies\n'
  $SUDO apt-get update
  $SUDO apt-get install -y ca-certificates curl tar git sudo systemd systemd-sysv btrfs-progs util-linux gnupg coreutils findutils grep sed

  ensure_gh_attestation_verify
  assert_systemd_active

  if [ "$SKIP_INCUS" = "1" ]; then
    return 0
  fi

  printf '==> Installing Incus\n'
  $SUDO apt-get install -y incus
  wait_for_systemd_operational

  $SUDO systemctl enable --now incus.service 2>/dev/null ||
    $SUDO systemctl enable --now incus 2>/dev/null ||
    die "failed to enable/start Incus with systemd"

  if [ "$GRANT_INCUS_ADMIN" = "1" ] && [ "$(id -u)" -ne 0 ]; then
    if getent group incus-admin >/dev/null 2>&1; then
      printf '==> Granting current Ubuntu user incus-admin access\n'
      warn "incus-admin is root-equivalent local authority"
      $SUDO usermod -aG incus-admin "$(id -un)"
    else
      warn "incus-admin group does not exist after package installation"
    fi
  fi

  if command -v incus >/dev/null 2>&1 && $SUDO incus info >/dev/null 2>&1; then
    if ! $SUDO incus storage list --format csv -c n 2>/dev/null | grep -q .; then
      printf '==> Initializing Incus with a minimal configuration\n'
      $SUDO incus admin init --minimal
    fi
  else
    die "Incus daemon is not ready after systemd startup"
  fi
}

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
    curl -fL --proto '=https' --tlsv1.2 -H "Authorization: Bearer $token" -o "$tmpdir/$archive" "$base/$archive"
    curl -fL --proto '=https' --tlsv1.2 -H "Authorization: Bearer $token" -o "$tmpdir/checksums.txt" "$base/checksums.txt"
  else
    curl -fL --proto '=https' --tlsv1.2 -o "$tmpdir/$archive" "$base/$archive"
    curl -fL --proto '=https' --tlsv1.2 -o "$tmpdir/checksums.txt" "$base/checksums.txt"
  fi
}

download_public_attestation_bundles() {
  digest="$1"
  metadata="$tmpdir/attestations.json"
  bundle_path="$tmpdir/attestations.jsonl"
  bundle_tmp="$tmpdir/attestation-bundle.json"
  api_url="https://api.github.com/repos/$REPOSITORY/attestations/sha256:$digest?per_page=100"

  need curl
  need grep
  need sed
  need cat

  if ! curl -fsSL --proto '=https' --tlsv1.2 \
    -H 'Accept: application/vnd.github+json' \
    -H "X-GitHub-Api-Version: $GITHUB_API_VERSION" \
    -o "$metadata" "$api_url"; then
    printf 'haco installer: failed to fetch public attestation metadata from GitHub\n' >&2
    return 1
  fi

  bundle_urls="$(
    grep -oE '"bundle_url"[[:space:]]*:[[:space:]]*"[^"]+"' "$metadata" 2>/dev/null |
      sed -E 's/^"bundle_url"[[:space:]]*:[[:space:]]*"//; s/"$//' || true
  )"
  if [ -z "$bundle_urls" ]; then
    printf 'haco installer: GitHub returned no public attestation bundles for sha256:%s\n' "$digest" >&2
    return 1
  fi

  : > "$bundle_path"
  if ! printf '%s\n' "$bundle_urls" | while IFS= read -r bundle_url; do
    [ -n "$bundle_url" ] || continue
    case "$bundle_url" in
      https://*) ;;
      *) printf 'haco installer: refusing non-HTTPS public attestation bundle URL\n' >&2; exit 1 ;;
    esac
    case "$bundle_url" in
      *\\*) printf 'haco installer: refusing unexpectedly escaped attestation bundle URL\n' >&2; exit 1 ;;
    esac

    if ! curl -fsSL --proto '=https' --tlsv1.2 -o "$bundle_tmp" "$bundle_url"; then
      printf 'haco installer: failed to download a public attestation bundle\n' >&2
      exit 1
    fi
    [ -s "$bundle_tmp" ] || { printf 'haco installer: downloaded an empty public attestation bundle\n' >&2; exit 1; }
    cat "$bundle_tmp" >> "$bundle_path"
    printf '\n' >> "$bundle_path"
  done; then
    return 1
  fi

  [ -s "$bundle_path" ] || {
    printf 'haco installer: no usable public attestation bundles were downloaded\n' >&2
    return 1
  }
  printf '%s\n' "$bundle_path"
}

verify_provenance() {
  if ! command -v gh >/dev/null 2>&1 || ! gh attestation verify --help >/dev/null 2>&1; then
    if [ "$REQUIRE_PROVENANCE" = "1" ]; then
      die "trusted provenance verification requires a GitHub CLI version with 'gh attestation verify' support"
    fi
    warn "provenance verification was explicitly disabled with HACO_REQUIRE_PROVENANCE=0"
    return 0
  fi

  bundle_path=""
  if ! has_authenticated_gh; then
    bundle_path="$(download_public_attestation_bundles "$actual")" || {
      if [ "$REQUIRE_PROVENANCE" = "1" ]; then
        die "trusted provenance verification could not obtain public attestation bundles"
      fi
      warn "public attestation bundle retrieval failed, but HACO_REQUIRE_PROVENANCE=0 explicitly allows continuing"
      return 0
    }
    printf 'Downloaded public GitHub attestation bundles without requiring a GitHub login.\n'
  fi

  if [ -n "$bundle_path" ]; then
    if ! gh attestation verify "$tmpdir/$archive" \
      --repo "$REPOSITORY" \
      --bundle "$bundle_path" \
      --signer-workflow "$SIGNER_WORKFLOW" \
      --source-ref "$SIGNER_SOURCE_REF" \
      --deny-self-hosted-runners >/dev/null; then
      [ "$REQUIRE_PROVENANCE" = "0" ] || die "trusted build provenance verification failed for $archive"
      warn "trusted build provenance verification failed, but HACO_REQUIRE_PROVENANCE=0 explicitly allows continuing"
      return 0
    fi
  else
    if ! gh attestation verify "$tmpdir/$archive" \
      --repo "$REPOSITORY" \
      --signer-workflow "$SIGNER_WORKFLOW" \
      --source-ref "$SIGNER_SOURCE_REF" \
      --deny-self-hosted-runners >/dev/null; then
      [ "$REQUIRE_PROVENANCE" = "0" ] || die "trusted build provenance verification failed for $archive"
      warn "trusted build provenance verification failed, but HACO_REQUIRE_PROVENANCE=0 explicitly allows continuing"
      return 0
    fi
  fi
  printf 'Verified GitHub/Sigstore provenance for %s from trusted main release workflow.\n' "$archive"

  if [ -n "$bundle_path" ]; then
    binding_tags="$(
      gh attestation verify "$tmpdir/$archive" \
        --repo "$REPOSITORY" \
        --bundle "$bundle_path" \
        --signer-workflow "$SIGNER_WORKFLOW" \
        --source-ref "$SIGNER_SOURCE_REF" \
        --predicate-type "$RELEASE_PREDICATE_TYPE" \
        --deny-self-hosted-runners \
        --format json \
        --jq '.[].verificationResult.statement.predicate.tag' 2>/dev/null || true
    )"
  else
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
  fi

  if printf '%s\n' "$binding_tags" | grep -Fx "$VERSION" >/dev/null 2>&1; then
    printf 'Verified signed release binding for %s.\n' "$VERSION"
    return 0
  fi
  [ "$REQUIRE_PROVENANCE" = "0" ] || die "signed release binding verification failed for $VERSION"
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
    NF { count++; if (substr($1, 1, 1) != "-") bad = 1 }
    END { exit !(count == 7 && bad != 1) }
  '; then
    die "release archive contains a non-regular entry"
  fi
}

install_binary() {
  binary="$1"
  install_target="$INSTALL_DIR/$binary"
  if [ -d "$INSTALL_DIR" ] && [ -w "$INSTALL_DIR" ]; then
    cp "$staging/$binary" "$install_target"
    chmod 0755 "$install_target"
    if [ "$(id -u)" -eq 0 ]; then
      chown root:root "$install_target"
    fi
  elif command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "$INSTALL_DIR"
    sudo cp "$staging/$binary" "$install_target"
    sudo chown root:root "$install_target"
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

install_release_binaries() {
  need tar
  need sha256sum
  need awk
  need mktemp
  need mkdir
  need cp
  need chmod
  need chown

  if [ "$VERSION" = "latest" ]; then
    VERSION="$(resolve_latest_version)"
    printf 'Resolved latest Hacocoon release to %s.\n' "$VERSION"
  else
    validate_version "$VERSION"
  fi

  archive="haco_${os}_${arch}.tar.gz"
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

  if has_authenticated_gh; then
    download_with_gh "$VERSION" || die "failed to download release assets with gh"
  else
    need curl
    download_with_curl "$VERSION" || die "failed to download release assets; private repositories require authenticated GitHub access"
  fi

  [ -s "$tmpdir/$archive" ] || die "downloaded archive is empty"
  [ -s "$tmpdir/checksums.txt" ] || die "downloaded checksums file is empty"
  expected="$(awk -v name="$archive" '$2 == name || $2 == "*" name { print $1; exit }' "$tmpdir/checksums.txt")"
  [ -n "$expected" ] || die "checksum for $archive not found"
  actual="$(sha256sum "$tmpdir/$archive" | awk '{print $1}')"
  [ "$actual" = "$expected" ] || die "checksum verification failed for $archive"
  printf 'Verified SHA-256 integrity for %s against checksums.txt.\n' "$archive"

  verify_provenance
  validate_release_archive "$tmpdir/$archive"

  staging="$tmpdir/staging"
  mkdir -m 0700 "$staging"
  tar -xzf "$tmpdir/$archive" -C "$staging"
  for binary in haco haco-controller haco-host haco-vscode haco-agent-host haco-notify haco-storage-helper; do
    [ -f "$staging/$binary" ] || die "release archive does not contain regular file $binary"
    [ ! -L "$staging/$binary" ] || die "release archive extracted symbolic link for $binary"
    chmod 0755 "$staging/$binary"
  done

  install_binary haco
  install_binary haco-controller
  install_binary haco-host
  install_binary haco-vscode
  install_binary haco-agent-host
  install_binary haco-notify
  install_storage_helper
  prepare_default_haco_root
}

configure_hacocoon_controller() {
  controller_bin="$1"
  case "$controller_bin" in
    /usr/local/bin/haco-controller|/usr/bin/haco-controller) ;;
    *) die "controller service requires a system-owned haco-controller at /usr/local/bin or /usr/bin (got $controller_bin)" ;;
  esac

  owner="$($SUDO stat -Lc '%u' "$controller_bin")"
  [ "$owner" = "0" ] || die "refusing controller service through non-root-owned binary: $controller_bin"
  if $SUDO find "$controller_bin" -perm /022 -print -quit | grep -q .; then
    die "refusing controller service through group/world-writable binary: $controller_bin"
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
      die "Physical Host controller service exited before creating its socket"
    fi
    attempts=$((attempts + 1))
    sleep 0.05
  done

  if ! $SUDO test -S "$HACOCOON_CONTROLLER_SOCKET"; then
    $SUDO systemctl status "$HACOCOON_CONTROLLER_SERVICE" --no-pager >&2 || true
    die "controller did not create $HACOCOON_CONTROLLER_SOCKET"
  fi

  socket_state="$($SUDO stat -Lc '%u:%g:%a' "$HACOCOON_CONTROLLER_SOCKET")"
  [ "$socket_state" = "0:0:600" ] ||
    die "unsafe controller socket ownership/mode: $socket_state (want 0:0:600)"
}

if [ "$BINARIES_ONLY" != "1" ]; then
  prepare_ubuntu_main
fi

printf '==> Installing Hacocoon release\n'
install_release_binaries

if [ "$BINARIES_ONLY" = "1" ]; then
  printf '%s\n' 'Hacocoon release binaries installed; shared Ubuntu host setup was explicitly skipped.'
  exit 0
fi

if [ "$SKIP_INCUS" = "1" ]; then
  printf '%s\n' 'haco installer: Incus and trusted haco-host reconciliation were explicitly skipped.'
  exit 0
fi

haco_bin="$(command -v haco || true)"
controller_bin="$(command -v haco-controller || true)"
[ -n "$haco_bin" ] && [ -n "$controller_bin" ] ||
  die "haco or haco-controller binary is unavailable after installation"
haco_bin="$(readlink -f "$haco_bin")"
controller_bin="$(readlink -f "$controller_bin")"

configure_hacocoon_controller "$controller_bin"
printf '==> Reconciling trusted haco-host and controller endpoint\n'
$SUDO "$haco_bin" host ensure || die "failed to prepare haco-host"
printf '==> Verifying trusted haco-host controller round trip\n'
$SUDO incus exec haco-host --project hacocoon -- /usr/local/bin/haco-host doctor >/dev/null ||
  die "haco-host cannot reach the Physical Host controller"

printf '%s\n' 'Shared Ubuntu Hacocoon installation completed successfully.'
if [ "$GRANT_INCUS_ADMIN" = "1" ] && [ "$(id -u)" -ne 0 ]; then
  printf '%s\n' 'Start a new login session (or use newgrp incus-admin) before relying on the new group membership.'
fi

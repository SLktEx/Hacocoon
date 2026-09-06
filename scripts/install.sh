#!/bin/sh
set -eu

REPOSITORY="SLktEx/Hacocoon"
SIGNER_WORKFLOW="$REPOSITORY/.github/workflows/release.yml"
SIGNER_SOURCE_REF="refs/heads/main"
RELEASE_PREDICATE_TYPE="https://hacocoon.dev/attestations/release/v1"
GITHUB_API_VERSION="2026-03-10"
INSTALL_DIR="${HACO_INSTALL_DIR:-/usr/local/bin}"
DEFAULT_HACO_ROOT="/var/lib/hacocoon"
VERSION="${1:-${HACO_VERSION:-latest}}"
REQUIRE_PROVENANCE="${HACO_REQUIRE_PROVENANCE:-1}"
BINARIES_ONLY="${HACO_INSTALL_BINARIES_ONLY:-0}"
SKIP_INCUS="${HACO_BOOTSTRAP_SKIP_INCUS:-0}"
GRANT_INCUS_ADMIN="${HACO_BOOTSTRAP_GRANT_INCUS_ADMIN:-0}"
HACOCOON_CONTROLLER_SERVICE="haco-controller.service"
HACOCOON_CONTROLLER_SOCKET="/run/hacocoon/control.sock"
HACOCOON_ACCESS_GROUP="hacocoon"
GITHUB_CLI_KEYRING_URL="https://cli.github.com/packages/githubcli-archive-keyring.gpg"
GITHUB_CLI_OLD_KEY_FINGERPRINT_TEXT="2C61 0620 1985 B60E 6C7A C873 23F3 D4EA 7571 6059"
GITHUB_CLI_CURRENT_KEY_FINGERPRINT_TEXT="7F38 BBB5 9D06 4DBC B3D8 4D72 5612 B364 6231 3325"
GITHUB_CLI_KEYRING_PATH="/etc/apt/keyrings/githubcli-archive-keyring.gpg"
GITHUB_CLI_SOURCE_PATH="/etc/apt/sources.list.d/github-cli.list"
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BUNDLE_ROOT="${HACO_BUNDLE_ROOT:-$SCRIPT_DIR}"

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
RELEASE_ARCHIVE="haco_${os}_${arch}.tar.gz"

has_bundled_release() {
  [ -s "$BUNDLE_ROOT/$RELEASE_ARCHIVE" ] &&
    [ -s "$BUNDLE_ROOT/checksums.txt" ] &&
    [ -s "$BUNDLE_ROOT/VERSION" ]
}

assert_ubuntu() (
  [ -r /etc/os-release ] || die "/etc/os-release is unavailable"
  . /etc/os-release
  [ "${ID:-}" = "ubuntu" ] || die "Hacocoon host installation currently supports Ubuntu only (got ${ID:-unknown})"
  need dpkg
  dpkg --compare-versions "${VERSION_ID:-0}" ge 26.04 ||
    die "Hacocoon host installation requires Ubuntu 26.04 or newer (got ${VERSION_ID:-unknown})"
)

prepare_privilege() {
  if [ "$(id -u)" -eq 0 ]; then
    SUDO=""
  else
    command -v sudo >/dev/null 2>&1 || die "sudo is required for Ubuntu host setup"
    SUDO="sudo"
  fi
}

resolve_install_identity() {
  # Privileged execution and the ordinary workspace owner are separate. The
  # WSL pre phase supplies a name, never caller-selected numeric IDs.
  install_caller_uid="$(id -u)"
  INSTALL_USER="${HACO_INSTALL_USER:-}"
  if [ -z "$INSTALL_USER" ]; then
    if [ "$install_caller_uid" != "0" ]; then
      INSTALL_USER="$(id -un)"
    else
      INSTALL_USER="${SUDO_USER:-root}"
    fi
  fi
  case "$INSTALL_USER" in
    ""|-*|*[!a-zA-Z0-9_.-]*) die "invalid installer user name" ;;
  esac
  INSTALL_UID="$(id -u -- "$INSTALL_USER")" || die "installer user does not exist"
  INSTALL_GID="$(id -g -- "$INSTALL_USER")" || die "installer group does not exist"
  case "$INSTALL_UID:$INSTALL_GID" in
    :*|*:|*[!0-9:]*) die "installer user identity is not numeric" ;;
  esac
  if [ -n "${HACO_INSTALL_USER:-}" ]; then
    [ "$INSTALL_UID" != "0" ] && [ "$INSTALL_GID" != "0" ] ||
      die "explicit installer user must have non-root UID and GID"
    [ "$install_caller_uid" = "0" ] || [ "$install_caller_uid" = "$INSTALL_UID" ] ||
      die "only root may select a different installer user"
  fi
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
  [ -n "$primary_fingerprints" ] || die "downloaded GitHub CLI package keyring contains no primary signing keys"

  current_seen=0
  for fingerprint in $primary_fingerprints; do
    case "$fingerprint" in
      "$old_fingerprint") ;;
      "$current_fingerprint") current_seen=1 ;;
      *) die "downloaded GitHub CLI package keyring contains an untrusted primary key: $fingerprint" ;;
    esac
  done
  [ "$current_seen" = "1" ] || die "downloaded GitHub CLI package keyring does not contain the pinned current signing key"
}

ensure_gh_attestation_verify() {
  if has_bundled_release; then
    return 0
  fi
  if [ "$REQUIRE_PROVENANCE" = "0" ]; then
    return 0
  fi
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
  validate_github_cli_keyring "$keyring_tmp"

  $SUDO mkdir -p -m 0755 /etc/apt/keyrings /etc/apt/sources.list.d
  $SUDO install -o root -g root -m 0644 "$keyring_tmp" "$GITHUB_CLI_KEYRING_PATH"
  rm -f "$keyring_tmp"
  architecture="$(dpkg --print-architecture)"
  printf 'deb [arch=%s signed-by=%s] https://cli.github.com/packages stable main\n' \
    "$architecture" "$GITHUB_CLI_KEYRING_PATH" | $SUDO tee "$GITHUB_CLI_SOURCE_PATH" >/dev/null
  $SUDO chmod 0644 "$GITHUB_CLI_SOURCE_PATH"
  $SUDO apt-get update
  $SUDO apt-get install -y gh
  has_gh_attestation_verify || die "installed GitHub CLI still lacks gh attestation verify"
}

root_subid_contains() {
  file="$1"
  id_value="$2"
  $SUDO test -r "$file" || return 1
  $SUDO awk -F: -v id="$id_value" '
    $1 == "root" && id >= $2 && id - $2 < $3 { found = 1 }
    END { exit found ? 0 : 1 }
  ' "$file"
}

allow_root_subid() {
  file="$1"
  id_value="$2"
  [ "$id_value" != "0" ] || return 0
  root_subid_contains "$file" "$id_value" && return 0
  $SUDO test -f "$file" || die "$file is unavailable for Incus subordinate-ID configuration"
  printf 'root:%s:1\n' "$id_value" | $SUDO tee -a "$file"
  root_subid_contains "$file" "$id_value" || die "failed to authorize subordinate ID $id_value in $file"
}

configure_workspace_owner_idmap() {
  workspace_uid="$INSTALL_UID"
  workspace_gid="$INSTALL_GID"
  case "$workspace_uid:$workspace_gid" in
    *[!0-9:]*) die "installer user identity is not numeric: $workspace_uid:$workspace_gid" ;;
  esac

  # Hacocoon keeps Environments unprivileged. Incus must nevertheless be able
  # to map the one host UID/GID that owns the ordinary user's leased workspace
  # to container root. Delegate only those exact IDs; do not grant a broad host
  # range.
  allow_root_subid /etc/subuid "$workspace_uid"
  allow_root_subid /etc/subgid "$workspace_gid"
}

bridge_netfilter_ready() {
  [ -e /proc/sys/net/bridge/bridge-nf-call-iptables ] &&
    [ -e /proc/sys/net/bridge/bridge-nf-call-ip6tables ]
}

ensure_bridge_netfilter() {
  need modprobe
  if ! bridge_netfilter_ready; then
    $SUDO modprobe br_netfilter ||
      die "br_netfilter is required for Hacocoon sandbox IP/MAC filtering but this kernel cannot load it"
  fi
  bridge_netfilter_ready ||
    die "br_netfilter loaded without exposing the required bridge netfilter hooks"

  # Persist only when br_netfilter is a loadable module. If it is built into
  # the kernel, the hooks above are already permanent and modules-load would be
  # unnecessary noise on boot.
  if command -v modinfo >/dev/null 2>&1 && modinfo br_netfilter >/dev/null 2>&1; then
    printf 'br_netfilter\n' | $SUDO tee /etc/modules-load.d/hacocoon.conf >/dev/null
    $SUDO chmod 0644 /etc/modules-load.d/hacocoon.conf
  fi
}

ensure_incus_userns_compatibility() {
  apparmor_userns_path="/proc/sys/kernel/apparmor_restrict_unprivileged_unconfined"
  apparmor_userns_conf="/etc/sysctl.d/90-hacocoon-incus-userns.conf"

  # Ubuntu adds an AppArmor restriction for unprivileged user namespaces that
  # is stricter than upstream AppArmor. systemd 259 uses a user namespace for
  # service sandboxing inside Ubuntu 26.04 Incus containers; with the Ubuntu
  # restriction enabled those services can remain stuck before networkd starts,
  # leaving otherwise healthy Incus bridges without a guest DHCPv4 client.
  # Incus upstream recommends restoring normal AppArmor behavior on Ubuntu
  # container hosts. This does not disable Hacocoon's AppArmor or nftables
  # isolation policy; it only removes Ubuntu's host-global extra userns gate.
  if [ ! -e "$apparmor_userns_path" ]; then
    return 0
  fi

  need sysctl
  printf '%s\n' \
    '# Required for systemd user-namespace sandboxing inside Incus containers.' \
    'kernel.apparmor_restrict_unprivileged_unconfined = 0' |
    $SUDO tee "$apparmor_userns_conf" >/dev/null
  $SUDO chmod 0644 "$apparmor_userns_conf"
  $SUDO sysctl -q -w kernel.apparmor_restrict_unprivileged_unconfined=0 ||
    die "failed to allow systemd user-namespace sandboxing inside Incus containers"
  [ "$($SUDO cat "$apparmor_userns_path")" = "0" ] ||
    die "Ubuntu AppArmor user-namespace restriction remained enabled after configuration"
}

configure_incus_boot_guard() {
  guard_source="$BUNDLE_ROOT/incus-boot-guard.py"
  if [ ! -f "$guard_source" ]; then
    guard_source="$SCRIPT_DIR/../modules/runtime/incus/packaging/incus-boot-guard.py"
  fi
  [ -f "$guard_source" ] || die "the Incus startup guard is missing from the installer"
  $SUDO install -d -o root -g root -m 0755 /usr/local/libexec
  $SUDO install -o root -g root -m 0755 "$guard_source" /usr/local/libexec/hacocoon-incus-boot-guard
  # First adoption is allowed only with the existing trusted daemon present.
  # Never stamp a new namespace over old PID records before retiring them.
  $SUDO /usr/bin/python3 -I /usr/local/libexec/hacocoon-incus-boot-guard --initialize ||
    die "cannot initialize Incus startup guard"
  guard_unit="$(mktemp)"
  cat > "$guard_unit" <<'EOF_GUARD'
[Service]
ExecStartPre=/usr/bin/python3 -I /usr/local/libexec/hacocoon-incus-boot-guard
EOF_GUARD
  $SUDO install -d -o root -g root -m 0755 /etc/systemd/system/incus.service.d
  $SUDO install -o root -g root -m 0644 "$guard_unit" /etc/systemd/system/incus.service.d/90-hacocoon-boot-guard.conf
  rm -f "$guard_unit"
  $SUDO systemctl daemon-reload
}

prepare_ubuntu_host() {
  assert_ubuntu
  prepare_privilege

  printf '==> Installing common Ubuntu host dependencies\n'
  $SUDO apt-get update
  $SUDO apt-get install -y ca-certificates curl tar git sudo systemd systemd-sysv btrfs-progs util-linux kmod procps gnupg coreutils findutils grep sed python3
  ensure_gh_attestation_verify

  pid1="$(ps -p 1 -o comm= 2>/dev/null | tr -d '[:space:]' || true)"
  [ "$pid1" = "systemd" ] || die "systemd must already be active as PID 1 before install.sh runs"

  if [ "$SKIP_INCUS" = "1" ]; then
    return 0
  fi

  printf '==> Installing and starting Incus\n'
  $SUDO apt-get install -y incus iptables nftables
  printf '==> Authorizing the local Hacocoon workspace owner for Incus idmap\n'
  configure_workspace_owner_idmap
  printf '==> Preparing bridge netfilter for Hacocoon sandbox filtering\n'
  ensure_bridge_netfilter
  printf '==> Preparing Ubuntu AppArmor user namespaces for Incus system services\n'
  ensure_incus_userns_compatibility
  $SUDO systemctl enable --now incus.service 2>/dev/null || $SUDO systemctl enable --now incus 2>/dev/null ||
    die "failed to enable/start Incus with systemd"

  if [ "$GRANT_INCUS_ADMIN" = "1" ] && [ "$INSTALL_UID" != "0" ]; then
    if getent group incus-admin >/dev/null 2>&1; then
      warn "granting incus-admin gives the current Ubuntu user root-equivalent local Incus authority"
      $SUDO usermod -aG incus-admin "$INSTALL_USER"
    else
      warn "incus-admin group does not exist after package installation"
    fi
  fi

  # The Incus adapter owns its Btrfs pool and trusted-host bridge explicitly.
  # Minimal initialization would also create an unused directory pool and
  # couple network availability to whether any storage pool already exists.
  if ! command -v incus >/dev/null 2>&1 || ! $SUDO incus info >/dev/null 2>&1; then
    die "Incus daemon is not ready after systemd startup"
  fi
  configure_incus_boot_guard
}

verify_trusted_host_connectivity() {
  network_attempt=0
  while [ "$network_attempt" -lt 10 ]; do
    # Guest networkd/DHCP can lag behind Incus's RUNNING state. Bound each
    # guest probe and wait without changing any network/firewall configuration.
    if $SUDO incus exec haco-host --project hacocoon -- env -i PATH=/usr/sbin:/usr/bin:/sbin:/bin timeout 8 /bin/sh -ec '
      getent ahostsv4 github.com >/dev/null
      ip -4 route show default | grep -q "^default "
      curl -q -4 -f -sS --connect-timeout 3 --max-time 5 -o /dev/null https://github.com
    ' >/dev/null 2>&1; then
      return 0
    fi
    network_attempt=$((network_attempt + 1))
    [ "$network_attempt" -ge 10 ] || sleep 1
  done
  return 1
}

has_authenticated_gh() {
  command -v gh >/dev/null 2>&1 &&
    { [ -n "${GH_TOKEN:-}" ] || [ -n "${GITHUB_TOKEN:-}" ] || gh auth status >/dev/null 2>&1; }
}

resolve_latest_version() {
  need curl
  latest_url="$(curl -fsSL --proto '=https' --tlsv1.2 -o /dev/null -w '%{url_effective}' "https://github.com/$REPOSITORY/releases/latest")" ||
    die "failed to resolve latest release"
  latest_tag="${latest_url##*/}"
  validate_version "$latest_tag"
  printf '%s\n' "$latest_tag"
}

download_public_attestation_bundles() {
  digest="$1"
  metadata="$tmpdir/attestations.json"
  bundle_path="$tmpdir/attestations.jsonl"
  bundle_tmp="$tmpdir/attestation-bundle.json"
  api_url="https://api.github.com/repos/$REPOSITORY/attestations/sha256:$digest?per_page=100"
  need curl

  curl -fsSL --proto '=https' --tlsv1.2 \
    -H 'Accept: application/vnd.github+json' \
    -H "X-GitHub-Api-Version: $GITHUB_API_VERSION" \
    -o "$metadata" "$api_url" || return 1

  bundle_urls="$(
    grep -oE '"bundle_url"[[:space:]]*:[[:space:]]*"[^"]+"' "$metadata" 2>/dev/null |
      sed -E 's/^"bundle_url"[[:space:]]*:[[:space:]]*"//; s/"$//' || true
  )"
  [ -n "$bundle_urls" ] || return 1
  : > "$bundle_path"

  if ! printf '%s\n' "$bundle_urls" | while IFS= read -r bundle_url; do
    [ -n "$bundle_url" ] || continue
    case "$bundle_url" in
      https://*) ;;
      *)
        printf '%s\n' 'haco installer: refusing non-HTTPS public attestation bundle URL' >&2
        exit 1
        ;;
    esac
    case "$bundle_url" in
      *\\*)
        printf '%s\n' 'haco installer: refusing escaped public attestation bundle URL' >&2
        exit 1
        ;;
    esac
    curl -fsSL --proto '=https' --tlsv1.2 -o "$bundle_tmp" "$bundle_url" || exit 1
    [ -s "$bundle_tmp" ] || exit 1
    cat "$bundle_tmp" >> "$bundle_path"
    printf '\n' >> "$bundle_path"
  done; then
    return 1
  fi
  [ -s "$bundle_path" ] || return 1
  printf '%s\n' "$bundle_path"
}

verify_provenance() {
  if [ "$REQUIRE_PROVENANCE" = "0" ]; then
    warn "provenance verification was explicitly disabled with HACO_REQUIRE_PROVENANCE=0"
    return 0
  fi
  has_gh_attestation_verify || die "trusted provenance verification requires a GitHub CLI version with 'gh attestation verify' support"

  bundle_path=""
  if ! has_authenticated_gh; then
    bundle_path="$(download_public_attestation_bundles "$actual")" ||
      die "trusted provenance verification could not obtain public attestation bundles"
    printf 'Downloaded public GitHub attestation bundles without requiring a GitHub login.\n'
  fi

  set -- "$tmpdir/$archive" --repo "$REPOSITORY" --signer-workflow "$SIGNER_WORKFLOW" --source-ref "$SIGNER_SOURCE_REF" --deny-self-hosted-runners
  if [ -n "$bundle_path" ]; then
    set -- "$@" --bundle "$bundle_path"
  fi
  gh attestation verify "$@" >/dev/null || die "trusted build provenance verification failed for $archive"
  printf 'Verified GitHub/Sigstore provenance for %s from trusted main release workflow.\n' "$archive"

  set -- "$tmpdir/$archive" --repo "$REPOSITORY" --signer-workflow "$SIGNER_WORKFLOW" --source-ref "$SIGNER_SOURCE_REF" \
    --predicate-type "$RELEASE_PREDICATE_TYPE" --deny-self-hosted-runners --format json --jq '.[].verificationResult.statement.predicate.tag'
  if [ -n "$bundle_path" ]; then
    set -- "$@" --bundle "$bundle_path"
  fi
  binding_tags="$(gh attestation verify "$@" 2>/dev/null || true)"
  printf '%s\n' "$binding_tags" | grep -Fx "$VERSION" >/dev/null 2>&1 ||
    die "signed release binding verification failed for $VERSION"
  printf 'Verified signed release binding for %s.\n' "$VERSION"
}

validate_release_archive() {
  archive_path="$1"
  archive_names="$(tar -tzf "$archive_path")" || die "release archive cannot be listed safely"
  if ! printf '%s\n' "$archive_names" | awk '
    $0 == "haco" { haco++; next }
    $0 == "hacoq" { hacoq++; next }
    $0 == "haco-controller" { controller++; next }
    $0 == "haco-host" { hacohost++; next }
    $0 == "haco-vscode" { vscode++; next }
    $0 == "haco-agent-host" { agenthost++; next }
    $0 == "haco-notify" { notify++; next }
    $0 ~ /^README[^/]*$/ { next }
    $0 ~ /^LICENSE[^/]*$/ { next }
    { bad=1 }
    END { exit !(bad != 1 && haco == 1 && hacoq == 1 && controller == 1 && hacohost == 1 && vscode == 1 && agenthost == 1 && notify == 1) }
  '; then
    die "release archive must contain each Hacocoon release binary exactly once; only root README/LICENSE files are allowed in addition"
  fi
  archive_verbose="$(LC_ALL=C tar -tvzf "$archive_path")" || die "release archive entry types cannot be inspected"
  printf '%s\n' "$archive_verbose" | awk 'NF && substr($1,1,1) != "-" { bad=1 } END { exit (bad == 1) }' ||
    die "release archive contains a non-regular entry"
}

install_binary() {
  binary="$1"
  target="$INSTALL_DIR/$binary"
  case "$INSTALL_DIR" in
    /usr/local/bin|/usr/bin)
      $SUDO install -o root -g root -m 0755 "$staging/$binary" "$target"
      ;;
    *)
      if [ -d "$INSTALL_DIR" ] && [ -w "$INSTALL_DIR" ]; then
        cp "$staging/$binary" "$target"
        chmod 0755 "$target"
      else
        $SUDO mkdir -p "$INSTALL_DIR"
        $SUDO cp "$staging/$binary" "$target"
        $SUDO chmod 0755 "$target"
      fi
      ;;
  esac
  printf 'Installed %s to %s\n' "$binary" "$target"
}

prepare_default_haco_root() {
  if [ -n "${HACO_ROOT:-}" ]; then
    return 0
  fi

  if [ "$BINARIES_ONLY" = "1" ]; then
    if [ -e "$DEFAULT_HACO_ROOT" ]; then
      return 0
    fi
    uid="$(id -u)"
    gid="$(id -g)"
    $SUDO mkdir -p "$DEFAULT_HACO_ROOT"
    $SUDO chown "$uid:$gid" "$DEFAULT_HACO_ROOT"
    $SUDO chmod 0700 "$DEFAULT_HACO_ROOT"
    return 0
  fi

  # Full Physical Host setup immediately reconciles haco-host through sudo and
  # the system controller also runs as root with this same state root. Keep the
  # default Physical Host state directory under that root authority. An empty
  # directory left by an older installer can be adopted safely; populated
  # non-root state is never silently taken over.
  if [ -e "$DEFAULT_HACO_ROOT" ]; then
    [ -d "$DEFAULT_HACO_ROOT" ] && [ ! -L "$DEFAULT_HACO_ROOT" ] ||
      die "$DEFAULT_HACO_ROOT must be a real directory"
    owner="$($SUDO stat -Lc '%u' "$DEFAULT_HACO_ROOT")"
    if [ "$owner" != "0" ]; then
      if $SUDO find "$DEFAULT_HACO_ROOT" -mindepth 1 -print -quit | grep -q .; then
        die "refusing to take over populated non-root Hacocoon state at $DEFAULT_HACO_ROOT"
      fi
      $SUDO chown root:root "$DEFAULT_HACO_ROOT"
    fi
    $SUDO chmod 0700 "$DEFAULT_HACO_ROOT"
  else
    $SUDO install -d -o root -g root -m 0700 "$DEFAULT_HACO_ROOT"
  fi
}

stage_release_archive() {
  archive="$RELEASE_ARCHIVE"
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM
  bundled=0

  if [ "$VERSION" = "latest" ]; then
    if [ -s "$BUNDLE_ROOT/VERSION" ]; then
      VERSION="$(tr -d '\r\n' < "$BUNDLE_ROOT/VERSION")"
      validate_version "$VERSION"
      printf 'Using bundled Hacocoon release %s.\n' "$VERSION"
    else
      VERSION="$(resolve_latest_version)"
      printf 'Resolved latest Hacocoon release to %s.\n' "$VERSION"
    fi
  else
    validate_version "$VERSION"
  fi

  if has_bundled_release; then
    bundled=1
    printf '==> Using bundled %s\n' "$archive"
    cp "$BUNDLE_ROOT/$archive" "$tmpdir/$archive"
    cp "$BUNDLE_ROOT/checksums.txt" "$tmpdir/checksums.txt"
  else
    printf '==> Bundled archive unavailable; downloading standalone release assets\n'
    base="https://github.com/$REPOSITORY/releases/download/$VERSION"
    need curl
    curl -fL --proto '=https' --tlsv1.2 -o "$tmpdir/$archive" "$base/$archive" || die "failed to download $archive"
    curl -fL --proto '=https' --tlsv1.2 -o "$tmpdir/checksums.txt" "$base/checksums.txt" || die "failed to download checksums.txt"
  fi

  expected="$(awk -v name="$archive" '$2 == name || $2 == "*" name { print $1; exit }' "$tmpdir/checksums.txt")"
  [ -n "$expected" ] || die "checksum for $archive not found"
  actual="$(sha256sum "$tmpdir/$archive" | awk '{print $1}')"
  [ "$actual" = "$expected" ] || die "checksum verification failed for $archive"
  printf 'Verified SHA-256 integrity for %s.\n' "$archive"
  if [ "$bundled" = "1" ]; then
    printf 'Using installer-bundled release payload; outer package provenance is verified by the release distribution pipeline.\n'
  else
    verify_provenance
  fi
  validate_release_archive "$tmpdir/$archive"
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

  stage_release_archive
  staging="$tmpdir/staging"
  mkdir -m 0700 "$staging"
  tar -xzf "$tmpdir/$archive" -C "$staging"
  for binary in haco hacoq haco-controller haco-host haco-vscode haco-agent-host haco-notify; do
    [ -f "$staging/$binary" ] || die "release archive does not contain regular file $binary"
    [ ! -L "$staging/$binary" ] || die "release archive extracted symbolic link for $binary"
    chmod 0755 "$staging/$binary"
  done

  for binary in haco hacoq haco-controller haco-host haco-vscode haco-agent-host haco-notify; do
    install_binary "$binary"
  done
  prepare_default_haco_root
}

resolve_hacocoon_access_user() {
  [ "$INSTALL_UID" != "0" ] || return 1
  printf '%s\n' "$INSTALL_USER"
}

configure_hacocoon_access_group() {
  if ! getent group "$HACOCOON_ACCESS_GROUP" >/dev/null 2>&1; then
    $SUDO /usr/sbin/groupadd --system "$HACOCOON_ACCESS_GROUP"
  fi

  HACOCOON_ACCESS_GID="$(getent group "$HACOCOON_ACCESS_GROUP" | awk -F: '{print $3; exit}')"
  case "$HACOCOON_ACCESS_GID" in
    ""|*[!0-9]*) die "unable to resolve numeric gid for $HACOCOON_ACCESS_GROUP group" ;;
    0) die "$HACOCOON_ACCESS_GROUP group must not use gid 0" ;;
  esac

  HACOCOON_ACCESS_USER="$(resolve_hacocoon_access_user || true)"
  if [ -n "$HACOCOON_ACCESS_USER" ]; then
    getent passwd "$HACOCOON_ACCESS_USER" >/dev/null 2>&1 ||
      die "installer access user does not exist: $HACOCOON_ACCESS_USER"
    $SUDO /usr/sbin/usermod -aG "$HACOCOON_ACCESS_GROUP" "$HACOCOON_ACCESS_USER"
  fi
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

  configure_hacocoon_access_group

  unit_tmp="$(mktemp)"
  cat > "$unit_tmp" <<EOF_UNIT
[Unit]
Description=Hacocoon Physical Host controller
Requires=incus.service
After=incus.service

[Service]
Type=simple
ExecStart=$controller_bin --standard-egress
Restart=on-failure
RestartSec=1s
RuntimeDirectory=hacocoon
RuntimeDirectoryMode=0755
UMask=0077
Environment=HACO_ROOT=/var/lib/hacocoon
Environment=HACO_CONTROL_GROUP_GID=$HACOCOON_ACCESS_GID

[Install]
WantedBy=multi-user.target
EOF_UNIT
  $SUDO install -o root -g root -m 0644 "$unit_tmp" "/etc/systemd/system/$HACOCOON_CONTROLLER_SERVICE"
  rm -f "$unit_tmp"
  $SUDO systemctl daemon-reload
  $SUDO systemctl enable "$HACOCOON_CONTROLLER_SERVICE" >/dev/null
  $SUDO systemctl restart "$HACOCOON_CONTROLLER_SERVICE"

  attempts=0
  while [ "$attempts" -lt 600 ]; do
    if $SUDO test -S "$HACOCOON_CONTROLLER_SOCKET"; then
      break
    fi
    $SUDO systemctl is-active --quiet "$HACOCOON_CONTROLLER_SERVICE" || die "Physical Host controller service exited before creating its socket"
    attempts=$((attempts + 1))
    sleep 0.05
  done
  $SUDO test -S "$HACOCOON_CONTROLLER_SOCKET" || die "controller did not create $HACOCOON_CONTROLLER_SOCKET"
  socket_state="$($SUDO stat -Lc '%u:%g:%a' "$HACOCOON_CONTROLLER_SOCKET")"
  expected_socket_state="0:$HACOCOON_ACCESS_GID:660"
  [ "$socket_state" = "$expected_socket_state" ] ||
    die "unsafe controller socket ownership/mode: $socket_state (want $expected_socket_state)"
}

assert_ubuntu
prepare_privilege
resolve_install_identity
if [ "$BINARIES_ONLY" != "1" ]; then
  prepare_ubuntu_host
fi

printf '==> Installing Hacocoon release\n'
install_release_binaries

if [ "$BINARIES_ONLY" = "1" ]; then
  printf '%s\n' 'Hacocoon release binaries installed; host setup was explicitly skipped.'
  exit 0
fi
if [ "$SKIP_INCUS" = "1" ]; then
  printf '%s\n' 'haco installer: -SkipIncus skips controller and trusted haco-host reconciliation.'
  exit 0
fi

haco_bin="$(command -v haco || true)"
controller_bin="$(command -v haco-controller || true)"
[ -n "$haco_bin" ] && [ -n "$controller_bin" ] || die "haco or haco-controller binary is unavailable after installation"
haco_bin="$(readlink -f "$haco_bin")"
controller_bin="$(readlink -f "$controller_bin")"

printf '==> Configuring Physical Host controller service\n'
configure_hacocoon_controller "$controller_bin"
printf '==> Reconciling trusted haco-host and controller endpoint\n'
$SUDO "$haco_bin" setup || die "controller-backed Host setup failed; run haco doctor, then rerun the installer"
printf '==> Verifying trusted haco-host controller round trip\n'
$SUDO incus exec haco-host --project hacocoon -- /usr/local/bin/haco-host doctor >/dev/null ||
  die "haco-host cannot reach the Physical Host controller"
printf '==> Verifying trusted haco-host DNS, route and HTTPS\n'
verify_trusted_host_connectivity || die "haco-host network is not ready (DNS, default route or HTTPS failed after bounded probes)"
printf '==> Verifying configured and live installation readiness\n'
$SUDO "$haco_bin" doctor || die "installation readiness checks did not pass; follow the reported next actions and rerun the current installer"

if [ -n "${HACOCOON_ACCESS_USER:-}" ]; then
  warn "membership in $HACOCOON_ACCESS_GROUP grants authority to control Hacocoon environments; treat it as a privileged local group"
  printf 'haco installer: added %s to %s; start a new login session (or run newgrp %s) before using haco without sudo.\n' \
    "$HACOCOON_ACCESS_USER" "$HACOCOON_ACCESS_GROUP" "$HACOCOON_ACCESS_GROUP"
fi
if [ "$GRANT_INCUS_ADMIN" = "1" ] && [ "$INSTALL_UID" != "0" ]; then
  printf '%s\n' 'haco installer: start a new login session (or use newgrp incus-admin) before relying on the new group membership.'
fi
printf '%s\n' 'Hacocoon common Ubuntu installation complete.'

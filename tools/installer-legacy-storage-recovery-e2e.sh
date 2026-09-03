#!/usr/bin/env bash
set -euo pipefail

readonly PROJECT="hacocoon"
readonly POOL="haco-local-default"
readonly ROOT="/var/lib/hacocoon"
readonly BACKING="$ROOT/images/local-default.raw"
readonly MOUNTPOINT="$ROOT/mounts/local-default"
readonly PRESERVE_VOLUME="legacy-preserve"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_root() {
  [[ "$(id -u)" == "0" ]] || fail "legacy storage recovery acceptance must run as root"
  [[ "$(ps -p 1 -o comm= | tr -d '[:space:]')" == "systemd" ]] || fail "systemd must be PID 1"
  command -v incus >/dev/null || fail "incus is unavailable"
  command -v btrfs >/dev/null || fail "btrfs is unavailable"
}

prepare_fixture() {
  require_root

  # This runs only in the disposable Windows installer WSL distribution. Make
  # the fresh #420 pool disappear cleanly, then recreate the exact historical
  # external-path shape that an older installation would leave behind.
  while IFS= read -r instance; do
    [[ -n "$instance" ]] || continue
    case "$instance" in
      haco-*) incus delete "$instance" --project "$PROJECT" --force ;;
      *) fail "refusing unexpected instance while preparing legacy fixture: $instance" ;;
    esac
  done < <(incus list --project "$PROJECT" --format csv -c n 2>/dev/null || true)

  while IFS= read -r fingerprint; do
    [[ -n "$fingerprint" ]] || continue
    incus image delete "$fingerprint" --project "$PROJECT"
  done < <(incus image list --project "$PROJECT" --format csv -c f 2>/dev/null | sort -u)

  if incus storage show "$POOL" --project "$PROJECT" >/dev/null 2>&1; then
    incus storage delete "$POOL" --project "$PROJECT"
  fi

  if findmnt -rn --mountpoint "$MOUNTPOINT" >/dev/null 2>&1; then
    umount "$MOUNTPOINT"
  fi
  while IFS= read -r loop; do
    [[ -n "$loop" ]] || continue
    losetup -d "$loop"
  done < <(losetup -j "$BACKING" 2>/dev/null | cut -d: -f1 || true)

  rm -f -- "$BACKING"
  install -d -o root -g root -m 0700 "$ROOT/images" "$ROOT/mounts" "$MOUNTPOINT"
  truncate -s 8G "$BACKING"
  chmod 0600 "$BACKING"

  loop="$(losetup --find --show --nooverlap "$BACKING")"
  [[ "$loop" =~ ^/dev/loop[0-9]+$ ]] || fail "unexpected loop device: $loop"
  mkfs.btrfs -f "$loop" >/dev/null
  mount "$loop" "$MOUNTPOINT" -o compress=zstd:3

  incus storage create "$POOL" btrfs "source=$MOUNTPOINT" --project "$PROJECT"
  incus storage volume create "$POOL" "$PRESERVE_VOLUME" --project "$PROJECT"

  source_path="$(incus storage get "$POOL" source --project "$PROJECT")"
  [[ "$source_path" == "$MOUNTPOINT" ]] || fail "legacy pool source is $source_path"
  incus storage volume show "$POOL" "$PRESERVE_VOLUME" --project "$PROJECT" >/dev/null
  echo "Prepared legacy external-path pool with preserved custom volume."
}

verify_recovery() {
  require_root

  source_path="$(incus storage get "$POOL" source --project "$PROJECT")"
  [[ "$source_path" == "$MOUNTPOINT" ]] || fail "installer changed legacy pool source: $source_path"
  incus storage volume show "$POOL" "$PRESERVE_VOLUME" --project "$PROJECT" >/dev/null ||
    fail "legacy custom volume did not survive recovery"

  fstype="$(findmnt -rn -o FSTYPE --mountpoint "$MOUNTPOINT")"
  [[ "$fstype" == "btrfs" ]] || fail "legacy mount filesystem is $fstype"
  options="$(findmnt -rn -o OPTIONS --mountpoint "$MOUNTPOINT")"
  [[ ",$options," == *,compress=zstd:3,* || ",$options," == *,compress=zstd,* ]] ||
    fail "legacy mount is missing zstd compression: $options"
  [[ ",$options," != *,autodefrag,* ]] || fail "legacy mount unexpectedly enables autodefrag: $options"

  systemctl cat hacocoon-legacy-storage.service >/dev/null || fail "legacy recovery unit is not installed"
  systemctl cat incus.service | grep -Fq 'Requires=hacocoon-legacy-storage.service' ||
    fail "Incus is not ordered after legacy storage recovery"
  systemctl is-active --quiet incus.service || fail "Incus is not active after recovery"
  incus exec haco-host --project "$PROJECT" -- /usr/local/bin/haco-host doctor >/dev/null ||
    fail "trusted haco-host is unusable after legacy pool recovery"

  echo "Legacy external-path pool recovered without replacing preserved storage."
}

case "${1:-}" in
  prepare) prepare_fixture ;;
  verify) verify_recovery ;;
  *) echo "usage: $0 <prepare|verify>" >&2; exit 2 ;;
esac

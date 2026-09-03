#!/usr/bin/env bash
set -euo pipefail

# Opt-in host experiment for issue #421. This is intentionally not CI: it
# creates and mounts disposable Btrfs loop files and therefore requires root.
# It never operates on an existing block device or filesystem.

readonly ENABLE_ENV="HACO_BTRFS_METADATA_RATIO_EXPERIMENT"
readonly IMAGE_SIZE="${HACO_BTRFS_METADATA_RATIO_IMAGE_SIZE:-6G}"
readonly FILE_COUNT="${HACO_BTRFS_METADATA_RATIO_FILES:-2048}"
readonly SNAPSHOT_COUNT="${HACO_BTRFS_METADATA_RATIO_SNAPSHOTS:-128}"
readonly RATIOS="${HACO_BTRFS_METADATA_RATIOS:-default 2 4 8}"
readonly RESULTS_DIR="${HACO_BTRFS_METADATA_RATIO_RESULTS:-$PWD/btrfs-metadata-ratio-results-$(date -u +%Y%m%dT%H%M%SZ)}"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

[[ "${!ENABLE_ENV:-}" == "1" ]] || fail "set $ENABLE_ENV=1 to acknowledge this root-only disposable-loop experiment"
[[ "$(id -u)" == "0" ]] || fail "Btrfs metadata_ratio experiment must run as root on an isolated Linux host"
[[ "$(uname -s)" == "Linux" ]] || fail "Linux is required"

for tool in btrfs mkfs.btrfs losetup mount umount mountpoint findmnt truncate python3 stat sync wc tr; do
  command -v "$tool" >/dev/null 2>&1 || fail "required tool '$tool' is unavailable"
done

[[ "$FILE_COUNT" =~ ^[1-9][0-9]*$ ]] || fail "HACO_BTRFS_METADATA_RATIO_FILES must be a positive integer"
[[ "$SNAPSHOT_COUNT" =~ ^[1-9][0-9]*$ ]] || fail "HACO_BTRFS_METADATA_RATIO_SNAPSHOTS must be a positive integer"

workdir="$(mktemp -d)"
current_mount=""
current_loop=""

cleanup_case() {
  set +e
  if [[ -n "$current_mount" ]] && mountpoint -q "$current_mount"; then
    umount "$current_mount"
  fi
  if [[ -n "$current_loop" ]]; then
    losetup -d "$current_loop" >/dev/null 2>&1 || true
  fi
  current_mount=""
  current_loop=""
  set -e
}

cleanup_all() {
  cleanup_case
  rm -rf -- "$workdir"
}
trap cleanup_all EXIT INT TERM

mkdir -p -- "$RESULTS_DIR"
printf 'case\tmount_options\tworkload_status\timage_logical_bytes\timage_allocated_bytes\tsubvolumes\n' > "$RESULTS_DIR/summary.tsv"

make_base_workload() {
  local base="$1"
  python3 - "$base" "$FILE_COUNT" <<'PY'
from pathlib import Path
import sys

root = Path(sys.argv[1])
count = int(sys.argv[2])
payload = (b"hacocoon-metadata-ratio\n" * 256)[:4096]
for i in range(count):
    bucket = root / f"d{i % 64:02d}"
    bucket.mkdir(exist_ok=True)
    (bucket / f"file-{i:06d}").write_bytes(payload)
PY
}

run_snapshot_workload() {
  local mountpoint_path="$1"
  local base="$mountpoint_path/base"
  local snapshots="$mountpoint_path/snapshots"

  btrfs subvolume create "$base" >/dev/null || return 1
  mkdir -p -- "$snapshots" || return 1
  make_base_workload "$base" || return 1
  sync || return 1

  local i snapshot
  for ((i = 1; i <= SNAPSHOT_COUNT; i++)); do
    snapshot="$snapshots/snap-$(printf '%04d' "$i")"
    btrfs subvolume snapshot "$base" "$snapshot" >/dev/null || return 1
    # Small unique writes make each clone diverge while keeping the workload
    # dominated by snapshot/reflink metadata rather than bulk data copies.
    printf 'snapshot=%d\n' "$i" > "$snapshot/clone-state" || return 1
    if (( i % 8 == 0 )); then
      printf 'base-generation=%d\n' "$i" > "$base/base-state" || return 1
    fi
  done
  sync || return 1
}

for ratio in $RATIOS; do
  cleanup_case

  case "$ratio" in
    default)
      label="default"
      mount_options="compress=zstd:3,noatime,nodiscard"
      ;;
    *[!0-9]*|'')
      fail "invalid metadata_ratio value '$ratio'; use 'default' or a positive integer"
      ;;
    0)
      fail "metadata_ratio=0 is represented by the 'default' case; do not pass 0 explicitly"
      ;;
    *)
      label="ratio-$ratio"
      mount_options="compress=zstd:3,noatime,nodiscard,metadata_ratio=$ratio"
      ;;
  esac

  image="$workdir/$label.raw"
  current_mount="$workdir/mnt-$label"
  mkdir -p -- "$current_mount"
  truncate -s "$IMAGE_SIZE" "$image"
  current_loop="$(losetup --find --show "$image")"
  [[ "$current_loop" =~ ^/dev/loop[0-9]+$ ]] || fail "losetup returned unexpected device '$current_loop'"

  mkfs.btrfs -f "$current_loop" > "$RESULTS_DIR/$label.mkfs.txt"
  mount -o "$mount_options" "$current_loop" "$current_mount"

  workload_status="ok"
  if ! run_snapshot_workload "$current_mount" > "$RESULTS_DIR/$label.workload.txt" 2>&1; then
    workload_status="failed"
  fi
  sync

  btrfs filesystem usage -b "$current_mount" > "$RESULTS_DIR/$label.usage.txt" 2>&1 || true
  btrfs filesystem df -b "$current_mount" > "$RESULTS_DIR/$label.df.txt" 2>&1 || true
  findmnt -rn -o SOURCE,TARGET,FSTYPE,OPTIONS --mountpoint "$current_mount" > "$RESULTS_DIR/$label.mount.txt" 2>&1 || true

  logical_bytes="$(stat -c '%s' "$image")"
  allocated_bytes="$(( $(stat -c '%b' "$image") * 512 ))"
  subvolumes="$(btrfs subvolume list "$current_mount" 2>/dev/null | wc -l | tr -d ' ')"
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$label" "$mount_options" "$workload_status" "$logical_bytes" "$allocated_bytes" "$subvolumes" \
    >> "$RESULTS_DIR/summary.tsv"

  cleanup_case
done

cat > "$RESULTS_DIR/README.txt" <<EOF
Hacocoon Btrfs metadata_ratio experiment

Image size:      $IMAGE_SIZE
Base files:      $FILE_COUNT
Snapshots:       $SNAPSHOT_COUNT
Cases:           $RATIOS

Compare summary.tsv together with each *.usage.txt and *.df.txt file.
Do not adopt a non-default metadata_ratio from one run. Repeat on supported Host
baselines and compare metadata allocation/ENOSPC behavior, image allocated bytes,
and workload completion before changing Hacocoon's default mount policy.
EOF

printf 'Btrfs metadata_ratio experiment complete: %s\n' "$RESULTS_DIR"
cat "$RESULTS_DIR/summary.tsv"

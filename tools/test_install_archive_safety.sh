#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
installer="$repo_root/scripts/install.sh"
root="$(mktemp -d)"
outside="/tmp/hacocoon-installer-escape-$$"
trap 'rm -rf "$root" "$outside"' EXIT

mkdir -p "$root/bin"
cat >"$root/bin/gh" <<'SH'
#!/bin/sh
exit 1
SH
cat >"$root/bin/curl" <<'SH'
#!/bin/sh
set -eu
out=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      out="$2"
      shift 2
      ;;
    -H|--proto)
      shift 2
      ;;
    -fL|--tlsv1.2)
      shift
      ;;
    https://*)
      url="$1"
      shift
      ;;
    *)
      shift
      ;;
  esac
done
[ -n "$out" ] || exit 2
case "$url" in
  */haco_linux_amd64.tar.gz)
    cp "$HACO_TEST_ARCHIVE" "$out"
    ;;
  */checksums.txt)
    digest="$(sha256sum "$HACO_TEST_ARCHIVE" | awk '{print $1}')"
    printf '%s  haco_linux_amd64.tar.gz\n' "$digest" >"$out"
    ;;
  *)
    exit 3
    ;;
esac
SH
cat >"$root/bin/sudo" <<'SH'
#!/bin/sh
set -eu
[ "${1:-}" = "--" ] && shift
exec "$@"
SH
cat >"$root/bin/chown" <<'SH'
#!/bin/sh
exit 0
SH
chmod 0755 "$root/bin/gh" "$root/bin/curl" "$root/bin/sudo" "$root/bin/chown"

make_archive() {
  kind="$1"
  path="$2"
  outside_name="$(basename "$outside")"
  python3 - "$kind" "$path" "$outside_name" <<'PY'
import io
import sys
import tarfile

kind, path, outside_name = sys.argv[1:]

def regular(tf, name, payload=b"binary\n", mode=0o755):
    info = tarfile.TarInfo(name)
    info.type = tarfile.REGTYPE
    info.mode = mode
    info.size = len(payload)
    tf.addfile(info, io.BytesIO(payload))

def expected_others(tf):
    regular(tf, "haco-controller", b"controller-safe\n")
    regular(tf, "haco-host", b"haco-host-safe\n")
    regular(tf, "haco-vscode", b"vscode-safe\n")
    regular(tf, "haco-agent-host", b"agent-host-safe\n")
    regular(tf, "haco-notify", b"notify-safe\n")
    regular(tf, "haco-storage-helper", b"storage-helper-safe\n")

with tarfile.open(path, "w:gz", format=tarfile.USTAR_FORMAT) as tf:
    if kind == "valid":
        regular(tf, "haco", b"haco-safe\n")
        expected_others(tf)
    elif kind == "valid-docs":
        regular(tf, "haco", b"haco-safe\n")
        expected_others(tf)
        regular(tf, "README.md", b"readme\n", mode=0o644)
        regular(tf, "LICENSE", b"license\n", mode=0o644)
    elif kind == "traversal":
        regular(tf, "../../" + outside_name, b"OVERWRITE\n")
        expected_others(tf)
    elif kind == "absolute":
        regular(tf, "/tmp/" + outside_name, b"OVERWRITE\n")
        expected_others(tf)
    elif kind == "symlink":
        link = tarfile.TarInfo("haco")
        link.type = tarfile.SYMTYPE
        link.linkname = "../../" + outside_name
        link.mode = 0o777
        tf.addfile(link)
        expected_others(tf)
    elif kind == "hardlink":
        expected_others(tf)
        link = tarfile.TarInfo("haco")
        link.type = tarfile.LNKTYPE
        link.linkname = "haco-vscode"
        link.mode = 0o755
        tf.addfile(link)
    elif kind == "fifo":
        fifo = tarfile.TarInfo("haco")
        fifo.type = tarfile.FIFOTYPE
        fifo.mode = 0o600
        tf.addfile(fifo)
        expected_others(tf)
    elif kind == "device":
        device = tarfile.TarInfo("haco")
        device.type = tarfile.CHRTYPE
        device.devmajor = 1
        device.devminor = 3
        device.mode = 0o600
        tf.addfile(device)
        expected_others(tf)
    elif kind == "extra":
        regular(tf, "haco")
        expected_others(tf)
        regular(tf, "surprise")
    else:
        raise SystemExit("unknown fixture kind: " + kind)
PY
}

run_installer() {
  fixture="$1"
  install_dir="$2"
  helper_dir="$3"
  mkdir -p "$install_dir"
  HACO_TEST_ARCHIVE="$fixture" \
  HACO_INSTALL_DIR="$install_dir" \
  HACO_STORAGE_HELPER_INSTALL_DIR="$helper_dir" \
  HACO_ROOT="$root/haco-root" \
  HACO_INSTALL_BINARIES_ONLY=1 \
  HACO_REQUIRE_PROVENANCE=0 \
  GH_TOKEN= \
  GITHUB_TOKEN= \
  PATH="$root/bin:$PATH" \
    sh "$installer" v1.2.3
}

valid="$root/valid.tar.gz"
make_archive valid "$valid"
run_installer "$valid" "$root/install-valid" "$root/helper-valid"
grep -Fx 'haco-safe' "$root/install-valid/haco" >/dev/null
grep -Fx 'controller-safe' "$root/install-valid/haco-controller" >/dev/null
grep -Fx 'haco-host-safe' "$root/install-valid/haco-host" >/dev/null
grep -Fx 'vscode-safe' "$root/install-valid/haco-vscode" >/dev/null
grep -Fx 'agent-host-safe' "$root/install-valid/haco-agent-host" >/dev/null
grep -Fx 'notify-safe' "$root/install-valid/haco-notify" >/dev/null
grep -Fx 'storage-helper-safe' "$root/helper-valid/haco-storage-helper" >/dev/null

valid_docs="$root/valid-docs.tar.gz"
make_archive valid-docs "$valid_docs"
run_installer "$valid_docs" "$root/install-valid-docs" "$root/helper-valid-docs"
grep -Fx 'haco-safe' "$root/install-valid-docs/haco" >/dev/null
grep -Fx 'storage-helper-safe' "$root/helper-valid-docs/haco-storage-helper" >/dev/null

printf 'sentinel\n' >"$outside"
for kind in traversal absolute symlink hardlink fifo device extra; do
  fixture="$root/$kind.tar.gz"
  install_dir="$root/install-$kind"
  helper_dir="$root/helper-$kind"
  make_archive "$kind" "$fixture"
  if run_installer "$fixture" "$install_dir" "$helper_dir" >"$root/$kind.out" 2>&1; then
    echo "expected installer to reject $kind archive" >&2
    cat "$root/$kind.out" >&2
    exit 1
  fi
  for binary in haco haco-controller haco-host haco-vscode haco-agent-host haco-notify; do
    if [ -e "$install_dir/$binary" ]; then
      echo "installer wrote $binary for rejected $kind archive" >&2
      exit 1
    fi
  done
  if [ -e "$helper_dir/haco-storage-helper" ]; then
    echo "installer wrote storage helper for rejected $kind archive" >&2
    exit 1
  fi
  if [ "$(cat "$outside")" != "sentinel" ]; then
    echo "rejected $kind archive modified outside sentinel" >&2
    exit 1
  fi
done

echo "installer archive safety tests: OK"

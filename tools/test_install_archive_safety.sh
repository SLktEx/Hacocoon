#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
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
chmod 0755 "$root/bin/gh" "$root/bin/curl"

make_archive() {
  kind="$1"
  path="$2"
  outside_name="$(basename "$outside")"
  python3 - "$kind" "$path" "$outside_name" <<'PY'
import io
import sys
import tarfile

kind, path, outside_name = sys.argv[1:]

def regular(tf, name, payload=b"binary\n"):
    info = tarfile.TarInfo(name)
    info.type = tarfile.REGTYPE
    info.mode = 0o755
    info.size = len(payload)
    tf.addfile(info, io.BytesIO(payload))

with tarfile.open(path, "w:gz", format=tarfile.USTAR_FORMAT) as tf:
    if kind == "valid":
        regular(tf, "haco", b"haco-safe\n")
        regular(tf, "haco-vscode", b"vscode-safe\n")
    elif kind == "traversal":
        regular(tf, "../../" + outside_name, b"OVERWRITE\n")
        regular(tf, "haco-vscode")
    elif kind == "absolute":
        regular(tf, "/tmp/" + outside_name, b"OVERWRITE\n")
        regular(tf, "haco-vscode")
    elif kind == "symlink":
        link = tarfile.TarInfo("haco")
        link.type = tarfile.SYMTYPE
        link.linkname = "../../" + outside_name
        link.mode = 0o777
        tf.addfile(link)
        regular(tf, "haco-vscode")
    elif kind == "hardlink":
        regular(tf, "haco-vscode")
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
        regular(tf, "haco-vscode")
    elif kind == "device":
        device = tarfile.TarInfo("haco")
        device.type = tarfile.CHRTYPE
        device.devmajor = 1
        device.devminor = 3
        device.mode = 0o600
        tf.addfile(device)
        regular(tf, "haco-vscode")
    elif kind == "extra":
        regular(tf, "haco")
        regular(tf, "haco-vscode")
        regular(tf, "surprise")
    else:
        raise SystemExit("unknown fixture kind: " + kind)
PY
}

run_installer() {
  fixture="$1"
  install_dir="$2"
  mkdir -p "$install_dir"
  HACO_TEST_ARCHIVE="$fixture" \
  HACO_INSTALL_DIR="$install_dir" \
  GH_TOKEN= \
  GITHUB_TOKEN= \
  PATH="$root/bin:$PATH" \
    sh "$installer" v1.2.3
}

valid="$root/valid.tar.gz"
make_archive valid "$valid"
run_installer "$valid" "$root/install-valid"
grep -Fx 'haco-safe' "$root/install-valid/haco" >/dev/null
grep -Fx 'vscode-safe' "$root/install-valid/haco-vscode" >/dev/null

printf 'sentinel\n' >"$outside"
for kind in traversal absolute symlink hardlink fifo device extra; do
  fixture="$root/$kind.tar.gz"
  install_dir="$root/install-$kind"
  make_archive "$kind" "$fixture"
  if run_installer "$fixture" "$install_dir" >"$root/$kind.out" 2>&1; then
    echo "expected installer to reject $kind archive" >&2
    cat "$root/$kind.out" >&2
    exit 1
  fi
  if [ -e "$install_dir/haco" ] || [ -e "$install_dir/haco-vscode" ]; then
    echo "installer wrote binaries for rejected $kind archive" >&2
    exit 1
  fi
  if [ "$(cat "$outside")" != "sentinel" ]; then
    echo "rejected $kind archive modified outside sentinel" >&2
    exit 1
  fi
done

echo "installer archive safety tests: OK"

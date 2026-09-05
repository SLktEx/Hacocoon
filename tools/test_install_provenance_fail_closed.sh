#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
installer="$repo_root/scripts/install.sh"
root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT

fixture="$root/fixture"
src="$root/src"
mkdir -p "$fixture" "$src"
for binary in haco hacoq haco-controller haco-host haco-vscode haco-agent-host haco-notify; do
  printf '#!/bin/sh\necho %s\n' "$binary" > "$src/$binary"
  chmod 0755 "$src/$binary"
done
tar -czf "$fixture/haco_linux_amd64.tar.gz" -C "$src" \
  haco hacoq haco-controller haco-host haco-vscode haco-agent-host haco-notify
(cd "$fixture" && sha256sum haco_linux_amd64.tar.gz > checksums.txt)
printf '{}\n' > "$fixture/attestation-bundle.json"

make_fake_curl() {
  target="$1"
  cat > "$target" <<'SH'
#!/bin/sh
set -eu
output=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      output="$2"
      shift 2
      ;;
    -w|-H|--proto)
      shift 2
      ;;
    -f|-L|-s|-S|-fsSL|--tlsv1.2)
      shift
      ;;
    http://*|https://*)
      url="$1"
      shift
      ;;
    *)
      shift
      ;;
  esac
done
case "$url" in
  */releases/latest)
    printf 'https://github.com/SLktEx/Hacocoon/releases/tag/%s' "${HACO_TEST_LATEST_TAG:-v1.2.3}"
    exit 0
    ;;
  https://api.github.com/repos/SLktEx/Hacocoon/attestations/sha256:*)
    [ -n "$output" ] || exit 90
    case "${HACO_TEST_PROVENANCE_MODE:-ok}" in
      no-bundle)
        printf '{"attestations":[]}\n' > "$output"
        ;;
      nonhttps-bundle)
        printf '{"attestations":[{"bundle_url":"http://attestations.example.invalid/bundle.json"}]}\n' > "$output"
        ;;
      *)
        printf '{"attestations":[{"bundle_url":"https://attestations.example.invalid/bundle.json"}]}\n' > "$output"
        ;;
    esac
    exit 0
    ;;
  https://attestations.example.invalid/bundle.json)
    [ -n "$output" ] || exit 90
    cp "$HACO_TEST_FIXTURE/attestation-bundle.json" "$output"
    exit 0
    ;;
esac
[ -n "$output" ] || exit 90
name="${url##*/}"
cp "$HACO_TEST_FIXTURE/$name" "$output"
SH
  chmod +x "$target"
}

make_fake_gh() {
  target="$1"
  cat > "$target" <<'SH'
#!/bin/sh
set -eu
if [ "${1:-}" = "auth" ] && [ "${2:-}" = "status" ]; then
  [ "${HACO_TEST_GH_AUTH:-0}" = "1" ]
  exit $?
fi
if [ "${1:-}" = "release" ] && [ "${2:-}" = "download" ]; then
  dir=""
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "--dir" ]; then
      dir="$2"
      shift 2
    else
      shift
    fi
  done
  [ -n "$dir" ] || exit 91
  cp "$HACO_TEST_FIXTURE/haco_linux_amd64.tar.gz" "$dir/"
  cp "$HACO_TEST_FIXTURE/checksums.txt" "$dir/"
  exit 0
fi
if [ "${1:-}" = "attestation" ] && [ "${2:-}" = "verify" ]; then
  if [ "${3:-}" = "--help" ]; then
    exit 0
  fi
  case "${HACO_TEST_PROVENANCE_MODE:-ok}" in
    generic-fail)
      exit 1
      ;;
  esac
  predicate=0
  bundle=""
  shift 2
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --predicate-type)
        predicate=1
        shift 2
        ;;
      --bundle)
        bundle="$2"
        shift 2
        ;;
      *)
        shift
        ;;
    esac
  done
  if [ "${HACO_TEST_GH_AUTH:-0}" != "1" ]; then
    [ -n "$bundle" ] && [ -s "$bundle" ] || exit 93
  fi
  if [ "$predicate" = "1" ]; then
    case "${HACO_TEST_PROVENANCE_MODE:-ok}" in
      binding-empty)
        exit 1
        ;;
    esac
    printf '%s\n' "${HACO_TEST_BINDING_TAG:-v1.2.3}"
  fi
  exit 0
fi
exit 92
SH
  chmod +x "$target"
}

make_unsupported_gh() {
  target="$1"
  cat > "$target" <<'SH'
#!/bin/sh
exit 1
SH
  chmod +x "$target"
}

make_fake_privilege_tools() {
  bin="$1"
  cat > "$bin/sudo" <<'SH'
#!/bin/sh
set -eu
[ "${1:-}" = "--" ] && shift
exec "$@"
SH
  cat > "$bin/chown" <<'SH'
#!/bin/sh
exit 0
SH
  chmod +x "$bin/sudo" "$bin/chown"
}

run_case() {
  name="$1"
  gh_mode="$2"
  version="$3"
  provenance_mode="$4"
  binding_tag="$5"
  expect="$6"

  case_root="$root/$name"
  bin="$case_root/bin"
  install="$case_root/install"
  mkdir -p "$bin" "$install"
  make_fake_curl "$bin/curl"
  make_fake_privilege_tools "$bin"
  case "$gh_mode" in
    supported) make_fake_gh "$bin/gh" ;;
    unsupported) make_unsupported_gh "$bin/gh" ;;
    *) echo "unknown gh mode: $gh_mode" >&2; exit 2 ;;
  esac

  stdout="$case_root/stdout"
  stderr="$case_root/stderr"
  set +e
  PATH="$bin:$PATH" \
    GH_TOKEN="" \
    GITHUB_TOKEN="" \
    HACO_TEST_GH_AUTH="0" \
    HACO_TEST_FIXTURE="$fixture" \
    HACO_TEST_PROVENANCE_MODE="$provenance_mode" \
    HACO_TEST_BINDING_TAG="$binding_tag" \
    HACO_TEST_LATEST_TAG="v1.2.3" \
    HACO_INSTALL_BINARIES_ONLY="1" \
    HACO_INSTALL_DIR="$install" \
    HACO_ROOT="$case_root/haco-root" \
    sh "$installer" "$version" >"$stdout" 2>"$stderr"
  code=$?
  set -e

  if [ "$expect" = "success" ]; then
    [ "$code" -eq 0 ] || {
      echo "$name: expected success, got $code" >&2
      cat "$stderr" >&2
      exit 1
    }
    for binary in haco hacoq haco-controller haco-host haco-vscode haco-agent-host haco-notify; do
      [ -x "$install/$binary" ] || { echo "$name: missing installed $binary" >&2; exit 1; }
    done
  else
    [ "$code" -ne 0 ] || { echo "$name: expected failure" >&2; exit 1; }
    for binary in haco hacoq haco-controller haco-host haco-vscode haco-agent-host haco-notify; do
      [ ! -e "$install/$binary" ] || { echo "$name: installed $binary after trust failure" >&2; exit 1; }
    done
  fi
}

run_case valid-explicit supported v1.2.3 ok v1.2.3 success
grep -Fq 'Downloaded public GitHub attestation bundles without requiring a GitHub login.' "$root/valid-explicit/stdout"
grep -Fq 'Verified signed release binding for v1.2.3.' "$root/valid-explicit/stdout"

run_case missing-attestation-tool unsupported v1.2.3 ok v1.2.3 fail
grep -Fq "trusted provenance verification requires a GitHub CLI version with 'gh attestation verify' support" "$root/missing-attestation-tool/stderr"

run_case missing-public-bundle supported v1.2.3 no-bundle v1.2.3 fail
grep -Fq 'trusted provenance verification could not obtain public attestation bundles' "$root/missing-public-bundle/stderr"

run_case nonhttps-public-bundle supported v1.2.3 nonhttps-bundle v1.2.3 fail
grep -Fq 'refusing non-HTTPS public attestation bundle URL' "$root/nonhttps-public-bundle/stderr"

run_case invalid-build-provenance supported v1.2.3 generic-fail v1.2.3 fail
grep -Fq 'trusted build provenance verification failed' "$root/invalid-build-provenance/stderr"

run_case wrong-release-binding supported v1.2.3 ok v9.9.9 fail
grep -Fq 'signed release binding verification failed for v1.2.3' "$root/wrong-release-binding/stderr"

run_case latest-resolves-before-verify supported latest ok v1.2.3 success
grep -Fq 'Resolved latest Hacocoon release to v1.2.3.' "$root/latest-resolves-before-verify/stdout"
grep -Fq 'Verified signed release binding for v1.2.3.' "$root/latest-resolves-before-verify/stdout"

echo 'PASS: installer provenance fails closed, supports anonymous public bundles, and binds latest to an explicit tag'

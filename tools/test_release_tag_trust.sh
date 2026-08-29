#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
checker="$repo_root/tools/check_release_tag_trust.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

remote="$tmp/remote.git"
work="$tmp/work"

git init --bare "$remote" >/dev/null
git init -b main "$work" >/dev/null
git -C "$work" config user.name "Hacocoon CI"
git -C "$work" config user.email "ci@example.invalid"
git -C "$work" remote add origin "$remote"

printf 'A\n' >"$work/history.txt"
git -C "$work" add history.txt
git -C "$work" commit -m A >/dev/null
printf 'B\n' >>"$work/history.txt"
git -C "$work" commit -am B >/dev/null
printf 'C\n' >>"$work/history.txt"
git -C "$work" commit -am C >/dev/null
main_commit="$(git -C "$work" rev-parse HEAD)"
git -C "$work" push -u origin main >/dev/null

# Lightweight and annotated release tags on trusted main history must pass.
git -C "$work" tag v1.0.0 "$main_commit"
(
  cd "$work"
  bash "$checker" v1.0.0 origin main
)

git -C "$work" tag -a v1.0.1 -m v1.0.1 "$main_commit"
(
  cd "$work"
  bash "$checker" v1.0.1 origin main
)

# The repository_dispatch workflow checks the trusted control repository out
# below the workspace root. When invoked from a non-repository parent, the
# checker must anchor itself to the repository containing the checker script.
mkdir -p "$work/tools"
cp "$checker" "$work/tools/check_release_tag_trust.sh"
(
  cd "$tmp"
  bash "$work/tools/check_release_tag_trust.sh" v1.0.0 origin main
)

# A release-looking tag on an attacker-only side branch must fail.
git -C "$work" switch -c attacker HEAD~1 >/dev/null
printf 'X\n' >>"$work/history.txt"
git -C "$work" commit -am X >/dev/null
git -C "$work" tag v9.9.9
if (
  cd "$work"
  bash "$checker" v9.9.9 origin main
); then
  echo "expected attacker-side tag to be rejected" >&2
  exit 1
fi

# Invalid release-looking names also fail closed.
git -C "$work" tag release-latest
if (
  cd "$work"
  bash "$checker" release-latest origin main
); then
  echo "expected invalid release tag to be rejected" >&2
  exit 1
fi

echo "release tag trust tests: OK"

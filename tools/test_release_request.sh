#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
checker="$repo_root/tools/check_release_request.sh"
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
main_commit="$(git -C "$work" rev-parse HEAD)"
git -C "$work" push -u origin main >/dev/null

(
  cd "$work"
  bash "$checker" v1.2.3 refs/heads/main main "$main_commit" origin
)

if (
  cd "$work"
  bash "$checker" v1.2.3 refs/heads/attacker main "$main_commit" origin
); then
  echo "expected non-default dispatch ref to be rejected" >&2
  exit 1
fi

if (
  cd "$work"
  bash "$checker" release-latest refs/heads/main main "$main_commit" origin
); then
  echo "expected invalid release version to be rejected" >&2
  exit 1
fi

git -C "$work" switch -c attacker HEAD~1 >/dev/null
printf 'X\n' >>"$work/history.txt"
git -C "$work" commit -am X >/dev/null
attacker_commit="$(git -C "$work" rev-parse HEAD)"
if (
  cd "$work"
  bash "$checker" v9.9.9 refs/heads/main main "$attacker_commit" origin
); then
  echo "expected source outside trusted main history to be rejected" >&2
  exit 1
fi

if (
  cd "$work"
  bash "$checker" v1.2.3 refs/heads/main main 0000000000000000000000000000000000000000 origin
); then
  echo "expected source SHA different from checkout to be rejected" >&2
  exit 1
fi

echo "trusted release request tests: OK"

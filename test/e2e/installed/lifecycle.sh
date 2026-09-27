#!/usr/bin/env bash
set -euo pipefail

[[ "$(id -u)" != 0 ]] || { echo 'product lifecycle requires an ordinary user' >&2; exit 1; }
root="$(mktemp -d)"
name="installed-cli-$(basename "$root" | tr -cd 'a-zA-Z0-9' | tr 'A-Z' 'a-z')"
mkdir "$root/workspace"
printf 'workspace-before\n' > "$root/workspace/marker"
created=0
attempted=0
cleanup() {
  local result=$?
  trap - EXIT
  if [[ "$attempted" == 1 && "$created" == 0 && "$result" != 0 ]]; then
    echo 'creation result is uncertain; fixture retained for inspection' >&2
    exit "$result"
  fi
  if [[ "$created" == 1 ]]; then
    if ! haco rm -f "$name"; then
      echo 'product cleanup failed; fixture retained' >&2
      exit 1
    fi
  fi
  # Exact ordinary files only; do not recursively follow anything a guest wrote.
  rm -f -- "$root/workspace/marker"
  rmdir -- "$root/workspace" "$root" || result=1
  exit "$result"
}
trap cleanup EXIT
attempted=1
haco env create --no-oci --workspace "$root/workspace" "$name"
created=1
haco exec "$name" -- sh -ec 'test "$(cat /workspace/marker)" = workspace-before; printf workspace-after > /workspace/marker'
haco env status --json "$name" | python3 -c 'import json,sys; assert json.load(sys.stdin)["state"] == "running"'
haco env stop "$name"
haco env status --json "$name" | python3 -c 'import json,sys; assert json.load(sys.stdin)["state"] == "stopped"'
haco env start "$name"
haco env status --json "$name" | python3 -c 'import json,sys; assert json.load(sys.stdin)["state"] == "running"'
haco rm -f "$name"
created=0
attempted=0
[[ "$(cat "$root/workspace/marker")" == workspace-after ]]
haco env list --json | python3 -c 'import json,sys; assert all(row["name"] != sys.argv[1] for row in json.load(sys.stdin))' "$name"
echo 'PASS: packaged product CLI lifecycle and retained Workspace'

#!/usr/bin/env bash
set -euo pipefail

for command in go git grep mktemp python3; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 1; }
done

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
workspace="$root/workspace"
bare="$root/remote.git"
export HACO_ROOT="$root/haco-root"
haco="$root/haco"

git init -q --bare "$bare"
git init -q "$workspace"
git -C "$workspace" config user.email test@example.invalid
git -C "$workspace" config user.name 'Hacocoon E2E'
printf 'first\n' > "$workspace/README.md"
git -C "$workspace" add README.md
git -C "$workspace" commit -qm first
first="$(git -C "$workspace" rev-parse HEAD)"
git -C "$workspace" remote add origin https://github.com/acme/demo.git
git -C "$workspace" config url."file://$bare".insteadOf https://github.com/acme/demo.git

mkdir -p "$HACO_ROOT/state"
python3 - "$HACO_ROOT/state/environments.json" "$workspace" <<'PY'
import json,sys
path,workspace=sys.argv[1:]
data={"environments":{"demo":{"name":"demo","workspace":{"id":"path:e2e","path":workspace},"access_mode":"rw","runtime_ref":"unused","created_at":"0001-01-01T00:00:00Z"}},"workspace_leases":{}}
with open(path,"w") as f: json.dump(data,f)
PY
cat > "$HACO_ROOT/policy.json" <<'JSON'
{
  "default": "deny",
  "rules": [
    {
      "capability": "github.git",
      "action": "push",
      "attributes": {
        "organization": "acme",
        "repository": "demo",
        "target_ref": "refs/heads/feature/e2e"
      },
      "decision": "allow",
      "reason": "narrow feature push"
    },
    {
      "capability": "github.git",
      "action": "force-push",
      "attributes": {
        "organization": "acme",
        "repository": "demo",
        "target_ref": "refs/heads/main"
      },
      "decision": "require-approval",
      "reason": "protected branch force push"
    }
  ]
}
JSON

go build -o "$haco" ./cmd/haco

"$haco" git push demo --branch feature/e2e >/dev/null
[[ "$(git --git-dir="$bare" rev-parse refs/heads/feature/e2e)" == "$first" ]]

git --git-dir="$bare" update-ref refs/heads/main "$first"
printf 'second\n' > "$workspace/README.md"
git -C "$workspace" add README.md
git -C "$workspace" commit -qm second
second="$(git -C "$workspace" rev-parse HEAD)"
printf 'yes\n' | "$haco" git push demo --branch main --force >"$root/force.out" 2>"$root/force.err"
grep -Fq '[y/N]' "$root/force.err"
[[ "$(git --git-dir="$bare" rev-parse refs/heads/main)" == "$second" ]]

printf 'third\n' > "$workspace/README.md"
git -C "$workspace" add README.md
git -C "$workspace" commit -qm third
third="$(git -C "$workspace" rev-parse HEAD)"
set +e
printf 'no\n' | "$haco" git push demo --branch main --force >"$root/no.out" 2>"$root/no.err"
no_code=$?
"$haco" git push demo --branch denied >"$root/deny.out" 2>"$root/deny.err"
deny_code=$?
set -e
[[ "$no_code" != 0 && "$deny_code" != 0 ]]
[[ "$(git --git-dir="$bare" rev-parse refs/heads/main)" == "$second" ]]
[[ "$third" != "$second" ]]
if git --git-dir="$bare" show-ref --verify --quiet refs/heads/denied; then
  echo 'denied branch was pushed' >&2
  exit 1
fi

audit="$HACO_ROOT/audit/capabilities.jsonl"
[[ -f "$audit" ]]
grep -Fq '"capability":"github.git"' "$audit"
grep -Fq '"organization":"acme"' "$audit"
grep -Fq '"repository":"demo"' "$audit"
grep -Fq '"target_ref":"refs/heads/main"' "$audit"
grep -Fq '"approved":true' "$audit"
grep -Fq '"approved":false' "$audit"
grep -Fq '"decision":"deny"' "$audit"
grep -Fq "$first" "$audit"
grep -Fq "$second" "$audit"

# Hacocoon must never have needed a GH_TOKEN/PAT in the Environment for this path.
if grep -Eqi 'gh_token|github_token|authorization|password' "$audit"; then
  echo 'credential-like material leaked into audit' >&2
  exit 1
fi

echo 'PASS: Hacocoon v0.5 brokered Git/GitHub E2E'

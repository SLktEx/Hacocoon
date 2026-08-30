#!/usr/bin/env bash
set -euo pipefail

for command in go git mktemp python3; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 1; }
done

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
workspace="$root/workspace"
bare="$root/attacker.git"
export HACO_ROOT="$root/haco-root"
haco="$root/haco"

git init -q --bare "$bare"
git init -q "$workspace"
git -C "$workspace" config user.email test@example.invalid
git -C "$workspace" config user.name 'Hacocoon E2E'
printf 'first\n' > "$workspace/README.md"
git -C "$workspace" add README.md
git -C "$workspace" commit -qm first
git -C "$workspace" remote add origin https://github.com/acme/demo.git
git -C "$workspace" remote add upstream https://github.com/SLktEx/Hacocoon.git

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
      "action": "fetch",
      "resource": "github://SLktEx/Hacocoon/fetch/upstream",
      "environment": "demo",
      "attributes": {
        "organization": "SLktEx",
        "repository": "Hacocoon",
        "repository_identity": "*",
        "remote": "upstream"
      },
      "decision": "allow",
      "reason": "public Hacocoon fetch acceptance"
    },
    {
      "capability": "github.git",
      "action": "push",
      "resource": "github://acme/demo/refs/heads/feature/e2e",
      "environment": "demo",
      "attributes": {
        "organization": "acme",
        "repository": "demo",
        "repository_identity": "*",
        "remote": "origin",
        "source_sha": "*",
        "target_ref": "refs/heads/feature/e2e"
      },
      "decision": "allow",
      "reason": "narrow feature push"
    }
  ]
}
JSON

go build -o "$haco" ./cmd/haco

# A successful fetch proves the normal brokered Git user path, including policy,
# repository pinning, sanitized Host Git execution, and remote-tracking ref
# updates. The public project remote requires no test credentials.
"$haco" plugin git fetch demo --remote upstream >"$root/fetch.out" 2>"$root/fetch.err"
upstream_main="$(git -C "$workspace" rev-parse --verify refs/remotes/upstream/main^{commit})"
[[ "$upstream_main" =~ ^[0-9a-f]{40,64}$ ]]

# A repository-local URL rewrite changes the transport destination after the
# policy layer has approved github.com/acme/demo. This was previously used by
# the E2E itself to redirect the push to a local bare repository; it is exactly
# the confused-deputy path the broker must reject.
git -C "$workspace" config url."file://$bare".insteadOf https://github.com/acme/demo.git
set +e
"$haco" plugin git push demo --branch feature/e2e >"$root/rewrite.out" 2>"$root/rewrite.err"
rewrite_code=$?
set -e
[[ "$rewrite_code" != 0 ]]
if git --git-dir="$bare" show-ref --verify --quiet refs/heads/feature/e2e; then
  echo 'repository URL rewrite bypassed GitHub authorization' >&2
  exit 1
fi

# A pushurl is even more direct: policy reads remote.origin.url while git push
# normally prefers remote.origin.pushurl. It must be rejected before execution.
git -C "$workspace" config --unset-all url."file://$bare".insteadOf
git -C "$workspace" config remote.origin.pushurl "file://$bare"
set +e
"$haco" plugin git push demo --branch feature/e2e >"$root/pushurl.out" 2>"$root/pushurl.err"
pushurl_code=$?
set -e
[[ "$pushurl_code" != 0 ]]
if git --git-dir="$bare" show-ref --verify --quiet refs/heads/feature/e2e; then
  echo 'repository pushurl bypassed GitHub authorization' >&2
  exit 1
fi

echo 'PASS: Hacocoon brokered Git fetch + transport-override hardening E2E'

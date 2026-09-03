#!/usr/bin/env bash
set -euo pipefail

for command in go git gh mktemp python3; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

[[ "${GITHUB_ACTIONS:-}" == "true" ]] || {
  echo 'real GitHub push E2E only runs in GitHub Actions' >&2
  exit 1
}
case "${GITHUB_EVENT_NAME:-}" in
  push|workflow_dispatch) ;;
  *)
    echo 'real GitHub push E2E requires a trusted main push or manual workflow dispatch' >&2
    exit 1
    ;;
esac
[[ "${GITHUB_REF:-}" == "refs/heads/main" ]] || {
  echo 'real GitHub push E2E only runs for main' >&2
  exit 1
}
[[ -n "${GITHUB_REPOSITORY:-}" ]] || {
  echo 'GITHUB_REPOSITORY is required' >&2
  exit 1
}
[[ -n "${GITHUB_RUN_ID:-}" && -n "${GITHUB_RUN_ATTEMPT:-}" ]] || {
  echo 'GitHub Actions run identity is required' >&2
  exit 1
}
[[ -n "${HACO_GITHUB_TOKEN:-}" ]] || {
  echo 'HACO_GITHUB_TOKEN is required for the host-side Git broker' >&2
  exit 1
}

root="$(mktemp -d)"
workspace="$root/workspace"
export HACO_ROOT="$root/haco-root"
haco="$root/haco"
repo_slug="$GITHUB_REPOSITORY"
owner="${repo_slug%%/*}"
repo="${repo_slug#*/}"
branch="haco-e2e/${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"
denied_branch="${branch}-denied"
target_ref="refs/heads/$branch"
marker=".haco-real-push-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"

github_ref_exists() {
  local ref="$1"
  GH_TOKEN="$HACO_GITHUB_TOKEN" gh api "repos/$repo_slug/git/ref/heads/$ref" >/dev/null 2>&1
}

cleanup() {
  set +e
  GH_TOKEN="$HACO_GITHUB_TOKEN" gh api --method DELETE "repos/$repo_slug/git/refs/heads/$branch" >/dev/null 2>&1 || true
  GH_TOKEN="$HACO_GITHUB_TOKEN" gh api --method DELETE "repos/$repo_slug/git/refs/heads/$denied_branch" >/dev/null 2>&1 || true
  rm -rf "$root"
}
trap cleanup EXIT

if github_ref_exists "$branch" || github_ref_exists "$denied_branch"; then
  echo 'refusing to reuse an existing real-push E2E branch' >&2
  exit 1
fi

go build -o "$haco" ./cmd/haco

git clone -q --no-hardlinks "$GITHUB_WORKSPACE" "$workspace"
git -C "$workspace" remote set-url origin "https://github.com/$repo_slug.git"
git -C "$workspace" config user.email 'haco-e2e@users.noreply.github.com'
git -C "$workspace" config user.name 'Hacocoon E2E'
printf 'run=%s attempt=%s\n' "$GITHUB_RUN_ID" "$GITHUB_RUN_ATTEMPT" > "$workspace/$marker"
git -C "$workspace" add "$marker"
git -C "$workspace" commit -qm 'test: real Hacocoon GitHub push E2E'
expected_sha="$(git -C "$workspace" rev-parse HEAD)"

mkdir -p "$HACO_ROOT/state"
python3 - "$HACO_ROOT/state/environments.json" "$workspace" <<'PY'
import json
import sys

path, workspace = sys.argv[1:]
data = {
    "environments": {
        "demo": {
            "name": "demo",
            "workspace": {"id": "path:real-github-e2e", "path": workspace},
            "access_mode": "rw",
            "runtime_ref": "unused",
            "created_at": "0001-01-01T00:00:00Z",
        }
    },
    "workspace_leases": {},
}
with open(path, "w", encoding="utf-8") as handle:
    json.dump(data, handle)
PY

python3 - "$HACO_ROOT/policy.json" "$owner" "$repo" "$target_ref" <<'PY'
import json
import sys

path, owner, repo, target_ref = sys.argv[1:]
branch = target_ref.removeprefix("refs/heads/")
data = {
    "default": "deny",
    "rules": [
        {
            "capability": "github.git",
            "action": "push",
            "resource": f"github://{owner}/{repo}/{target_ref}",
            "environment": "demo",
            "attributes": {
                "organization": owner,
                "repository": repo,
                "repository_identity": "*",
                "remote": "origin",
                "source_sha": "*",
                "target_ref": target_ref,
            },
            "decision": "allow",
            "reason": f"real GitHub E2E push to {branch}",
        }
    ],
}
with open(path, "w", encoding="utf-8") as handle:
    json.dump(data, handle)
PY

# The policy only allows the exact ephemeral target branch. This request must
# fail before Git transport executes, and GitHub must remain unchanged.
set +e
"$haco" plugin git push demo --branch "$denied_branch" >"$root/denied.out" 2>"$root/denied.err"
denied_code=$?
set -e
if [[ "$denied_code" == 0 ]]; then
  echo 'policy unexpectedly allowed a push to the denied branch' >&2
  exit 1
fi
if github_ref_exists "$denied_branch"; then
  echo 'denied branch was created on GitHub' >&2
  exit 1
fi

# The host-side broker receives HACO_GITHUB_TOKEN, maps it only into the
# isolated Git credential path, and pushes the exact SHA approved by policy.
"$haco" plugin git push demo --branch "$branch"

actual_sha="$(GH_TOKEN="$HACO_GITHUB_TOKEN" gh api "repos/$repo_slug/git/ref/heads/$branch" --jq '.object.sha')"
if [[ "$actual_sha" != "$expected_sha" ]]; then
  echo "remote SHA mismatch: got $actual_sha, want $expected_sha" >&2
  exit 1
fi

# Credentials are process-local host authority. They must never be persisted
# in Hacocoon state/policy/audit files.
if grep -R -q -F -- "$HACO_GITHUB_TOKEN" "$HACO_ROOT"; then
  echo 'GitHub credential was persisted under HACO_ROOT' >&2
  exit 1
fi

echo "PASS: real Hacocoon GitHub push created $target_ref at $expected_sha"

#!/usr/bin/env bash
set -euo pipefail

for command in go grep mktemp; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
export HACO_ROOT="$root/haco-root"
haco="$root/haco"
mkdir -p "$HACO_ROOT"
cat > "$HACO_ROOT/policy.json" <<'JSON'
{
  "default": "deny",
  "rules": [
    {"capability":"local.echo","action":"echo","resource":"safe","decision":"allow","reason":"test allow"},
    {"capability":"local.echo","action":"echo","resource":"sensitive","decision":"require-approval","reason":"test approval"}
  ]
}
JSON

go build -o "$haco" ./cmd/haco

safe_output="$("$haco" capability request local.echo echo --resource safe --param message=hello)"
[[ "$safe_output" == "hello" ]]

approved_output="$(printf 'yes\n' | "$haco" capability request local.echo echo --resource sensitive --param message=approved-secret 2>"$root/approval.err")"
[[ "$approved_output" == "approved-secret" ]]
grep -Fq '[y/N]' "$root/approval.err"

set +e
printf 'no\n' | "$haco" capability request local.echo echo --resource sensitive --param message=must-not-run >"$root/denied.out" 2>"$root/denied.err"
denied_code=$?
"$haco" capability request local.echo echo --resource unknown --param message=default-deny >"$root/default.out" 2>"$root/default.err"
default_code=$?
set -e
[[ "$denied_code" != "0" ]]
[[ "$default_code" != "0" ]]
[[ ! -s "$root/denied.out" ]]
[[ ! -s "$root/default.out" ]]

audit="$HACO_ROOT/audit/capabilities.jsonl"
[[ -f "$audit" ]]
grep -Fq '"decision":"allow"' "$audit"
grep -Fq '"decision":"require-approval"' "$audit"
grep -Fq '"approved":true' "$audit"
grep -Fq '"approved":false' "$audit"
grep -Fq '"decision":"deny"' "$audit"
if grep -Fq 'approved-secret' "$audit" || grep -Fq 'must-not-run' "$audit"; then
  echo "capability parameters leaked into audit" >&2
  exit 1
fi

echo "PASS: Hacocoon v0.4 capability E2E"

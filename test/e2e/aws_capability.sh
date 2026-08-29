#!/usr/bin/env bash
set -euo pipefail
for command in go mktemp grep; do command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 1; }; done
root="$(mktemp -d)"; trap 'rm -rf "$root"' EXIT
bin="$root/bin"; mkdir -p "$bin" "$root/haco-root"; export PATH="$bin:$PATH"; export HACO_ROOT="$root/haco-root"; export HACO_FAKE_AWS_LOG="$root/aws.log"
cat > "$bin/aws" <<'SH'
#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$HACO_FAKE_AWS_LOG"
echo '{"Account":"123456789012","Arn":"arn:aws:iam::123456789012:user/e2e"}'
SH
chmod +x "$bin/aws"
cat > "$HACO_ROOT/policy.json" <<'JSON'
{
  "default":"deny",
  "rules":[
    {
      "capability":"aws.api",
      "action":"sts.get-caller-identity",
      "resource":"aws://ap-northeast-1/identity",
      "decision":"allow",
      "reason":"read caller identity"
    }
  ]
}
JSON
haco="$root/haco"; go build -o "$haco" ./cmd/haco
"$haco" capability request aws.api sts.get-caller-identity --resource aws://ap-northeast-1/identity > "$root/whoami.json"
grep -Fq '123456789012' "$root/whoami.json"
set +e
"$haco" capability request aws.api ec2.describe-instance --resource aws://ap-northeast-1/ec2/instance/i-0123456789abcdef0 >"$root/denied.out" 2>"$root/denied.err"
code=$?
set -e
[[ "$code" != 0 ]]
[[ ! -s "$root/denied.out" ]]
if grep -Fq 'describe-instances' "$HACO_FAKE_AWS_LOG"; then echo 'policy-denied AWS call reached provider' >&2; exit 1; fi
audit="$HACO_ROOT/audit/capabilities.jsonl"; [[ -f "$audit" ]]
grep -Fq '"capability":"aws.api"' "$audit"; grep -Fq 'aws://ap-northeast-1/identity' "$audit"; if grep -Eqi 'AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN' "$HACO_FAKE_AWS_LOG" "$audit"; then echo 'credential material leaked' >&2; exit 1; fi
echo 'PASS: Hacocoon v0.7 AWS capability E2E'

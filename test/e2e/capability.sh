#!/usr/bin/env bash
set -euo pipefail

for command in go grep mktemp sleep python3; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

source "$(dirname "$0")/controller.sh"

root="$(mktemp -d)"
cleanup() {
  set +e
  haco_stop_test_controller
  rm -rf "$root"
}
trap cleanup EXIT
export HACO_ROOT="$root/haco-root"
haco="$root/haco"
controller="$root/haco-controller"
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

chmod 600 "$HACO_ROOT/policy.json"
python3 test/e2e/environment_fixture.py "$HACO_ROOT/state/environments.json" first "$root/first"
python3 test/e2e/environment_fixture.py "$HACO_ROOT/state/environments.json" second "$root/second"

go build -o "$haco" ./cmd/haco
go build -o "$controller" ./cmd/haco-controller
haco_start_test_controller \
  "$controller" \
  "$root/control.sock" \
  "$root/controller.out" \
  "$root/controller.err"

safe_output="$("$haco" capability request local.echo echo --resource safe --param message=hello)"
[[ "$safe_output" == "hello" ]]

# Approval is collected by the client terminal, transferred as a typed decision over
# the bidirectional controller stream, then audited/executed by the controller.
approved_output="$(printf 'yes\n' | "$haco" capability request local.echo echo --resource sensitive --param message=approved-secret 2>"$root/approval.err")"
[[ "$approved_output" == "approved-secret" ]]
grep -Fq '3=allow all Environments' "$root/approval.err"

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

# A saved choice can resolve the default ask without replacing explicit rules.
python3 - "$HACO_ROOT/policy.json" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
policy = json.loads(path.read_text())
policy["default"] = "require-approval"
path.write_text(json.dumps(policy))
PY
saved_output="$(printf '1\n' | "$haco" capability request local.echo echo --resource remembered --environment first --param message=saved-choice-secret 2>"$root/saved.err")"
[[ "$saved_output" == "saved-choice-secret" ]]
replayed_output="$("$haco" capability request local.echo echo --resource remembered --environment first --param message=replayed </dev/null 2>"$root/replayed.err")"
[[ "$replayed_output" == "replayed" ]]
if grep -Fq 'Approve capability' "$root/replayed.err"; then
  echo "saved choice unexpectedly prompted again" >&2
  exit 1
fi
if "$haco" capability request local.echo echo --resource remembered --environment second --param message=must-not-run </dev/null >"$root/other.out" 2>"$root/other.err"; then
  echo "saved choice escaped its Environment" >&2
  exit 1
fi
[[ ! -s "$root/other.out" ]]
grep -Fq 'Approve capability' "$root/other.err"
python3 - "$HACO_ROOT/policy.json" "$audit" <<'PY'
import json, pathlib, sys
policy = json.loads(pathlib.Path(sys.argv[1]).read_text())
assert len(policy["rules"]) == 2, "administrator rules changed"
saved = policy["saved_decisions"]
assert len(saved) == 1, "wrong saved choice count"
assert saved[0]["environment"] == "first"
assert saved[0]["resource"] == "remembered"
assert saved[0]["decision"] == "allow"
audit = pathlib.Path(sys.argv[2]).read_text()
assert "saved-choice-secret" not in audit
assert "allow-environment" in audit
PY

ask_output="$(printf '5\nyes\n' | "$haco" capability request local.echo echo --resource ask-always --environment first --param message=ask-once 2>"$root/ask.err")"
[[ "$ask_output" == "ask-once" ]]
if "$haco" capability request local.echo echo --resource ask-always --environment first --param message=must-not-run </dev/null >"$root/ask-again.out" 2>"$root/ask-again.err"; then
  echo "saved ask unexpectedly authorized another request" >&2
  exit 1
fi
[[ ! -s "$root/ask-again.out" ]]
grep -Fq 'Approve capability' "$root/ask-again.err"
# Simulate a new canonical creation in this repository-only catalog fixture.
# The name and Workspace remain the same; the durable instance identity changes.
python3 test/e2e/environment_fixture.py "$HACO_ROOT/state/environments.json" first "$root/first"
if "$haco" capability request local.echo echo --resource remembered --environment first --param message=must-not-run </dev/null >"$root/recreated.out" 2>"$root/recreated.err"; then
  echo "saved choice escaped its creation identity" >&2
  exit 1
fi
[[ ! -s "$root/recreated.out" ]]
grep -Fq 'Approve capability' "$root/recreated.err"
echo "PASS: Hacocoon capability approval / saved scope / replay / persistent ask / recreation E2E"

#!/usr/bin/env bash
set -euo pipefail

# Assertions below use the English catalog, independent of the invoking locale.
export HACO_UI_LANGUAGE=en

for command in go grep mktemp sleep python3; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

source "$(dirname "$0")/controller.sh"

root="$(mktemp -d)"
notify_pid=""
cleanup() {
  set +e
  haco_stop_test_controller
  if [[ -n "$notify_pid" ]] && kill -0 "$notify_pid" >/dev/null 2>&1; then
    kill -TERM "$notify_pid" >/dev/null 2>&1 || true
    wait "$notify_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

bin="$root/bin"
mkdir -p "$bin" "$root/home" "$root/haco-root"
export HACO_ROOT="$root/haco-root"
unset WSL_DISTRO_NAME || true

# Base discovery reads the native image catalog. This command-only fixture has
# no Incus daemon and supplies an empty catalog; real images are covered by E2E.
cat >"$bin/incus" <<'INCUS'
#!/usr/bin/env bash
if [[ "$*" == 'query -X GET /1.0/images/aliases?project=hacocoon&recursion=1' ]]; then
  printf '[]\n'
  exit 0
fi
printf 'unexpected Incus command in command-only fixture\n' >&2
exit 1
INCUS
chmod +x "$bin/incus"
export PATH="$bin:$PATH"

go build -o "$bin/haco" ./cmd/haco
for name in haco-controller haco-vscode haco-agent-host haco-notify; do
  go build -o "$bin/$name" "./cmd/$name"
done
for name in haco haco-controller haco-vscode haco-agent-host haco-notify; do
  test -x "$bin/$name"
done

# Build with the normal Go cache; isolate HOME only for product execution.
# Go module directories are read-only and must not become disposable user data.
export HOME="$root/home"

# Only the executable entrypoint owns logging setup. Invalid configuration
# must fail once, without a package initializer silently selecting defaults.
if HACO_LOG_LEVEL=invalid "$bin/haco-controller" >"$root/controller-log.out" 2>"$root/controller-log.err"; then
  echo 'controller accepted invalid logging configuration' >&2
  exit 1
fi
[[ ! -s "$root/controller-log.out" ]]
printf '%s\n' 'controller logging configuration is invalid' >"$root/controller-log.expected"
cmp "$root/controller-log.expected" "$root/controller-log.err"

# Product identity/help must be available before any Incus/runtime/controller
# initialization. The new haco deliberately exposes no legacy namespaces yet.
"$bin/haco" --version >"$root/haco-version-short.out" 2>"$root/haco-version-short.err"
grep -Eq '^haco dev \(checkpoint v0\.[0-9]+, commit [^)]+\)$' "$root/haco-version-short.out"
[[ ! -s "$root/haco-version-short.err" ]]

"$bin/haco" version --json >"$root/haco-version.json" 2>"$root/haco-version-json.err"
grep -Eq '"checkpoint":"v0\.[0-9]+"' "$root/haco-version.json"
grep -Fq '"version":"dev"' "$root/haco-version.json"
grep -Fq '"commit":' "$root/haco-version.json"
grep -Fq '"build_date":"unknown"' "$root/haco-version.json"
[[ ! -s "$root/haco-version-json.err" ]]

"$bin/haco" help >"$root/haco-help.out" 2>"$root/haco-help.err"
grep -Fq 'Usage:' "$root/haco-help.out"
grep -Fq 'version' "$root/haco-help.out"
[[ ! -s "$root/haco-help.err" ]]

set +e
"$bin/haco" env >"$root/haco-env.out" 2>"$root/haco-env.err"
haco_env_code=$?
set -e
[[ "$haco_env_code" == "2" ]]
[[ ! -s "$root/haco-env.out" ]]
grep -Fxq 'Usage:' "$root/haco-env.err"
grep -Fxq '  haco env <command>' "$root/haco-env.err"
grep -Eq '^  create +[^ ]' "$root/haco-env.err"
"$bin/haco" env --help >"$root/haco-env-help.out" 2>"$root/haco-env-help.err"
cmp "$root/haco-env.err" "$root/haco-env-help.out"
[[ ! -s "$root/haco-env-help.err" ]]

set +e
"$bin/haco" definitely-not-a-command >"$root/haco-invalid.out" 2>"$root/haco-invalid.err"
haco_invalid_code=$?
set -e
[[ "$haco_invalid_code" == "2" ]]
[[ ! -s "$root/haco-invalid.out" ]]
grep -Fq 'command "definitely-not-a-command" is not available yet' "$root/haco-invalid.err"

# Exercise the installed controller-backed product commands.
haco_start_test_controller \
  "$bin/haco-controller" \
  "$root/control.sock" \
  "$root/controller.out" \
  "$root/controller.err"

# Product development commands use the same controller without legacy fallback.
"$bin/haco" env list >"$root/product-env-list.out"
grep -Fq 'No Environments.' "$root/product-env-list.out"
"$bin/haco" env list --json >"$root/product-env-list-json.out"
grep -Fxq '[]' "$root/product-env-list-json.out"

"$bin/haco" git pending >"$root/product-git-pending.out"
grep -Fxq '(none)' "$root/product-git-pending.out"
"$bin/haco" git pending --json >"$root/product-git-pending-json.out"
grep -Fxq '[]' "$root/product-git-pending-json.out"

"$bin/haco" approve --list >"$root/product-approval-list.out"
grep -Fxq '(none)' "$root/product-approval-list.out"
"$bin/haco" approve --list --json >"$root/product-approval-list-json.out"
grep -Fxq '[]' "$root/product-approval-list-json.out"
"$bin/haco" approve >"$root/product-approval-empty.out"
grep -Fxq 'No pending approvals.' "$root/product-approval-empty.out"
if "$bin/haco" approve stale-request >"$root/product-approval-stale.out" 2>"$root/product-approval-stale.err"; then
  echo 'stale approval unexpectedly succeeded' >&2
  exit 1
fi

# Configuration uses the shipped CLI/controller, with no provider repair or
# direct Policy write. JSON is explicit for files that are parsed or reapplied.
# Stale snapshots must not erase a newer saved document.
"$bin/haco" config >"$root/config-human.out"
grep -Fq 'revision:' "$root/config-human.out"
"$bin/haco" config --json >"$root/config-initial.json"
python3 - "$root/config-initial.json" <<'PY'
import json, sys
p = sys.argv[1]
with open(p) as f: data = json.load(f)
assert data['revision'].startswith('sha256:')
data['policy']['rules'] = [{'capability':'local.echo', 'action':'echo',
    'resource':'config-authority-marker', 'environment':'*', 'decision':'deny'}]
with open(p, 'w') as f: json.dump(data, f)
PY
"$bin/haco" config --file "$root/config-initial.json" --json >"$root/config-applied.json"
if "$bin/haco" config --file "$root/config-initial.json" --json >"$root/config-stale.out" 2>"$root/config-stale.err"; then
  echo 'stale configuration was accepted' >&2
  exit 1
fi
[[ ! -s "$root/config-stale.out" ]]
"$bin/haco" config --json >"$root/config-current.json"
cmp "$root/config-applied.json" "$root/config-current.json"
cat >"$root/editor with spaces" <<'PY'
#!/usr/bin/env python3
import json, sys
with open(sys.argv[1]) as f: data = json.load(f)
data['policy'] = {'default':'deny','rules':[]}
with open(sys.argv[1], 'w') as f: json.dump(data, f)
print('Configuration editor completed')
PY
chmod 700 "$root/editor with spaces"
VISUAL="" EDITOR="'$root/editor with spaces'" "$bin/haco" config --edit --json >"$root/config-edited.json"
python3 - "$HACO_ROOT" "$root/config-edited.json" <<'PY'
import json, pathlib, sys
root = pathlib.Path(sys.argv[1])
data = json.loads(pathlib.Path(sys.argv[2]).read_text())
assert data['policy']['default'] == 'deny' and data['policy']['rules'] == []
assert (root / 'policy.json').stat().st_mode & 0o777 == 0o600
events = [json.loads(line) for line in (root / 'audit/capabilities.jsonl').read_text().splitlines()]
events = [e for e in events if e['capability'] == 'policy.configuration']
assert [e['type'] for e in events] == ['configuration-change-requested', 'configuration-changed'] * 2
assert 'config-authority-marker' not in json.dumps(events)
PY
echo 'Configuration inspect / editor / stale-write refusal / minimized audit: PASS'
set +e
HACO_ROOT="$root/product-missing-root" HACO_CONTROL_SOCKET="$root/missing-product.sock" \
  "$bin/haco" env list >"$root/product-missing.out" 2>"$root/product-missing.err"
product_missing_code=$?
set -e
[[ "$product_missing_code" == "1" ]]
[[ ! -e "$root/product-missing-root/state" ]]

# Agent Host: release is intentionally idempotent, so a never-created session
# gives us a deterministic successful process-level path without real Incus.
"$bin/haco-agent-host" release --session e2e-never-created >"$root/agent.out" 2>"$root/agent.err"
grep -Fq 'released: haco-agent-' "$root/agent.out"
[[ ! -s "$root/agent.err" ]]

# VS Code adapter: exercise the shipped process entrypoint and its usage/error
# routing. Real environment/SSH behavior stays in the Incus/client E2E layers.
set +e
"$bin/haco-vscode" definitely-not-a-command >"$root/vscode.out" 2>"$root/vscode.err"
vscode_code=$?
set -e
[[ "$vscode_code" == "1" ]]
[[ ! -s "$root/vscode.out" ]]
grep -Fq 'usage: haco-vscode <open|delete>' "$root/vscode.err"
grep -Fq 'unknown command "definitely-not-a-command"' "$root/vscode.err"

# Browser notifier: prove the real server process can start and terminate
# cleanly on SIGTERM without requiring a desktop session.
"$bin/haco-notify" web --listen 127.0.0.1:0 >"$root/notify.out" 2>"$root/notify.err" &
notify_pid=$!
notify_ready=0
for ((attempt = 0; attempt < 50; attempt++)); do
  if grep -Eq '^Hacocoon browser notifications: http://127\.0\.0\.1:[1-9][0-9]*/$' "$root/notify.out"; then
    notify_ready=1
    break
  fi
  if ! kill -0 "$notify_pid" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done
[[ "$notify_ready" == "1" ]] || {
  echo 'haco-notify did not report a ready browser listener' >&2
  cat "$root/notify.err" >&2 || true
  exit 1
}
python3 - "$root/notify.out" <<'PY'
import json, pathlib, sys, urllib.request
line = pathlib.Path(sys.argv[1]).read_text().strip()
prefix = 'Hacocoon browser notifications: '
assert line.startswith(prefix), line
endpoint = line[len(prefix):]
# Use the address the product advertised, including the assigned ephemeral
# port. Bypass ambient proxies when observing this private local listener.
client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
with client.open(endpoint + 'api/v1/events?offset=0&limit=1', timeout=5) as response:
    assert response.status == 200
    assert response.headers['Cache-Control'] == 'no-store'
    batch = json.load(response)
    assert isinstance(batch['events'], list)
PY
kill -TERM "$notify_pid"
wait "$notify_pid"
notify_pid=""
[[ ! -s "$root/notify.err" ]]

echo 'PASS: shipped haco and helper black-box E2E'

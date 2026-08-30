#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == "--managed-ci" ]]; then
  shift
  [[ "${GITHUB_ACTIONS:-}" == "true" && "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || {
    echo 'managed egress E2E requires the GitHub-hosted CI substrate' >&2
    exit 2
  }
  [[ -n "${HACO_E2E_HACO_BIN:-}" && -n "${HACO_E2E_SHARED_ROOT:-}" ]] || {
    echo 'managed egress E2E requires the prebuilt haco binary and shared root' >&2
    exit 2
  }
elif [[ "${HACO_E2E_INCUS:-}" != "1" ]]; then
  echo "SKIP: set HACO_E2E_INCUS=1 on a supported Incus host"
  exit 0
fi
[[ "$#" == "0" ]] || { echo "usage: $0 [--managed-ci]" >&2; exit 2; }

for command in go incus grep mktemp python3 sudo; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done
incus version >/dev/null

free_port() {
  python3 - <<'PY'
import socket
s=socket.socket(); s.bind(('127.0.0.1',0)); print(s.getsockname()[1]); s.close()
PY
}

wait_tcp() {
  local host="$1" port="$2"
  for ((attempt=0; attempt<80; attempt++)); do
    if python3 - "$host" "$port" <<'PY' >/dev/null 2>&1
import socket,sys
s=socket.create_connection((sys.argv[1],int(sys.argv[2])),0.25)
s.close()
PY
    then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

root="$(mktemp -d)"
workspace="$root/workspace"
serve_dir="$root/upstream"
haco="${HACO_E2E_HACO_BIN:-$root/haco}"
environment="egress-e2e-$$"
target_ip="11.23.45.67"
target_port="$(free_port)"
allowed_host="haco-egress-e2e.test"
denied_host="denied-egress-e2e.test"
export HACO_ROOT="${HACO_E2E_SHARED_ROOT:-$root/haco-root}"
http_pid=""
eg_pid=""
created=0
ip_added=0
policy_had_original=0

cleanup() {
  local code=$?
  set +e
  if [[ "$code" != "0" ]]; then
    echo '::group::egress CLI stderr' >&2
    cat "$root/egress.err" >&2 2>/dev/null || true
    echo '::endgroup::' >&2
    echo '::group::upstream HTTP stderr' >&2
    cat "$root/http.err" >&2 2>/dev/null || true
    echo '::endgroup::' >&2
  fi
  if [[ -n "$eg_pid" ]] && kill -0 "$eg_pid" >/dev/null 2>&1; then
    kill -TERM "$eg_pid" >/dev/null 2>&1 || true
    wait "$eg_pid" >/dev/null 2>&1 || true
  fi
  if [[ -n "$http_pid" ]] && kill -0 "$http_pid" >/dev/null 2>&1; then
    kill -TERM "$http_pid" >/dev/null 2>&1 || true
    wait "$http_pid" >/dev/null 2>&1 || true
  fi
  if [[ "$created" == "1" ]]; then
    "$haco" delete "$environment" >/dev/null 2>&1 || incus delete "haco-$environment" --project hacocoon --force >/dev/null 2>&1 || true
  fi
  if [[ "$policy_had_original" == "1" ]]; then
    cp "$root/policy.before" "$HACO_ROOT/policy.json" >/dev/null 2>&1 || true
  else
    rm -f "$HACO_ROOT/policy.json"
  fi
  if [[ -f "$root/hosts.before" ]]; then
    sudo cp "$root/hosts.before" /etc/hosts >/dev/null 2>&1 || true
  fi
  if [[ "$ip_added" == "1" ]]; then
    sudo ip addr del "$target_ip/32" dev lo >/dev/null 2>&1 || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

mkdir -p "$workspace" "$serve_dir" "$HACO_ROOT"
printf 'proxy-round-trip-ok\n' >"$serve_dir/index.txt"
if [[ -e "$HACO_ROOT/policy.json" ]]; then
  cp "$HACO_ROOT/policy.json" "$root/policy.before"
  policy_had_original=1
fi
cp /etc/hosts "$root/hosts.before"

if [[ -z "${HACO_E2E_HACO_BIN:-}" ]]; then
  go build -o "$haco" ./cmd/haco
fi
[[ -x "$haco" ]]

# `doctor` is intentionally exercised through the same final executable before
# any Environment is created.
"$haco" doctor >"$root/doctor.out" 2>"$root/doctor.err"
grep -Fq 'Hacocoon Secure Workspace Runtime' "$root/doctor.out"
grep -Fq 'Incus available: true' "$root/doctor.out"

cat >"$HACO_ROOT/policy.json" <<JSON
{
  "default": "deny",
  "rules": [
    {
      "capability": "network.egress",
      "action": "connect",
      "resource": "$allowed_host",
      "environment": "$environment",
      "attributes": {"protocol": "http", "port": "$target_port"},
      "decision": "allow",
      "reason": "real CLI egress acceptance"
    }
  ]
}
JSON

# Give the trusted Host a local endpoint whose address is public-looking to the
# proxy's SSRF guard. /etc/hosts keeps this fully local and deterministic while
# still exercising Host DNS resolution and pinned-address dialing.
sudo ip addr add "$target_ip/32" dev lo
ip_added=1
printf '%s %s\n' "$target_ip" "$allowed_host" | sudo tee -a /etc/hosts >/dev/null
python3 -m http.server "$target_port" --bind "$target_ip" --directory "$serve_dir" >"$root/http.out" 2>"$root/http.err" &
http_pid=$!
wait_tcp "$target_ip" "$target_port" || { echo 'upstream HTTP test server did not start' >&2; exit 1; }

"$haco" create --workspace "$workspace" "$environment" >/dev/null
created=1
bridge_cidr="$(incus network get haco-sandbox0 ipv4.address --project default)"
bridge_ip="${bridge_cidr%/*}"
[[ -n "$bridge_ip" && "$bridge_ip" != "$bridge_cidr" ]]

"$haco" egress serve >"$root/egress.out" 2>"$root/egress.err" &
eg_pid=$!
wait_tcp "$bridge_ip" 18080 || { echo 'haco egress serve did not open the managed proxy listener' >&2; exit 1; }

proxy_request() {
  local hostname="$1"
  "$haco" exec "$environment" -- bash -c \
    "exec 3<>/dev/tcp/$bridge_ip/18080; printf 'GET http://$hostname:$target_port/index.txt HTTP/1.1\\r\\nHost: $hostname:$target_port\\r\\nConnection: close\\r\\n\\r\\n' >&3; cat <&3"
}

allowed_response="$(proxy_request "$allowed_host")"
printf '%s' "$allowed_response" | grep -Fq '200 OK'
printf '%s' "$allowed_response" | grep -Fq 'proxy-round-trip-ok'

denied_response="$(proxy_request "$denied_host")"
printf '%s' "$denied_response" | grep -Fq '403 Forbidden'
printf '%s' "$denied_response" | grep -Fq 'egress denied'

audit="$HACO_ROOT/audit/capabilities.jsonl"
[[ -f "$audit" ]]
grep -Fq "$allowed_host" "$audit"
grep -Fq '"decision":"allow"' "$audit"
grep -Fq "$denied_host" "$audit"
grep -Fq '"decision":"deny"' "$audit"

kill -TERM "$eg_pid"
wait "$eg_pid" >/dev/null 2>&1 || true
eg_pid=""
if wait_tcp "$bridge_ip" 18080; then
  echo 'egress proxy listener remained after process termination' >&2
  exit 1
fi

"$haco" delete "$environment"
created=0

echo 'PASS: haco doctor + egress serve -> real Incus E2E'

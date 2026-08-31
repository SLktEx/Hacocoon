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
  echo 'SKIP: set HACO_E2E_INCUS=1 on a supported Incus host'
  exit 0
fi
[[ "$#" == "0" ]] || { echo "usage: $0 [--managed-ci]" >&2; exit 2; }

for command in bash go grep incus ip mktemp python3 sudo; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done
incus version >/dev/null

free_port() {
  local host="$1"
  python3 - "$host" <<'PY'
import socket, sys
s = socket.socket()
s.bind((sys.argv[1], 0))
print(s.getsockname()[1])
s.close()
PY
}

wait_tcp() {
  local host="$1" port="$2"
  for ((attempt = 0; attempt < 80; attempt++)); do
    if python3 - "$host" "$port" <<'PY' >/dev/null 2>&1
import socket, sys
s = socket.create_connection((sys.argv[1], int(sys.argv[2])), 0.25)
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
environment="egress-process-e2e-$$"
target_ip="11.23.45.67"
target_port=""
allowed_host="haco-egress-process-e2e.test"
denied_host="denied-egress-process-e2e.test"
export HACO_ROOT="${HACO_E2E_SHARED_ROOT:-$root/haco-root}"
http_pid=""
eg_pid=""
created=0
ip_added=0
policy_had_original=0

wait_environment_route() {
  local host="$1"
  for ((attempt = 0; attempt < 60; attempt++)); do
    if "$haco" exec "$environment" -- sh -c "ip -4 route get '$host' >/dev/null 2>&1" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  echo "Environment never gained an IPv4 route to egress proxy $host" >&2
  "$haco" exec "$environment" -- sh -c 'ip -4 address show; echo ROUTES; ip -4 route show' >&2 || true
  return 1
}

require_response_contains() {
  local label="$1" response="$2" needle="$3"
  if printf '%s' "$response" | grep -Fq "$needle"; then
    return 0
  fi
  echo "$label response missing $needle:" >&2
  printf '%s\n' "$response" >&2
  if [[ -f "$HACO_ROOT/audit/capabilities.jsonl" ]]; then
    echo 'capability audit:' >&2
    cat "$HACO_ROOT/audit/capabilities.jsonl" >&2 || true
  fi
  return 1
}

cleanup() {
  local code=$?
  set +e
  if [[ "$code" != "0" ]]; then
    echo '::group::egress CLI stdout/stderr' >&2
    cat "$root/egress.out" >&2 2>/dev/null || true
    cat "$root/egress.err" >&2 2>/dev/null || true
    echo '::endgroup::' >&2
    echo '::group::upstream HTTP stdout/stderr' >&2
    cat "$root/http.out" >&2 2>/dev/null || true
    cat "$root/http.err" >&2 2>/dev/null || true
    echo '::endgroup::' >&2
    if [[ -f "$HACO_ROOT/audit/capabilities.jsonl" ]]; then
      echo '::group::capability audit' >&2
      cat "$HACO_ROOT/audit/capabilities.jsonl" >&2 || true
      echo '::endgroup::' >&2
    fi
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

# Healthy doctor path through the final executable and the same real Incus
# substrate used for the server acceptance below.
"$haco" doctor >"$root/doctor.out" 2>"$root/doctor.err"
grep -Fq 'Hacocoon Secure Workspace Runtime' "$root/doctor.out"
grep -Fq 'Incus available: true' "$root/doctor.out"
[[ ! -s "$root/doctor.err" ]]

# Give the trusted Host a runner-local endpoint with a public-looking address.
# This keeps the test deterministic while preserving production SSRF checks.
sudo ip addr add "$target_ip/32" dev lo
ip_added=1
target_port="$(free_port "$target_ip")"
printf '%s %s\n' "$target_ip" "$allowed_host" | sudo tee -a /etc/hosts >/dev/null
python3 -m http.server "$target_port" --bind "$target_ip" --directory "$serve_dir" >"$root/http.out" 2>"$root/http.err" &
http_pid=$!
wait_tcp "$target_ip" "$target_port" || { echo 'upstream HTTP test server did not start' >&2; exit 1; }

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
      "reason": "process-level HTTP egress acceptance"
    },
    {
      "capability": "network.egress",
      "action": "connect",
      "resource": "$allowed_host",
      "environment": "$environment",
      "attributes": {"protocol": "https", "port": "$target_port"},
      "decision": "allow",
      "reason": "process-level CONNECT egress acceptance"
    }
  ]
}
JSON
chmod 0600 "$HACO_ROOT/policy.json"

"$haco" create --base haco/ubuntu-26.04 --workspace "$workspace" "$environment" >/dev/null
created=1
bridge_cidr="$(incus network get haco-sandbox0 ipv4.address --project default)"
bridge_ip="${bridge_cidr%/*}"
[[ -n "$bridge_ip" && "$bridge_ip" != "$bridge_cidr" ]]

# Run the shipped command as a real background server and observe its listener
# from outside the process.
"$haco" egress serve >"$root/egress.out" 2>"$root/egress.err" &
eg_pid=$!
wait_tcp "$bridge_ip" 18080 || { echo 'haco egress serve did not open the managed proxy listener' >&2; exit 1; }

# Incus can report the instance running before DHCP has installed the guest
# route. Match the production real-egress acceptance and wait for that route
# instead of weakening the managed ACL or racing the first proxy request.
wait_environment_route "$bridge_ip"

proxy_request() {
  local hostname="$1"
  "$haco" exec "$environment" -- timeout 10 bash -lc \
    "exec 3<>/dev/tcp/$bridge_ip/18080; printf 'GET http://$hostname:$target_port/index.txt HTTP/1.1\\r\\nHost: $hostname:$target_port\\r\\nConnection: close\\r\\n\\r\\n' >&3; cat <&3"
}

proxy_connect() {
  local hostname="$1"
  "$haco" exec "$environment" -- timeout 10 bash -lc \
    "exec 3<>/dev/tcp/$bridge_ip/18080; printf 'CONNECT $hostname:$target_port HTTP/1.1\\r\\nHost: $hostname:$target_port\\r\\nConnection: close\\r\\n\\r\\n' >&3; IFS= read -r line <&3; printf '%s\\n' \"\$line\"; if [[ \"\$line\" == *'200 Connection Established'* ]]; then printf '\\x00' >&3; fi"
}

allowed_response="$(proxy_request "$allowed_host")"
require_response_contains 'allowed HTTP' "$allowed_response" '200 OK'
require_response_contains 'allowed HTTP' "$allowed_response" 'proxy-round-trip-ok'

denied_response="$(proxy_request "$denied_host")"
require_response_contains 'denied HTTP' "$denied_response" '403 Forbidden'
require_response_contains 'denied HTTP' "$denied_response" 'egress denied'

# CONNECT is part of the Standard proxy contract. The E2E checks the real
# process-level authorization/upgrade boundary; TLS ClientHello/SNI forwarding
# remains covered by the focused proxy tests.
allowed_connect="$(proxy_connect "$allowed_host")"
require_response_contains 'allowed CONNECT' "$allowed_connect" '200 Connection Established'

denied_connect="$(proxy_connect "$denied_host")"
require_response_contains 'denied CONNECT' "$denied_connect" '403 Forbidden'

# Policy decisions must remain externally auditable through the production
# capability audit sink.
audit="$HACO_ROOT/audit/capabilities.jsonl"
[[ -f "$audit" ]]
grep -Fq "$allowed_host" "$audit"
grep -Fq '"decision":"allow"' "$audit"
grep -Fq "$denied_host" "$audit"
grep -Fq '"decision":"deny"' "$audit"

kill -TERM "$eg_pid"
set +e
wait "$eg_pid"
eg_code=$?
set -e
eg_pid=""
[[ "$eg_code" == "0" || "$eg_code" == "143" ]]
if wait_tcp "$bridge_ip" 18080; then
  echo 'egress proxy listener remained after process termination' >&2
  exit 1
fi

"$haco" delete "$environment"
created=0

echo 'PASS: haco doctor + egress serve process E2E on real Incus'

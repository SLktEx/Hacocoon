#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == "--managed-ci" ]]; then
  shift
  [[ "${GITHUB_ACTIONS:-}" == "true" && "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || {
    echo 'managed client access E2E requires the GitHub-hosted CI substrate' >&2
    exit 2
  }
  [[ -n "${HACO_E2E_HACO_BIN:-}" && -n "${HACO_E2E_SHARED_ROOT:-}" ]] || {
    echo 'managed client access E2E requires the prebuilt haco binary and shared root' >&2
    exit 2
  }
elif [[ "${HACO_E2E_INCUS:-}" != "1" ]]; then
  echo "SKIP: set HACO_E2E_INCUS=1 on a supported Incus host"
  exit 0
fi
[[ "$#" == "0" ]] || { echo "usage: $0 [--managed-ci]" >&2; exit 2; }

for command in go incus ssh ssh-keygen python3; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done
incus version >/dev/null

free_port() {
  python3 - <<'PY'
import socket
s = socket.socket()
s.bind(('127.0.0.1', 0))
print(s.getsockname()[1])
s.close()
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
haco="${HACO_E2E_HACO_BIN:-$root/haco}"
environment="client-e2e-$$"
ssh_port="$(free_port)"
forward_port="$(free_port)"
export HACO_ROOT="${HACO_E2E_SHARED_ROOT:-$root/haco-root}"
created=0
forwarded=0
egress_pid=""
policy_had_original=0

restore_policy() {
  if [[ "$policy_had_original" == "1" ]]; then
    cp "$root/policy.before" "$HACO_ROOT/policy.json"
  else
    rm -f "$HACO_ROOT/policy.json"
  fi
}

cleanup() {
  local code=$?
  set +e
  if [[ "$code" != "0" && -f "$root/ssh-egress.err" ]]; then
    echo '::group::SSH bootstrap egress stderr' >&2
    cat "$root/ssh-egress.err" >&2 || true
    echo '::endgroup::' >&2
  fi
  if [[ -n "$egress_pid" ]] && kill -0 "$egress_pid" >/dev/null 2>&1; then
    kill -TERM "$egress_pid" >/dev/null 2>&1 || true
    wait "$egress_pid" >/dev/null 2>&1 || true
  fi
  restore_policy >/dev/null 2>&1 || true
  if [[ "$forwarded" == "1" ]]; then
    "$haco" unforward "$environment" "tcp-$forward_port-22" >/dev/null 2>&1 || true
  fi
  if [[ "$created" == "1" ]]; then
    "$haco" delete "$environment" >/dev/null 2>&1 || incus delete "haco-$environment" --project hacocoon --force >/dev/null 2>&1 || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

mkdir -p "$workspace" "$HACO_ROOT"
printf 'client-e2e\n' > "$workspace/probe.txt"
ssh-keygen -q -t ed25519 -N '' -f "$root/id_ed25519"
if [[ -z "${HACO_E2E_HACO_BIN:-}" ]]; then
  go build -o "$haco" ./cmd/haco
fi
[[ -x "$haco" ]]
if [[ -e "$HACO_ROOT/policy.json" ]]; then
  cp "$HACO_ROOT/policy.json" "$root/policy.before"
  policy_had_original=1
fi

"$haco" create --workspace "$workspace" "$environment" >/dev/null
created=1

status_json="$("$haco" status "$environment" --json)"
printf '%s' "$status_json" | grep -q '"state":"running"'
printf '%s' "$status_json" | grep -Fq "$workspace"

# Stock Bases may omit sshd. In that case exercise the production bootstrap
# path itself immediately after create: temporarily authorize package egress,
# start the real managed proxy, and let `haco ssh` wait for sandbox routing and
# install openssh-server through the proxy environment injected by the managed
# Incus profile. The dedicated real-Incus egress E2E owns source-resolution and
# policy semantics; this block proves the actual user-facing SSH composition.
needs_ssh_bootstrap=0
if ! "$haco" exec "$environment" -- sh -c 'command -v sshd >/dev/null 2>&1'; then
  needs_ssh_bootstrap=1
  bridge_cidr="$(incus network get haco-sandbox0 ipv4.address --project default)"
  bridge_ip="${bridge_cidr%/*}"
  [[ -n "$bridge_ip" && "$bridge_ip" != "$bridge_cidr" ]]

  proxy_value="$("$haco" exec "$environment" -- sh -c 'printf %s "${http_proxy:-}"')"
  [[ "$proxy_value" == "http://$bridge_ip:18080/" ]] || {
    echo "unexpected sandbox proxy URI: $proxy_value" >&2
    exit 1
  }

  cat >"$HACO_ROOT/policy.json" <<'JSON'
{
  "default": "allow",
  "rules": []
}
JSON
  "$haco" egress serve >"$root/ssh-egress.out" 2>"$root/ssh-egress.err" &
  egress_pid=$!
  wait_tcp "$bridge_ip" 18080 || {
    echo 'temporary SSH bootstrap egress proxy did not start' >&2
    cat "$root/ssh-egress.err" >&2 || true
    exit 1
  }
fi

ssh_command="$("$haco" ssh "$environment" --public-key "$root/id_ed25519.pub" --host-port "$ssh_port")"
[[ "$ssh_command" == "ssh -p $ssh_port root@127.0.0.1" ]]

if [[ "$needs_ssh_bootstrap" == "1" ]]; then
  "$haco" exec "$environment" -- sh -c 'command -v sshd >/dev/null 2>&1'
  kill -TERM "$egress_pid"
  wait "$egress_pid" >/dev/null 2>&1 || true
  egress_pid=""
  restore_policy
fi

# From here onward there is no external-network authority. Client commands use
# only the running Environment and loopback-only Incus proxy devices.
connections_json="$("$haco" connections "$environment" --json)"
python3 - "$connections_json" "$ssh_port" <<'PY'
import json,sys
rows=json.loads(sys.argv[1]); port=int(sys.argv[2])
match=[r for r in rows if r.get('port') == port and r.get('target_port') == 22]
assert len(match) == 1, rows
assert match[0].get('user') == 'root', match[0]
assert match[0].get('command') == f'ssh -p {port} root@127.0.0.1', match[0]
PY

ssh -i "$root/id_ed25519" -p "$ssh_port" \
  -o BatchMode=yes \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o ConnectTimeout=10 \
  root@127.0.0.1 -- sh -c "printf remote-ssh-ok" | grep -q remote-ssh-ok

# Exercise a generic local forward without installing any additional package in
# the Environment: the SSH service established above is already a real TCP
# endpoint on guest port 22.
forward_output="$("$haco" forward "$environment" --host-port "$forward_port" --target-port 22)"
printf '%s' "$forward_output" | grep -Fq "tcp://127.0.0.1:$forward_port"
forwarded=1

connections_json="$("$haco" connections "$environment" --json)"
python3 - "$connections_json" "$ssh_port" "$forward_port" <<'PY'
import json,sys
rows=json.loads(sys.argv[1]); ssh_port=int(sys.argv[2]); forward_port=int(sys.argv[3])
ports={(r.get('port'), r.get('target_port')) for r in rows}
assert (ssh_port, 22) in ports, rows
assert (forward_port, 22) in ports, rows
PY

ssh -i "$root/id_ed25519" -p "$forward_port" \
  -o BatchMode=yes \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o ConnectTimeout=10 \
  root@127.0.0.1 -- sh -c "printf forwarded-ssh-ok" | grep -q forwarded-ssh-ok

"$haco" unforward "$environment" "tcp-$forward_port-22"
forwarded=0
connections_json="$("$haco" connections "$environment" --json)"
python3 - "$connections_json" "$ssh_port" "$forward_port" <<'PY'
import json,sys
rows=json.loads(sys.argv[1]); ssh_port=int(sys.argv[2]); forward_port=int(sys.argv[3])
ports={(r.get('port'), r.get('target_port')) for r in rows}
assert (ssh_port, 22) in ports, rows
assert (forward_port, 22) not in ports, rows
PY

set +e
ssh -i "$root/id_ed25519" -p "$forward_port" \
  -o BatchMode=yes \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o ConnectTimeout=2 \
  root@127.0.0.1 -- true >/dev/null 2>&1
removed_forward_code=$?
set -e
[[ "$removed_forward_code" != "0" ]]

"$haco" delete "$environment"
created=0

echo "PASS: Hacocoon client access CLI -> real Incus E2E"

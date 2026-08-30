#!/usr/bin/env bash
set -euo pipefail

if [[ "${HACO_E2E_INCUS:-}" != "1" ]]; then
  echo "SKIP: set HACO_E2E_INCUS=1 on a supported Incus host"
  exit 0
fi

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

root="$(mktemp -d)"
workspace="$root/workspace"
haco="${HACO_E2E_HACO_BIN:-$root/haco}"
environment="client-e2e-$$"
ssh_port="$(free_port)"
forward_port="$(free_port)"
export HACO_ROOT="${HACO_E2E_SHARED_ROOT:-$root/haco-root}"
created=0
forwarded=0

cleanup() {
  set +e
  if [[ "$forwarded" == "1" ]]; then
    "$haco" unforward "$environment" "tcp-$forward_port-22" >/dev/null 2>&1 || true
  fi
  if [[ "$created" == "1" ]]; then
    "$haco" delete "$environment" >/dev/null 2>&1 || incus delete "haco-$environment" --project hacocoon --force >/dev/null 2>&1 || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

mkdir -p "$workspace"
printf 'client-e2e\n' > "$workspace/probe.txt"
ssh-keygen -q -t ed25519 -N '' -f "$root/id_ed25519"
if [[ -z "${HACO_E2E_HACO_BIN:-}" ]]; then
  go build -o "$haco" ./cmd/haco
fi
[[ -x "$haco" ]]

"$haco" create --workspace "$workspace" "$environment" >/dev/null
created=1

status_json="$("$haco" status "$environment" --json)"
printf '%s' "$status_json" | grep -q '"state":"running"'
printf '%s' "$status_json" | grep -Fq "$workspace"

ssh_command="$("$haco" ssh "$environment" --public-key "$root/id_ed25519.pub" --host-port "$ssh_port")"
[[ "$ssh_command" == "ssh -p $ssh_port root@127.0.0.1" ]]

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

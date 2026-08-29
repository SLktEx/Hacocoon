#!/usr/bin/env bash
set -euo pipefail

if [[ "${HACO_E2E_INCUS:-}" != "1" ]]; then
  echo "SKIP: set HACO_E2E_INCUS=1 on a supported Incus host"
  exit 0
fi

for command in go incus ssh ssh-keygen curl python3; do
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
haco="$root/haco"
environment="client-e2e-$$"
ssh_port="$(free_port)"
http_port="$(free_port)"
target_http=18080
export HACO_ROOT="$root/haco-root"
created=0
forwarded=0

cleanup() {
  set +e
  if [[ "$forwarded" == "1" ]]; then
    "$haco" unforward "$environment" "tcp-$http_port-$target_http" >/dev/null 2>&1 || true
  fi
  if [[ "$created" == "1" ]]; then
    "$haco" delete "$environment" >/dev/null 2>&1 || incus delete "haco-$environment" --project hacocoon --force >/dev/null 2>&1 || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

mkdir -p "$workspace"
printf 'client-e2e\n' > "$workspace/index.html"
ssh-keygen -q -t ed25519 -N '' -f "$root/id_ed25519"
go build -o "$haco" ./cmd/haco

"$haco" create --workspace "$workspace" "$environment" >/dev/null
created=1

status_json="$("$haco" status "$environment" --json)"
printf '%s' "$status_json" | grep -q '"state":"running"'
printf '%s' "$status_json" | grep -Fq "$workspace"

ssh_command="$("$haco" ssh "$environment" --public-key "$root/id_ed25519.pub" --host-port "$ssh_port")"
[[ "$ssh_command" == "ssh -p $ssh_port root@127.0.0.1" ]]

ssh -i "$root/id_ed25519" -p "$ssh_port" \
  -o BatchMode=yes \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o ConnectTimeout=10 \
  root@127.0.0.1 -- sh -c "printf remote-ssh-ok" | grep -q remote-ssh-ok

"$haco" exec "$environment" -- sh -ceu \
  "apt-get update >/dev/null && apt-get install -y --no-install-recommends python3 >/dev/null; nohup python3 -m http.server $target_http --bind 127.0.0.1 --directory /workspace >/tmp/haco-http.log 2>&1 &"

"$haco" forward "$environment" --host-port "$http_port" --target-port "$target_http" >/dev/null
forwarded=1
curl --fail --retry 20 --retry-delay 1 "http://127.0.0.1:$http_port/index.html" | grep -q client-e2e

"$haco" unforward "$environment" "tcp-$http_port-$target_http"
forwarded=0
"$haco" delete "$environment"
created=0

echo "PASS: Hacocoon v0.3 client access E2E"

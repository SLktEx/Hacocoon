#!/usr/bin/env bash
set -euo pipefail

if [[ "${HACO_E2E_INCUS:-}" != "1" ]]; then
  echo "SKIP: set HACO_E2E_INCUS=1 on a supported, prepared Incus/controller host"
  exit 0
fi
for command in haco incus ssh ssh-keygen curl python3; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 1; }
done
# Uses the installed product/controller, including its existing Base and Policy.
# This acceptance never repairs installation or silently changes Policy.
haco="$(command -v haco)"
root="$(mktemp -d)"
environment="client-e2e-$$"
created=0
cleanup() {
  local result=$?
  trap - EXIT
  if [[ "$created" == 1 ]]; then
    if ! "$haco" env delete "$environment"; then
      echo "FAIL: fixture cleanup needs recovery; retained $root" >&2
      exit 1
    fi
  fi
  rm -rf -- "$root"
  exit "$result"
}
trap cleanup EXIT
mkdir -p "$root/workspace"
printf 'client-e2e\n' > "$root/workspace/index.html"
ssh-keygen -q -t ed25519 -N '' -f "$root/id_ed25519"
"$haco" env create --workspace "$root/workspace" --no-oci "$environment"
created=1
"$haco" env ssh --key "$root/id_ed25519.pub" --json "$environment" > "$root/connection.json"
python3 - "$root" "$environment" "$haco" <<'PY'
import base64,json,pathlib,sys
root,name,haco=sys.argv[1:]; p=pathlib.Path(root)
c=json.loads((p/'connection.json').read_text())
assert c['kind']=='ssh' and not c['host'] and c['port']==0 and c['target_port']==22
assert c['target']['environment']==name and c['target']['grant']==c['id']
token=base64.urlsafe_b64encode(json.dumps(c['target'],separators=(',',':')).encode()).decode().rstrip('=')
# Go's target field order matches the response object order.
(p/'known_hosts').write_text(f"haco-{name} {c['host_public_key']}\n")
(p/'config').write_text(f'''Host haco-{name}
 HostName haco-{name}
 User root
 IdentityFile {root}/id_ed25519
 IdentitiesOnly yes
 StrictHostKeyChecking yes
 UserKnownHostsFile {root}/known_hosts
 HostKeyAlias haco-{name}
 ProxyCommand {haco} stream {token}
''')
PY
ssh -G -F "$root/config" "haco-$environment" > "$root/effective-config"
grep -q '^proxycommand ' "$root/effective-config"
ssh -F "$root/config" -o BatchMode=yes "haco-$environment" cat /workspace/index.html | grep -qx client-e2e
"$haco" env stop "$environment"
# First contact after stop is ordinary OpenSSH; no setup/start command intervenes.
ssh -F "$root/config" -o BatchMode=yes "haco-$environment" cat /workspace/index.html | grep -qx client-e2e
ssh -F "$root/config" -o BatchMode=yes "haco-$environment" \
  'command -v python3 >/dev/null; nohup python3 -m http.server 18080 --bind 127.0.0.1 --directory /workspace >/tmp/haco-http.log 2>&1 </dev/null &'
"$haco" env forward --target-port 18080 --json "$environment" > "$root/forward.json"
read -r forward_id port < <(python3 -c 'import json,sys;c=json.load(open(sys.argv[1]));print(c["id"],c["port"])' "$root/forward.json")
curl --fail --retry 20 --retry-delay 1 "http://127.0.0.1:$port/index.html" | grep -q client-e2e
"$haco" env disconnect "$environment" "$forward_id"
echo "PASS: installed controller SSH stream, stopped resume and independent user forwarding"

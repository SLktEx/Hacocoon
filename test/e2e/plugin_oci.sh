#!/usr/bin/env bash
set -euo pipefail

for command in go grep mktemp python3; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
bin="$root/bin"
workspace="$root/workspace"
export HACO_ROOT="$root/haco-root"
export HACO_STORAGE_PRIVILEGE_MODE=direct
export PATH="$bin:$PATH"
mkdir -p "$bin" "$workspace" "$HACO_ROOT/state"

haco="$root/haco"
go build -o "$haco" ./cmd/haco

digest="sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
reference="example.invalid/hacocoon/e2e:latest"
identity="$reference@$digest"

# Disabled-by-default is part of the plugin boundary and must remain visible at
# the final CLI process.
unset HACO_PLUGIN_OCI
set +e
"$haco" plugin oci status >"$root/disabled.out" 2>"$root/disabled.err"
disabled_code=$?
set -e
[[ "$disabled_code" != "0" ]]
grep -Fq 'OCI plugin is disabled' "$root/disabled.err"

# The process matrix uses a deterministic Incus boundary. Heavy runtime
# behavior is covered separately; this suite guarantees every shipped OCI CLI
# route, JSON contract, state transition, and failure path is wired through the
# actual haco executable.
cat >"$bin/incus" <<SH
#!/bin/sh
set -u
printf '%s\n' "\$*" >>'$root/incus.log'
command_name="\${1:-}"
[ "\$#" -gt 0 ] && shift
case "\$command_name" in
  version)
    printf '%s\n' '6.12-oci-e2e'
    ;;
  project)
    case "\${1:-}" in
      show|create) exit 0 ;;
      *) exit 2 ;;
    esac
    ;;
  image)
    case "\${1:-}" in
      info)
        printf '%s\n' '{"fingerprint":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}'
        ;;
      list)
        printf '%s\n' '[]'
        ;;
      delete)
        exit 0
        ;;
      *) exit 2 ;;
    esac
    ;;
  list)
    printf '%s\n' '[]'
    ;;
  exec)
    while [ "\$#" -gt 0 ] && [ "\$1" != '--' ]; do shift; done
    [ "\$#" -gt 0 ] && shift
    [ "\$#" -gt 0 ] || exit 2
    if [ "\${1:-}" = docker ] && [ "\${2:-}" = images ]; then
      printf '%s\t%s\t%s\n' 'example.invalid/hacocoon/e2e' 'latest' '$digest'
      exit 0
    fi
    if [ "\${1:-}" = docker ] && [ "\${2:-}" = image ] && [ "\${3:-}" = rm ]; then
      printf '%s\n' 'removed'
      exit 0
    fi
    if [ "\${1:-}" = /bin/sh ] && [ "\${2:-}" = -c ]; then
      cat <<'PROBE'
docker_cli	1
dockerd	1
containerd	1
systemctl	1
docker_group	1
socket_unit_sha256	absent
service_unit_sha256	absent
socket_enabled	0
socket_active	0
engine_active	0
containerd_active	1
vendor_docker_enabled	0
vendor_docker_active	0
PROBE
      exit 0
    fi
    exit 2
    ;;
  *) exit 2 ;;
esac
SH
chmod +x "$bin/incus"

cat >"$HACO_ROOT/state/environments.json" <<JSON
{
  "environments": {
    "oci-demo": {
      "name": "oci-demo",
      "workspace": {"id": "path:oci-e2e", "path": "$workspace"},
      "access_mode": "rw",
      "runtime_ref": "haco-oci-demo",
      "created_at": "0001-01-01T00:00:00Z"
    }
  },
  "workspace_leases": {}
}
JSON

export HACO_PLUGIN_OCI=docker
status_output="$("$haco" plugin oci status)"
[[ "$status_output" == 'driver: docker' ]]

sample_json="$("$haco" plugin oci seed sample --json)"
python3 - "$sample_json" <<'PY'
import json,sys
row=json.loads(sys.argv[1])
assert row['sampled'] == 1, row
assert row['failed'] == 0, row
PY

recommend_json="$("$haco" plugin oci seed recommend --json)"
python3 - "$recommend_json" "$reference" "$digest" <<'PY'
import json,sys
row=json.loads(sys.argv[1]); ref=sys.argv[2]; digest=sys.argv[3]
assert row['sampling']['fresh'] == 1, row
recs=row['recommendations']
assert len(recs) == 1, recs
assert recs[0]['reference'] == ref and recs[0]['digest'] == digest, recs
PY

delete_json="$("$haco" plugin oci image delete "$identity" --all-environments --json)"
python3 - "$delete_json" "$reference" "$digest" <<'PY'
import json,sys
row=json.loads(sys.argv[1])
assert row['reference'] == sys.argv[2] and row['digest'] == sys.argv[3], row
assert row['seed_rebuild_required'] is True, row
assert row['removed_environments'] == ['oci-demo'], row
PY

reenable_json="$("$haco" plugin oci image reenable "$identity" --json)"
python3 - "$reenable_json" <<'PY'
import json,sys
row=json.loads(sys.argv[1]); assert row['removed'] is True, row
PY

pin_json="$("$haco" plugin oci seed pin "$identity" --json)"
python3 - "$pin_json" "$identity" <<'PY'
import json,sys
row=json.loads(sys.argv[1]); assert row['image']['reference']+'@'+row['image']['digest'] == sys.argv[2], row
PY
pins_json="$("$haco" plugin oci seed pins --json)"
python3 - "$pins_json" <<'PY'
import json,sys
rows=json.loads(sys.argv[1]); assert len(rows) == 1, rows
PY
unpin_json="$("$haco" plugin oci seed unpin "$identity" --json)"
python3 - "$unpin_json" <<'PY'
import json,sys
row=json.loads(sys.argv[1]); assert row['removed'] is True, row
PY
pins_json="$("$haco" plugin oci seed pins --json)"
python3 - "$pins_json" <<'PY'
import json,sys
assert json.loads(sys.argv[1]) == [], sys.argv[1]
PY

gc_json="$("$haco" plugin oci seed gc --json)"
recover_json="$("$haco" plugin oci seed recover --json)"
python3 - "$gc_json" "$recover_json" <<'PY'
import json,sys
for raw in sys.argv[1:]:
    row=json.loads(raw)
    assert not row.get('failures'), row
PY

set +e
"$haco" plugin oci seed current --json >"$root/current.out" 2>"$root/current.err"
current_code=$?
"$haco" plugin oci seed build --json >"$root/build.out" 2>"$root/build.err"
build_code=$?
set -e
[[ "$current_code" != "0" ]]
grep -Fq 'not found' "$root/current.err"
[[ "$build_code" != "0" ]]
grep -Fq 'requires HACO_PLUGIN_OCI=nerdctl' "$root/build.err"

docker_status_json="$("$haco" plugin oci docker status oci-demo --json)"
python3 - "$docker_status_json" <<'PY'
import json,sys
row=json.loads(sys.argv[1])
assert row['environment'] == 'oci-demo', row
assert row['docker_cli'] and row['dockerd'] and row['containerd'] and row['systemd'] and row['docker_group'], row
assert row['ready'] is False, row
PY
set +e
"$haco" plugin oci docker prepare oci-demo --json >"$root/docker-prepare.out" 2>"$root/docker-prepare.err"
docker_prepare_code=$?
set -e
[[ "$docker_prepare_code" != "0" ]]
grep -Fq 'systemd units are missing or differ' "$root/docker-prepare.err"

# Make the matrix obvious when a future command is added: every currently
# shipped leaf route appears above and its provider calls are visible here.
grep -Fq 'exec haco-oci-demo' "$root/incus.log"
grep -Fq 'image info images:ubuntu/26.04' "$root/incus.log"
grep -Fq 'image list --project hacocoon --format json' "$root/incus.log"

echo 'PASS: haco plugin oci process-level command matrix'

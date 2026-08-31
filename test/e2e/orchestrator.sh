#!/usr/bin/env bash
set -euo pipefail

for command in go python3 mktemp grep sed; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 1; }
done

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
bin="$root/bin"
state="$root/incus-state"
workspace="$root/workspace"
haco="$root/haco"
mkdir -p "$bin" "$state" "$workspace"
export HACO_ROOT="$root/haco-root"
export HACO_FAKE_INCUS_STATE="$state"
export HACO_FAKE_INCUS_LOG="$root/incus.log"
export HACO_INCUS_BASES_JSON='{"my-dev":"images:custom-moving"}'
export PATH="$bin:$PATH"

# The orchestration E2E already models Incus rather than requiring a privileged
# daemon. Model the local sparse-raw block/Btrfs boundary as well so the test can
# verify managed-pool selection without requiring loop/mount privileges on the
# GitHub-hosted runner.
cat > "$bin/losetup" <<'SH'
#!/bin/sh
set -u
state="$HACO_FAKE_INCUS_STATE"
case "${1:-}" in
  --version)
    echo 'losetup fake'
    ;;
  -j)
    path="${2:-}"
    if [ -f "$state/loop-path" ] && [ "$(cat "$state/loop-path")" = "$path" ]; then
      printf '/dev/loop-haco: []: (%s)\n' "$path"
    fi
    ;;
  --find)
    [ "${2:-}" = '--show' ] || exit 2
    path="${3:-}"
    [ -n "$path" ] || exit 2
    printf '%s\n' "$path" > "$state/loop-path"
    printf '%s\n' '/dev/loop-haco'
    ;;
  -c)
    exit 0
    ;;
  -d)
    rm -f "$state/loop-path"
    ;;
  *) exit 2 ;;
esac
SH

cat > "$bin/blkid" <<'SH'
#!/bin/sh
set -u
state="$HACO_FAKE_INCUS_STATE"
if [ -f "$state/btrfs-formatted" ]; then
  printf '%s\n' 'btrfs'
  exit 0
fi
exit 2
SH

cat > "$bin/mkfs.btrfs" <<'SH'
#!/bin/sh
set -u
: > "$HACO_FAKE_INCUS_STATE/btrfs-formatted"
SH

cat > "$bin/findmnt" <<'SH'
#!/bin/sh
set -u
state="$HACO_FAKE_INCUS_STATE"
if [ -f "$state/mount-device" ]; then
  cat "$state/mount-device"
  exit 0
fi
exit 1
SH

cat > "$bin/mount" <<'SH'
#!/bin/sh
set -u
[ "$#" -ge 2 ] || exit 2
printf '%s\n' "$1" > "$HACO_FAKE_INCUS_STATE/mount-device"
SH

chmod +x "$bin/losetup" "$bin/blkid" "$bin/mkfs.btrfs" "$bin/findmnt" "$bin/mount"

cat > "$bin/incus" <<'SH'
#!/bin/sh
set -u
state="$HACO_FAKE_INCUS_STATE"
printf '%s\n' "$*" >> "$HACO_FAKE_INCUS_LOG"
command_name="${1:-}"
[ "$#" -gt 0 ] && shift
config_file() {
  instance="$1"
  key="$2"
  safe_key="$(printf '%s' "$key" | sed 's/[^A-Za-z0-9_-]/_/g')"
  printf '%s/config-%s-%s' "$state" "$instance" "$safe_key"
}
case "$command_name" in
  version)
    echo '6.12-fake'
    ;;
  project)
    action="${1:-}"; project="${2:-}"
    case "$action" in
      show) [ -f "$state/project-$project" ] ;;
      create) : > "$state/project-$project" ;;
      *) exit 2 ;;
    esac
    ;;
  storage)
    action="${1:-}"; pool="${2:-}"
    case "$action" in
      show) [ -f "$state/storage-$pool" ] ;;
      create)
        [ -n "$pool" ] || exit 2
        : > "$state/storage-$pool"
        ;;
      *) exit 2 ;;
    esac
    ;;
  profile)
    if [ "${1:-}" = 'show' ] && [ "${2:-}" = 'default' ]; then
      printf '%s\n' '{"devices":{"root":{"type":"disk","path":"/","pool":"default"}}}'
      exit 0
    fi
    if [ "${1:-}" = 'show' ] && [ "${2:-}" = 'haco-sandbox' ]; then
      printf '%s\n' '{"config":{"environment.HTTP_PROXY":"http://10.200.0.1:18080","environment.HTTPS_PROXY":"http://10.200.0.1:18080","environment.NO_PROXY":"localhost,127.0.0.1,::1","environment.http_proxy":"http://10.200.0.1:18080","environment.https_proxy":"http://10.200.0.1:18080","environment.no_proxy":"localhost,127.0.0.1,::1"},"devices":{"eth0":{"type":"nic","name":"eth0","network":"haco-sandbox0","security.ipv4_filtering":"true","security.ipv6_filtering":"true","security.mac_filtering":"true","security.port_isolation":"true"}}}'
      exit 0
    fi
    exit 2
    ;;
  network)
    case "${1:-}" in
      show)
        [ "${2:-}" = 'haco-sandbox0' ] || exit 2
        exit 0
        ;;
      get)
        [ "${2:-}" = 'haco-sandbox0' ] || exit 2
        case "${3:-}" in
          ipv4.address) printf '%s\n' '10.200.0.1/24' ;;
          ipv4.nat|ipv4.firewall|ipv4.routing) printf '%s\n' 'true' ;;
          ipv6.address) printf '%s\n' 'none' ;;
          raw.dnsmasq) printf '%s\n' 'port=0' ;;
          security.acls|security.acls.default.ingress.action|security.acls.default.egress.action|security.acls.default.ingress.logged|security.acls.default.egress.logged)
            file="$(config_file 'network-haco-sandbox0' "${3:-}")"
            [ -f "$file" ] && cat "$file"
            ;;
          *) exit 2 ;;
        esac
        exit 0
        ;;
      set)
        [ "${2:-}" = 'haco-sandbox0' ] || exit 2
        assignment="${3:-}"
        key="${assignment%%=*}"; value="${assignment#*=}"
        [ -n "$key" ] && [ "$assignment" != "$key" ] || exit 2
        printf '%s\n' "$value" > "$(config_file 'network-haco-sandbox0' "$key")"
        exit 0
        ;;
      acl)
        if [ "${2:-}" = 'show' ] && [ "${3:-}" = 'haco-sandbox-egress' ]; then
          printf '%s\n' \
            'config: {}' \
            'description: ""' \
            'egress:' \
            '- action: allow' \
            '  state: enabled' \
            '  destination: 10.200.0.1/32' \
            '  protocol: tcp' \
            '  destination_port: "18080"' \
            '  description: Hacocoon Standard egress proxy' \
            'ingress: []' \
            'name: haco-sandbox-egress'
          exit 0
        fi
        exit 2
        ;;
      *) exit 2 ;;
    esac
    ;;
  image)
    if [ "${1:-}" = 'info' ] && [ -n "${2:-}" ]; then
      if [ "${2:-}" = 'images:custom-moving' ]; then
        printf '%s\n' '{"fingerprint":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}'
      else
        printf '%s\n' '{"fingerprint":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}'
      fi
      exit 0
    fi
    exit 2
    ;;
  init|launch)
    image="${1:-}"; instance="${2:-}"
    [ -n "$image" ] && [ -n "$instance" ] || exit 2
    echo 'STOPPED' > "$state/instance-$instance"
    ;;
  config)
    case "${1:-}" in
      set)
        instance="${2:-}"; assignment="${3:-}"
        key="${assignment%%=*}"; value="${assignment#*=}"
        [ -n "$instance" ] && [ -n "$key" ] && [ "$assignment" != "$key" ] || exit 2
        printf '%s\n' "$value" > "$(config_file "$instance" "$key")"
        exit 0
        ;;
      get)
        instance="${2:-}"; key="${3:-}"
        file="$(config_file "$instance" "$key")"
        [ -f "$file" ] && cat "$file"
        exit 0
        ;;
      device)
        case "${2:-}" in
          add)
            instance="${3:-}"; device="${4:-}"; kind="${5:-}"
            case "$device:$kind" in
              eth0:nic)
                network=''
                for arg in "$@"; do
                  case "$arg" in network=*) network="${arg#network=}" ;; esac
                done
                [ "$network" = 'haco-sandbox0' ] || exit 2
                : > "$state/nic-$instance"
                exit 0
                ;;
              workspace:disk)
                source_path=''
                for arg in "$@"; do
                  case "$arg" in source=*) source_path="${arg#source=}" ;; esac
                done
                [ -n "$source_path" ] || exit 2
                printf '%s\n' "$source_path" > "$state/workspace-$instance"
                exit 0
                ;;
              *) exit 2 ;;
            esac
            ;;
          set)
            instance="${3:-}"; device="${4:-}"; assignment="${5:-}"
            key="${assignment%%=*}"; value="${assignment#*=}"
            [ -n "$instance" ] && [ -n "$device" ] && [ -n "$key" ] && [ "$assignment" != "$key" ] || exit 2
            printf '%s\n' "$value" > "$(config_file "$instance" "$device.$key")"
            exit 0
            ;;
          get)
            instance="${3:-}"; device="${4:-}"; key="${5:-}"
            file="$(config_file "$instance" "$device.$key")"
            [ -f "$file" ] && cat "$file"
            exit 0
            ;;
        esac
        ;;
    esac
    exit 2
    ;;
  start)
    instance="${1:-}"
    echo 'RUNNING' > "$state/instance-$instance"
    ;;
  list)
    instance="${1:-}"
    [ -f "$state/instance-$instance" ] || exit 0
    column=''
    previous=''
    for arg in "$@"; do
      if [ "$previous" = '-c' ]; then column="$arg"; fi
      previous="$arg"
    done
    case "$column" in
      n) printf '%s\n' "$instance" ;;
      s|*) cat "$state/instance-$instance" ;;
    esac
    ;;
  delete)
    instance="${1:-}"
    rm -f "$state/instance-$instance" "$state/workspace-$instance" "$state/nic-$instance" "$state"/config-"$instance"-* 2>/dev/null || true
    ;;
  exec)
    instance="${1:-}"
    shift
    while [ "$#" -gt 0 ] && [ "$1" != '--' ]; do shift; done
    [ "$#" -gt 0 ] && shift
    [ "$#" -gt 0 ] || exit 2
    workspace="$(cat "$state/workspace-$instance")"
    executable="$1"; shift
    case "$executable" in
      sh)
        [ "${1:-}" = '-c' ] || exit 2
        script="${2:-}"
        translated="$(printf '%s' "$script" | sed "s#/workspace#$workspace#g")"
        sh -c "$translated"
        ;;
      cat)
        target="$(printf '%s' "${1:-}" | sed "s#^/workspace#$workspace#")"
        cat "$target"
        ;;
      test)
        if [ "${1:-}" = '-w' ] && [ "${2:-}" = '/workspace' ]; then
          test -w "$workspace"
        else
          test "$@"
        fi
        ;;
      *) "$executable" "$@" ;;
    esac
    ;;
  *) exit 2 ;;
esac
SH
chmod +x "$bin/incus"

go build -o "$haco" ./cmd/haco

# v0.11 Base catalog: public names stay provider-neutral and inspect resolves
# the current logical source to an immutable revision.
"$haco" base list > "$root/bases.txt"
grep -Fxq 'haco/ubuntu-24.04' "$root/bases.txt"
grep -Fxq 'haco/ubuntu-26.04' "$root/bases.txt"
grep -Fxq 'my-dev' "$root/bases.txt"
base_info="$($haco base inspect my-dev --json)"
python3 - "$base_info" <<'PY'
import json,sys
r=json.loads(sys.argv[1])
assert r['name'] == 'my-dev', r
assert r['revision'] == 'sha256:' + ('b' * 64), r
PY

"$haco" create --base my-dev --workspace "$workspace" base-demo >/dev/null
status_json="$($haco status base-demo --json)"
python3 - "$status_json" <<'PY'
import json,sys
r=json.loads(sys.argv[1])
env=r['environment']
assert env['base']['name'] == 'my-dev', r
assert env['base']['revision'] == 'sha256:' + ('b' * 64), r
assert env['resources']['cpu']['mode'] == 'unlimited', r
PY
grep -Fq 'image info images:custom-moving --format json' "$HACO_FAKE_INCUS_LOG"
grep -Fq 'storage create haco-local-default btrfs source=' "$HACO_FAKE_INCUS_LOG"
grep -Fq 'init images:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb haco-base-demo' "$HACO_FAKE_INCUS_LOG"
grep -Fq -- '--no-profiles --storage haco-local-default' "$HACO_FAKE_INCUS_LOG"
grep -Fq 'config device add haco-base-demo eth0 nic name=eth0 network=haco-sandbox0' "$HACO_FAKE_INCUS_LOG"
if grep -Fq -- '--profile haco-sandbox' "$HACO_FAKE_INCUS_LOG"; then
  echo 'managed local orchestration unexpectedly inherited the sandbox profile' >&2
  exit 1
fi
if grep -Fq 'profile show default --project default --format json' "$HACO_FAKE_INCUS_LOG"; then
  echo 'managed local orchestration unexpectedly consulted the Incus default root pool' >&2
  exit 1
fi
"$haco" delete base-demo

# v0.12 resource budgets: CLI values become persisted provider-neutral metadata,
# while the Incus adapter applies and verifies provider-native limits before start.
"$haco" create --cpu 2 --memory 512MiB --pids 64 --root-size 8GiB --workspace "$workspace" resource-demo >/dev/null
resource_status="$($haco status resource-demo --json)"
python3 - "$resource_status" <<'PY'
import json,sys
r=json.loads(sys.argv[1])['environment']['resources']
assert r['cpu'] == {'mode':'finite','value':2}, r
assert r['memory_bytes'] == {'mode':'finite','value':512 * 1024 * 1024}, r
assert r['pids'] == {'mode':'finite','value':64}, r
assert r['root_bytes'] == {'mode':'finite','value':8 * 1024 * 1024 * 1024}, r
PY
grep -Fq 'config set haco-resource-demo limits.cpu=2 --project hacocoon' "$HACO_FAKE_INCUS_LOG"
grep -Fq 'config set haco-resource-demo limits.memory=536870912B --project hacocoon' "$HACO_FAKE_INCUS_LOG"
grep -Fq 'config set haco-resource-demo limits.processes=64 --project hacocoon' "$HACO_FAKE_INCUS_LOG"
grep -Fq 'config device set haco-resource-demo root size=8589934592B --project hacocoon' "$HACO_FAKE_INCUS_LOG"
last_limit_line="$(grep -n 'config device get haco-resource-demo root size --project hacocoon' "$HACO_FAKE_INCUS_LOG" | tail -1 | cut -d: -f1)"
start_line="$(grep -n '^start haco-resource-demo --project hacocoon$' "$HACO_FAKE_INCUS_LOG" | tail -1 | cut -d: -f1)"
[[ -n "$last_limit_line" && -n "$start_line" && "$last_limit_line" -lt "$start_line" ]]
"$haco" delete resource-demo

json="$($haco run --cpu 1 --memory 256MiB --pids 32 --workspace "$workspace" --json -- sh -c "printf 'agent-ok\\n'; printf 'from-run\\n' > /workspace/result.txt")"
python3 - "$json" <<'PY'
import json,sys
r=json.loads(sys.argv[1])
assert r['environment'].startswith('run-'), r
assert r['execution']['exit_code'] == 0, r
assert r['execution']['stdout'] == 'agent-ok\n', r
assert r['execution']['stderr'] == '', r
assert r['cleaned_up'] is True, r
PY
[[ "$(cat "$workspace/result.txt")" == 'from-run' ]]
run_name="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["environment"])' "$json")"
grep -Fq "image info images:ubuntu/26.04 --format json" "$HACO_FAKE_INCUS_LOG"
grep -Fq "init images:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa haco-$run_name" "$HACO_FAKE_INCUS_LOG"
grep -Fq "config set haco-$run_name limits.cpu=1 --project hacocoon" "$HACO_FAKE_INCUS_LOG"
grep -Fq "config set haco-$run_name limits.memory=268435456B --project hacocoon" "$HACO_FAKE_INCUS_LOG"
grep -Fq "config set haco-$run_name limits.processes=32 --project hacocoon" "$HACO_FAKE_INCUS_LOG"
grep -Fq "delete haco-$run_name" "$HACO_FAKE_INCUS_LOG"
[[ ! -e "$state/instance-haco-$run_name" ]]

set +e
"$haco" run --workspace "$workspace" -- sh -c "printf 'run-error\\n' >&2; exit 17" >"$root/run.out" 2>"$root/run.err"
run_code=$?
set -e
[[ "$run_code" == 17 ]]
grep -Fq 'run-error' "$root/run.err"
# Both successful and failed ephemeral runs must clean up.
[[ "$(grep -c '^delete haco-run-' "$HACO_FAKE_INCUS_LOG")" -ge 2 ]]

mkdir -p "$HACO_ROOT"
cat > "$HACO_ROOT/policy.json" <<'JSON'
{"default":"deny","rules":[{"capability":"local.echo","action":"echo","resource":"*","environment":"agent-run","decision":"require-approval","reason":"security approval test"}]}
JSON
printf 'yes\n' | "$haco" capability request local.echo echo --environment agent-run --param message=hello >/dev/null 2>"$root/approval.err"
"$haco" events --json > "$root/events.jsonl"
python3 - "$root/events.jsonl" <<'PY'
import json,sys
rows=[json.loads(line) for line in open(sys.argv[1]) if line.strip()]
assert rows, rows
assert all(r['source']=='capability' for r in rows), rows
policy=next(r for r in rows if r['type']=='policy-decision' and r.get('decision')=='require-approval')
approval=next(r for r in rows if r['type']=='approval-decision' and r.get('approved') is True)
assert policy.get('request_id'), rows
assert policy['request_id'] == approval.get('request_id'), rows
raw=open(sys.argv[1]).read().lower()
assert 'parameters' not in raw
assert 'message' not in raw
PY

echo 'PASS: Hacocoon v0.6/v0.11/v0.12/v0.13 orchestration, Base, resource, storage, and sandbox-network E2E'

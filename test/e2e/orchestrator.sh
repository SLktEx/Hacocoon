#!/usr/bin/env bash
set -euo pipefail

for command in go python3 mktemp grep sed sleep tail cut; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 1; }
done

source "$(dirname "$0")/controller.sh"

root="$(mktemp -d)"
cleanup() {
  set +e
  haco_stop_test_controller
  rm -rf "$root"
}
trap cleanup EXIT
bin="$root/bin"
state="$root/incus-state"
workspace="$root/workspace"
haco="$root/haco"
controller="$root/haco-controller"
mkdir -p "$bin" "$state" "$workspace"
export HACO_ROOT="$root/haco-root"
export HACO_FAKE_INCUS_STATE="$state"
export HACO_FAKE_INCUS_LOG="$root/incus.log"
export HACO_FAKE_NFT_LOG="$root/nft.log"
export HACO_INCUS_BASES_JSON='{"my-dev":"images:custom-moving"}'
export PATH="$bin:$PATH"

# Model the privileged Host authority without mutating the GitHub runner.
cat > "$bin/sudo" <<'SH'
#!/bin/sh
set -eu
[ "${1:-}" = '-n' ] && shift
[ "${1:-}" = '--' ] && shift
[ "$#" -gt 0 ] || exit 2
exec "$@"
SH

cat > "$bin/ip" <<'SH'
#!/bin/sh
set -eu
case "$*" in
  '-o -4 address show'|'-o -4 address show dev lo')
    printf '%s\n' '1: lo    inet 169.254.254.1/32 scope host lo'
    ;;
  'address add 169.254.254.1/32 dev lo')
    exit 0
    ;;
  *) exit 2 ;;
esac
SH

cat > "$bin/nft" <<'SH'
#!/bin/sh
set -eu
state="$HACO_FAKE_INCUS_STATE"
printf '%s\n' "$*" >> "$HACO_FAKE_NFT_LOG"
command_name="${1:-}"
case "$command_name" in
  list)
    [ "${2:-}" = table ] && [ "${3:-}" = inet ] || exit 2
    table="${4:-}"
    if [ "$table" = hacocoon_sandbox ]; then
      cat <<'EOF'
table inet hacocoon_sandbox {
	chain input {
		type filter hook input priority -200; policy accept;
		iifname "hbr*" ct state established,related accept
		iifname "hbr*" udp sport 68 udp dport 67 accept
		iifname "hbr*" ip daddr 169.254.254.1 tcp dport 18080 accept
		iifname "hbr*" drop
	}
	chain forward {
		type filter hook forward priority -200; policy accept;
		iifname "hbr*" ip daddr 255.255.255.255 udp sport 68 udp dport 67 accept
		oifname "hbr*" udp sport 67 udp dport 68 accept
		iifname "hbr*" drop
		oifname "hbr*" drop
	}
}
EOF
      exit 0
    fi
    case "$table" in
      haco_guard_*)
        marker="$state/nft-$table"
        [ -f "$marker" ] || { echo 'Error: No such file or directory' >&2; exit 1; }
        iface="$(cat "$marker-iface")"
        mac="$(cat "$marker-mac")"
        subnet="$(cat "$marker-subnet")"
        [ -f "$marker-dhcp" ] || exit 2
        cat <<EOF
table inet $table {
	chain prerouting {
		type filter hook prerouting priority raw; policy accept;
		iifname "$iface" ether saddr != $mac drop
		iifname "$iface" ip saddr 0.0.0.0 udp sport 68 udp dport 67 accept
		iifname "$iface" ip saddr != $subnet drop
	}
}
EOF
        exit 0
        ;;
    esac
    exit 2
    ;;
  add)
    kind="${2:-}"
    [ "${3:-}" = inet ] || exit 2
    table="${4:-}"
    case "$kind:$table" in
      table:haco_guard_*)
        : > "$state/nft-$table"
        exit 0
        ;;
      chain:haco_guard_*)
        [ "${5:-}" = prerouting ] || exit 2
        exit 0
        ;;
      rule:haco_guard_*)
        [ "${5:-}" = prerouting ] && [ "${6:-}" = iifname ] || exit 2
        iface="$(printf '%s' "${7:-}" | tr -d '"')"
        printf '%s\n' "$iface" > "$state/nft-$table-iface"
        if [ "${8:-}" = ether ] && [ "${9:-}" = saddr ] && [ "${10:-}" = '!=' ] && [ "${12:-}" = drop ]; then
          printf '%s\n' "${11:-}" > "$state/nft-$table-mac"
          exit 0
        fi
        if [ "${8:-}" = ip ] && [ "${9:-}" = saddr ] && [ "${10:-}" = '0.0.0.0' ] && [ "${11:-}" = udp ] && [ "${12:-}" = sport ] && [ "${13:-}" = 68 ] && [ "${14:-}" = udp ] && [ "${15:-}" = dport ] && [ "${16:-}" = 67 ] && [ "${17:-}" = accept ]; then
          : > "$state/nft-$table-dhcp"
          exit 0
        fi
        if [ "${8:-}" = ip ] && [ "${9:-}" = saddr ] && [ "${10:-}" = '!=' ] && [ "${12:-}" = drop ]; then
          printf '%s\n' "${11:-}" > "$state/nft-$table-subnet"
          exit 0
        fi
        exit 2
        ;;
    esac
    exit 2
    ;;
  delete)
    [ "${2:-}" = table ] && [ "${3:-}" = inet ] || exit 2
    table="${4:-}"
    case "$table" in
      haco_guard_*) rm -f "$state/nft-$table" "$state/nft-$table-"*; exit 0 ;;
    esac
    exit 2
    ;;
  *) exit 2 ;;
esac
SH
chmod +x "$bin/sudo" "$bin/ip" "$bin/nft"

cat > "$bin/incus" <<'SH'
#!/bin/sh
set -u
state="$HACO_FAKE_INCUS_STATE"
printf '%s\n' "$*" >> "$HACO_FAKE_INCUS_LOG"
command_name="${1:-}"
[ "$#" -gt 0 ] && shift
config_file() {
  instance="$1"; key="$2"
  safe_key="$(printf '%s' "$key" | sed 's/[^A-Za-z0-9_-]/_/g')"
  printf '%s/config-%s-%s' "$state" "$instance" "$safe_key"
}
case "$command_name" in
  version) echo '6.12-fake' ;;
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
        [ "$pool" = haco-local-default ] && [ "${3:-}" = btrfs ] || exit 2
        saw_size=0
        saw_mount_options=0
        for arg in "$@"; do
          case "$arg" in
            source=*) exit 2 ;;
            size=128GiB) saw_size=1 ;;
            btrfs.mount_options=compress=zstd:3,noatime,nodiscard) saw_mount_options=1 ;;
          esac
        done
        [ "$saw_size:$saw_mount_options" = 1:1 ] || exit 2
        printf '%s\n' 'compress=zstd:3,noatime,nodiscard' > "$state/storage-$pool"
        ;;
      get)
        [ "${3:-}" = btrfs.mount_options ] || exit 2
        cat "$state/storage-$pool"
        ;;
      *) exit 2 ;;
    esac
    ;;
  profile)
    if [ "${1:-}" = show ] && [ "${2:-}" = default ]; then
      printf '%s\n' '{"devices":{"root":{"type":"disk","path":"/","pool":"default"}}}'
      exit 0
    fi
    exit 2
    ;;
  network)
    action="${1:-}"; name="${2:-}"
    case "$action" in
      show)
        case "$name" in
          hbr*) [ -f "$state/network-$name" ] || exit 1; printf '%s\n' 'managed: true' ;;
          *) exit 2 ;;
        esac
        ;;
      create)
        case "$name" in hbr*) ;; *) exit 2 ;; esac
        : > "$state/network-$name"
        for arg in "$@"; do
          case "$arg" in
            user.hacocoon.owner=*) printf '%s\n' "${arg#user.hacocoon.owner=}" > "$state/network-owner-$name" ;;
          esac
        done
        ;;
      get)
        case "$name" in hbr*) ;; *) exit 2 ;; esac
        [ -f "$state/network-$name" ] || exit 1
        case "${3:-}" in
          user.hacocoon.owner) [ -f "$state/network-owner-$name" ] && cat "$state/network-owner-$name" || exit 2 ;;
          ipv4.address) printf '%s\n' '10.240.0.1/24' ;;
          ipv4.nat) printf '%s\n' 'false' ;;
          ipv4.firewall|ipv4.dhcp|ipv4.routing) printf '%s\n' 'true' ;;
          ipv6.address) printf '%s\n' 'none' ;;
          raw.dnsmasq) printf '%s\n' 'port=0' ;;
          *) exit 2 ;;
        esac
        ;;
      delete)
        case "$name" in hbr*) rm -f "$state/network-$name" "$state/network-owner-$name"; exit 0 ;; *) exit 2 ;; esac
        ;;
      *) exit 2 ;;
    esac
    ;;
  image)
    if [ "${1:-}" = info ] && [ -n "${2:-}" ]; then
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
    echo STOPPED > "$state/instance-$instance"
    ;;
  config)
    case "${1:-}" in
      set)
        instance="${2:-}"; assignment="${3:-}"
        key="${assignment%%=*}"; value="${assignment#*=}"
        [ -n "$instance" ] && [ -n "$key" ] && [ "$assignment" != "$key" ] || exit 2
        printf '%s\n' "$value" > "$(config_file "$instance" "$key")"
        ;;
      get)
        instance="${2:-}"; key="${3:-}"
        file="$(config_file "$instance" "$key")"
        [ -f "$file" ] && cat "$file"
        ;;
      device)
        case "${2:-}" in
          add)
            instance="${3:-}"; device="${4:-}"; kind="${5:-}"
            case "$device:$kind" in
              eth0:nic)
                network=''; hwaddr=''; isolation=''
                for arg in "$@"; do
                  case "$arg" in
                    network=*) network="${arg#network=}" ;;
                    hwaddr=*) hwaddr="${arg#hwaddr=}" ;;
                    security.port_isolation=*) isolation="${arg#security.port_isolation=}" ;;
                  esac
                done
                case "$network" in hbr*) ;; *) exit 2 ;; esac
                [ -f "$state/network-$network" ] || exit 2
                [ -n "$hwaddr" ] && [ "$isolation" = true ] || exit 2
                printf '%s\n' "$network" > "$state/nic-network-$instance"
                printf '%s\n' "$hwaddr" > "$state/nic-hwaddr-$instance"
                ;;
              workspace:disk)
                source_path=''
                for arg in "$@"; do case "$arg" in source=*) source_path="${arg#source=}" ;; esac; done
                [ -n "$source_path" ] || exit 2
                printf '%s\n' "$source_path" > "$state/workspace-$instance"
                ;;
              *) exit 2 ;;
            esac
            ;;
          set)
            instance="${3:-}"; device="${4:-}"; assignment="${5:-}"
            key="${assignment%%=*}"; value="${assignment#*=}"
            [ -n "$instance" ] && [ -n "$device" ] && [ -n "$key" ] && [ "$assignment" != "$key" ] || exit 2
            printf '%s\n' "$value" > "$(config_file "$instance" "$device.$key")"
            ;;
          get)
            instance="${3:-}"; device="${4:-}"; key="${5:-}"
            if [ "$device:$key" = eth0:network ]; then cat "$state/nic-network-$instance"; exit 0; fi
            if [ "$device:$key" = eth0:hwaddr ]; then cat "$state/nic-hwaddr-$instance"; exit 0; fi
            file="$(config_file "$instance" "$device.$key")"
            [ -f "$file" ] && cat "$file"
            ;;
          *) exit 2 ;;
        esac
        ;;
      *) exit 2 ;;
    esac
    ;;
  start)
    instance="${1:-}"; echo RUNNING > "$state/instance-$instance"
    ;;
  list)
    case " $* " in *' --format json '*) printf '%s\n' '[]'; exit 0 ;; esac
    instance="${1:-}"; [ -f "$state/instance-$instance" ] || exit 0
    column=''; previous=''
    for arg in "$@"; do [ "$previous" = -c ] && column="$arg"; previous="$arg"; done
    case "$column" in n) printf '%s\n' "$instance" ;; s|*) cat "$state/instance-$instance" ;; esac
    ;;
  delete)
    instance="${1:-}"
    rm -f "$state/instance-$instance" "$state/workspace-$instance" "$state/nic-network-$instance" "$state/nic-hwaddr-$instance" "$state"/config-"$instance"-* 2>/dev/null || true
    ;;
  exec)
    instance="${1:-}"; shift
    while [ "$#" -gt 0 ] && [ "$1" != -- ]; do shift; done
    [ "$#" -gt 0 ] && shift
    [ "$#" -gt 0 ] || exit 2
    workspace="$(cat "$state/workspace-$instance")"
    executable="$1"; shift
    case "$executable" in
      sh)
        [ "${1:-}" = -c ] || exit 2
        translated="$(printf '%s' "${2:-}" | sed "s#/workspace#$workspace#g")"
        sh -c "$translated"
        ;;
      cat)
        target="$(printf '%s' "${1:-}" | sed "s#^/workspace#$workspace#")"; cat "$target"
        ;;
      test)
        if [ "${1:-}" = -w ] && [ "${2:-}" = /workspace ]; then test -w "$workspace"; else test "$@"; fi
        ;;
      *) "$executable" "$@" ;;
    esac
    ;;
  *) exit 2 ;;
esac
SH
chmod +x "$bin/incus"

go build -o "$haco" ./cmd/haco
go build -o "$controller" ./cmd/haco-controller
haco_start_test_controller "$controller" "$root/control.sock" "$root/controller.out" "$root/controller.err"

# Base catalog: logical names resolve to immutable revisions.
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
r=json.loads(sys.argv[1]); env=r['environment']
assert env['base']['name'] == 'my-dev', r
assert env['base']['revision'] == 'sha256:' + ('b' * 64), r
assert env['resources']['cpu']['mode'] == 'unlimited', r
PY
grep -Fq 'image info images:custom-moving --format json' "$HACO_FAKE_INCUS_LOG"
storage_line="$(grep -F 'storage create haco-local-default btrfs ' "$HACO_FAKE_INCUS_LOG" | head -1)"
[[ "$storage_line" == *'size=128GiB'* ]]
[[ "$storage_line" == *'btrfs.mount_options=compress=zstd:3,noatime,nodiscard'* ]]
[[ "$storage_line" != *'source='* ]]
grep -Fq 'init images:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb haco-base-demo' "$HACO_FAKE_INCUS_LOG"
grep -Fq -- '--no-profiles --storage haco-local-default' "$HACO_FAKE_INCUS_LOG"
bridge_line="$(grep -F 'network create hbr' "$HACO_FAKE_INCUS_LOG" | head -1)"
[ -n "$bridge_line" ]
[[ "$bridge_line" == *'ipv4.nat=false'* ]]
[[ "$bridge_line" == *'ipv4.firewall=true'* ]]
[[ "$bridge_line" == *'ipv4.dhcp=true'* ]]
[[ "$bridge_line" == *'user.hacocoon.owner=environment-network-v1'* ]]
nic_line="$(grep -F 'config device add haco-base-demo eth0 nic ' "$HACO_FAKE_INCUS_LOG" | tail -1)"
[[ "$nic_line" == *'network=hbr'* ]]
[[ "$nic_line" == *'hwaddr=02:'* ]]
[[ "$nic_line" == *'security.port_isolation=true'* ]]
[[ "$nic_line" != *'nictype=routed'* ]]
[[ "$nic_line" != *'security.ipv4_filtering'* ]]
[[ "$nic_line" != *'security.ipv6_filtering'* ]]
[[ "$nic_line" != *'security.mac_filtering'* ]]
grep -Fq 'ether saddr != 02:' "$HACO_FAKE_NFT_LOG"
grep -Fq 'ip saddr 0.0.0.0 udp sport 68 udp dport 67 accept' "$HACO_FAKE_NFT_LOG"
grep -Fq 'ip saddr != 10.240.0.0/24 drop' "$HACO_FAKE_NFT_LOG"
if grep -Fq -- '--profile haco-sandbox' "$HACO_FAKE_INCUS_LOG"; then
  echo 'managed local orchestration unexpectedly inherited the sandbox profile' >&2
  exit 1
fi
"$haco" delete base-demo

# Resource budgets are applied and verified before start.
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
[[ "$(cat "$workspace/result.txt")" == from-run ]]
run_name="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["environment"])' "$json")"
grep -Fq 'image info images:ubuntu/26.04 --format json' "$HACO_FAKE_INCUS_LOG"
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
grep -Fq run-error "$root/run.err"
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

echo 'PASS: Hacocoon orchestration, Base, resource, Incus-owned storage, and isolated-bridge E2E'

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

cat > "$bin/incus" <<'SH'
#!/bin/sh
set -u
state="$HACO_FAKE_INCUS_STATE"
printf '%s\n' "$*" >> "$HACO_FAKE_INCUS_LOG"
command_name="${1:-}"
[ "$#" -gt 0 ] && shift
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
  profile)
    if [ "${1:-}" = 'show' ] && [ "${2:-}" = 'default' ]; then
      printf '%s\n' '{"devices":{"root":{"type":"disk","path":"/","pool":"default"}}}'
      exit 0
    fi
    exit 2
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
    if [ "${1:-}" = 'device' ] && [ "${2:-}" = 'add' ]; then
      instance="${3:-}"
      shift 4
      source_path=''
      for arg in "$@"; do
        case "$arg" in source=*) source_path="${arg#source=}" ;; esac
      done
      [ -n "$source_path" ] || exit 2
      printf '%s\n' "$source_path" > "$state/workspace-$instance"
      exit 0
    fi
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
    rm -f "$state/instance-$instance" "$state/workspace-$instance"
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
"$haco" image list > "$root/bases.txt"
grep -Fxq 'haco/ubuntu-24.04' "$root/bases.txt"
grep -Fxq 'haco/ubuntu-26.04' "$root/bases.txt"
grep -Fxq 'my-dev' "$root/bases.txt"
base_info="$($haco image inspect my-dev --json)"
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
PY
grep -Fq 'image info images:custom-moving --format json' "$HACO_FAKE_INCUS_LOG"
grep -Fq 'init images:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb haco-base-demo' "$HACO_FAKE_INCUS_LOG"
"$haco" delete base-demo

json="$($haco run --workspace "$workspace" --json -- sh -c "printf 'agent-ok\\n'; printf 'from-run\\n' > /workspace/result.txt")"
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

echo 'PASS: Hacocoon v0.6/v0.11 orchestration and Base E2E'

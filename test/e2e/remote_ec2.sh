#!/usr/bin/env bash
set -euo pipefail
for command in go tar python3 mktemp grep; do command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 1; }; done
root="$(mktemp -d)"; trap 'rm -rf "$root"' EXIT
bin="$root/bin"; workspace="$root/workspace"; mkdir -p "$bin" "$workspace"
export PATH="$bin:$PATH"
export HACO_ROOT="$root/haco-root"
export HACO_FAKE_AWS_LOG="$root/aws.log"
export HACO_FAKE_AWS_COUNTER="$root/counter"
export HACO_EC2_REGION="ap-northeast-1"
export HACO_EC2_AMI="ami-0123456789abcdef0"
export HACO_EC2_INSTANCE_TYPE="t3.large"
export HACO_EC2_SUBNET_ID="subnet-0123456789abcdef0"
export HACO_EC2_SECURITY_GROUP_IDS="sg-0123456789abcdef0"
export HACO_EC2_INSTANCE_PROFILE="hacocoon-remote"
export HACO_EC2_WORKSPACE_BUCKET="hacocoon-workspaces-example"
export HACO_EC2_WORKSPACE_PREFIX="e2e"
export HACO_RUNTIME_PROVIDER="runtime.ec2"
cat > "$bin/aws" <<'SH'
#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$HACO_FAKE_AWS_LOG"
args="$*"
case "$args" in
  *" sts get-caller-identity "*) echo '123456789012' ;;
  *" ssm send-command "*)
    n=0; [ -f "$HACO_FAKE_AWS_COUNTER" ] && n="$(cat "$HACO_FAKE_AWS_COUNTER")"
    n=$((n+1)); printf '%s' "$n" > "$HACO_FAKE_AWS_COUNTER"
    case "$n" in
      1) echo '11111111-1111-1111-1111-111111111111' ;;
      *) echo '22222222-2222-2222-2222-222222222222' ;;
    esac
    ;;
  *" ec2 run-instances "*) echo 'i-0123456789abcdef0' ;;
  *" ssm describe-instance-information "*) echo 'Online' ;;
  *" s3 cp "*)
    set -- $args; prev=''
    for arg in "$@"; do
      if [ "$prev" = cp ] && [ "${arg#s3://}" = "$arg" ]; then [ -f "$arg" ] || exit 91; fi
      prev="$arg"
    done
    ;;
  *" ssm get-command-invocation "*)
    case "$args" in
      *22222222-2222-2222-2222-222222222222*) printf '%s\n' '{"Status":"Failed","ResponseCode":17,"StandardOutputContent":"remote-out","StandardErrorContent":"remote-err"}' ;;
      *) echo '{"Status":"Success","ResponseCode":0,"StandardOutputContent":"","StandardErrorContent":""}' ;;
    esac
    ;;
  *" ec2 describe-instances "*) echo 'running' ;;
  *" ssm start-session "*) exit 0 ;;
esac
SH
chmod +x "$bin/aws"
printf 'from-host\n' > "$workspace/host.txt"
haco="$root/haco"; go build -o "$haco" ./cmd/haco
set +e
"$haco" create --read-only --workspace "$workspace" disabled >"$root/disabled.out" 2>"$root/disabled.err"
disabled_code=$?
set -e
[[ "$disabled_code" != 0 ]]
grep -Fqi 'experimental EC2 is disabled' "$root/disabled.err"
[[ ! -e "$HACO_FAKE_AWS_LOG" ]] || [[ ! -s "$HACO_FAKE_AWS_LOG" ]]
export HACO_EXPERIMENTAL_EC2=1
"$haco" create --read-only --workspace "$workspace" remote > "$root/create.out"
grep -Fq $'remote\t' "$root/create.out"
"$haco" status remote --json > "$root/status.json"
python3 - "$root/status.json" <<'PY'
import json,sys
x=json.load(open(sys.argv[1])); assert x['environment']['runtime_ref'].startswith('haco-runtime-v1:runtime.ec2:'),x; assert x['state']=='running',x
PY
"$haco" shell remote
set +e
"$haco" exec remote -- sh -c 'printf ignored' >"$root/exec.out" 2>"$root/exec.err"
code=$?
set -e
[[ "$code" == 17 ]]
[[ "$(cat "$root/exec.out")" == 'remote-out' ]]
grep -Fq 'remote-err' "$root/exec.err"
set +e
"$haco" forward remote --host-port 18080 --target-port 8080 >/dev/null 2>"$root/forward.err"
forward_code=$?
set -e
[[ "$forward_code" != 0 ]]
grep -Fqi 'unsupported' "$root/forward.err"
"$haco" delete remote
if "$haco" status remote --json >/dev/null 2>&1; then echo 'deleted environment still visible' >&2; exit 1; fi
for want in 'sts get-caller-identity --query Account --output text' 'ec2 run-instances' 'HttpTokens=required,HttpEndpoint=enabled' 'ssm describe-instance-information' 'ssm send-command' 'ssm start-session' 'ec2 terminate-instances' 's3 rm'; do grep -Fq "$want" "$HACO_FAKE_AWS_LOG" || { echo "missing $want" >&2; cat "$HACO_FAKE_AWS_LOG" >&2; exit 1; }; done
if grep -Eqi 'AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN' "$HACO_FAKE_AWS_LOG"; then echo 'credential material leaked into aws argv' >&2; exit 1; fi
echo 'PASS: Hacocoon v0.7 remote EC2 process E2E'

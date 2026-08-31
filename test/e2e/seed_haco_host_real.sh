#!/usr/bin/env bash
set -euo pipefail

if [[ "${HACO_E2E_HACO_HOST_SEED:-}" != "1" ]]; then
  echo "SKIP: set HACO_E2E_HACO_HOST_SEED=1 on a supported Incus host"
  exit 0
fi

for command in go incus jq mktemp grep awk sha256sum; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

root="$(mktemp -d)"
haco="$root/haco"
haco_host_binary="$root/haco-host"
controller="$root/haco-controller"
controller_log="$root/controller.log"
control_socket="$root/control.sock"
staged_nerdctl="$root/nerdctl"
workspace_a="$root/workspace-a"
workspace_b="$root/workspace-b"
env_a="seed-host-a-$$"
env_b="seed-host-b-$$"
ref_a="haco-$env_a"
ref_b="haco-$env_b"
base="haco/ubuntu-26.04"
image_ref="docker.io/library/busybox:1.36"
credential_sentinel="HACO_HOST_CREDENTIAL_ONLY_$$"
export HACO_ROOT="$root/haco-root"
export HACO_CONTROL_SOCKET="$control_socket"
export HACO_PLUGIN_OCI=nerdctl
created_a=0
created_b=0
trusted_host_created=0
controller_pid=""

cleanup() {
  set +e
  if [[ "$created_a" == "1" ]]; then
    "$haco" delete "$env_a" >/dev/null 2>&1 || incus delete "$ref_a" --project hacocoon --force >/dev/null 2>&1 || true
  fi
  if [[ "$created_b" == "1" ]]; then
    "$haco" delete "$env_b" >/dev/null 2>&1 || incus delete "$ref_b" --project hacocoon --force >/dev/null 2>&1 || true
  fi
  if [[ "$trusted_host_created" == "1" ]]; then
    marker="$(incus config get haco-host user.hacocoon.role --project hacocoon 2>/dev/null || true)"
    if [[ "$marker" == "trusted-host" ]]; then
      incus delete haco-host --project hacocoon --force >/dev/null 2>&1 || true
    fi
  fi
  if [[ -n "$controller_pid" ]]; then
    kill "$controller_pid" >/dev/null 2>&1 || true
    wait "$controller_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

mkdir -p "$workspace_a" "$workspace_b"
printf 'seed-a\n' >"$workspace_a/host.txt"
printf 'seed-b\n' >"$workspace_b/host.txt"

go build -o "$haco" ./cmd/haco
go build -o "$haco_host_binary" ./cmd/haco-host
go build -o "$controller" ./cmd/haco-controller

"$controller" >"$controller_log" 2>&1 &
controller_pid=$!
for _ in $(seq 1 100); do
  [[ -S "$control_socket" ]] && break
  if ! kill -0 "$controller_pid" >/dev/null 2>&1; then
    cat "$controller_log" >&2 || true
    echo "haco-controller exited before creating its Unix socket" >&2
    exit 1
  fi
  sleep 0.05
done
[[ -S "$control_socket" ]] || {
  cat "$controller_log" >&2 || true
  echo "haco-controller did not create $control_socket" >&2
  exit 1
}

printf '%s\n' '=== Physical Host platform boundary ==='
uname -a
incus version
incus storage list
printf 'physical-containerd=%s\n' "$(systemctl is-active containerd.service 2>/dev/null || true)"
printf 'physical-nerdctl=%s\n' "$(command -v nerdctl 2>/dev/null || printf absent)"

printf '%s\n' '=== provision trusted haco-host OCI store ==='
"$haco" host ensure --oci
trusted_host_created=1
[[ "$(incus config get haco-host user.hacocoon.role --project hacocoon)" == "trusted-host" ]] || {
  echo "haco-host ownership marker mismatch" >&2
  exit 1
}
incus exec haco-host --project hacocoon -- systemctl is-active --quiet containerd.service
incus exec haco-host --project hacocoon -- nerdctl --version
incus exec haco-host --project hacocoon -- test ! -S /var/lib/incus/unix.socket
incus exec haco-host --project hacocoon -- test ! -S /var/lib/incus/unix.socket.user

# Place a reusable-credential-shaped sentinel only in the trusted Host. It must
# never appear in the published Seed or an untrusted Environment.
incus exec haco-host --project hacocoon -- install -d -m 0700 /root/.docker
printf '{"auths":{},"hacocoon_sentinel":"%s"}\n' "$credential_sentinel" | \
  incus exec haco-host --project hacocoon -- sh -c 'umask 077; cat > /root/.docker/config.json'

# The current tooling-Base builder injects the verified nerdctl client as a
# file. Stage that exact haco-host binary on the Physical Host without executing
# it; all OCI store operations remain routed through haco-host.
incus file pull haco-host/usr/local/bin/nerdctl "$staged_nerdctl" --project hacocoon
chmod 0700 "$staged_nerdctl"
export HACO_NERDCTL_BINARY="$staged_nerdctl"

printf '%s\n' '=== immutable parent Base ==='
parent_json="$($haco base inspect "$base" --json)"
printf '%s\n' "$parent_json"
parent_revision="$(jq -r '.revision' <<<"$parent_json")"
[[ "$parent_revision" =~ ^sha256:[0-9a-f]{64}$ ]] || {
  echo "parent Base was not resolved to an immutable sha256 revision" >&2
  exit 1
}

printf '%s\n' '=== OCI acquisition occurs inside haco-host ==='
incus exec haco-host --project hacocoon -- nerdctl pull "$image_ref" >/dev/null
digest="$(incus exec haco-host --project hacocoon -- nerdctl images --format '{{.Digest}}' "$image_ref" | awk 'NF { print $1; exit }')"
[[ "$digest" =~ ^sha256:[0-9a-f]{64}$ ]] || {
  echo "failed to resolve immutable OCI digest inside haco-host" >&2
  incus exec haco-host --project hacocoon -- nerdctl images >&2 || true
  exit 1
}
identity="$image_ref@$digest"
printf 'identity=%s\n' "$identity"

printf '%s\n' '=== pin + build offline Seed ==='
"$haco" plugin oci seed pin "$identity" --base "$base"
"$haco" plugin oci seed pins --base "$base" | grep -Fq "$identity"
build_json="$($haco plugin oci seed build --base "$base" --json)"
printf '%s\n' "$build_json"
seed_revision="$(jq -r '.seed_revision' <<<"$build_json")"
[[ "$seed_revision" =~ ^sha256:[0-9a-f]{64}$ ]] || {
  echo "Seed build did not publish an immutable revision" >&2
  exit 1
}
current_json="$($haco plugin oci seed current --base "$base" --json)"
[[ "$(jq -r '.seed_revision' <<<"$current_json")" == "$seed_revision" ]] || {
  echo "current Seed pointer mismatch" >&2
  exit 1
}
[[ "$(jq -r '.parent.revision' <<<"$current_json")" == "$parent_revision" ]] || {
  echo "current Seed parent revision drifted" >&2
  exit 1
}

if incus list --project hacocoon --format csv -c n | grep -E '^haco-(seed|tooling)-build-' >/dev/null; then
  echo "temporary Seed/Tooling builder remained after publication" >&2
  incus list --project hacocoon >&2 || true
  exit 1
fi

printf '%s\n' '=== create two Seed-derived Environments ==='
"$haco" create --base "$base" --workspace "$workspace_a" "$env_a"
created_a=1
"$haco" create --base "$base" --workspace "$workspace_b" "$env_b"
created_b=1

for env in "$env_a" "$env_b"; do
  "$haco" exec "$env" -- nerdctl image inspect "$identity" >/dev/null
  result="$($haco exec "$env" -- nerdctl run --rm --net none "$identity" sh -c 'printf seed-runtime-ok')"
  [[ "$result" == "seed-runtime-ok" ]] || {
    echo "Seed OCI runtime failed in $env: $result" >&2
    exit 1
  }
  "$haco" exec "$env" -- test ! -e /root/.docker/config.json
  if "$haco" exec "$env" -- sh -c "grep -R -F '$credential_sentinel' /root /etc /var/lib/hacocoon 2>/dev/null"; then
    echo "haco-host credential sentinel escaped into $env" >&2
    exit 1
  fi
done

printf '%s\n' '=== writable containerd state is independent ==='
"$haco" exec "$env_a" -- sh -c "printf 'only-a\\n' > /var/lib/containerd/hacocoon-state-a"
"$haco" exec "$env_b" -- test ! -e /var/lib/containerd/hacocoon-state-a
"$haco" exec "$env_b" -- sh -c "printf 'only-b\\n' > /var/lib/containerd/hacocoon-state-b"
"$haco" exec "$env_a" -- test ! -e /var/lib/containerd/hacocoon-state-b

printf '%s\n' '=== Docker compatibility is instance-local and socket-activated ==='
for env in "$env_a" "$env_b"; do
  before="$($haco exec "$env" -- systemctl is-active hacocoon-docker.service 2>/dev/null || true)"
  [[ "$before" != "active" ]] || {
    echo "Docker compatibility daemon was unexpectedly active in $env" >&2
    exit 1
  }
  docker_server="$($haco exec "$env" -- docker info --format '{{.ServerVersion}}')"
  [[ -n "$docker_server" ]] || {
    echo "Docker Engine API did not return a server version in $env" >&2
    exit 1
  }
  "$haco" exec "$env" -- systemctl is-active --quiet hacocoon-docker.service
  "$haco" exec "$env" -- test -S /run/docker.sock
done

printf '%s\n' '=== untrusted Environment boundary ==='
for ref in "$ref_a" "$ref_b"; do
  config="$(incus config show "$ref" --expanded --project hacocoon)"
  for forbidden in \
    '/var/lib/incus/unix.socket' \
    '/var/lib/incus/unix.socket.user' \
    '/var/lib/hacocoon-control.sock' \
    '/run/docker.sock'; do
    if grep -Fq "source: $forbidden" <<<"$config"; then
      echo "forbidden trusted/Host socket $forbidden exposed to $ref" >&2
      exit 1
    fi
  done
  if incus config device list "$ref" --project hacocoon | grep -Fxq haco-control; then
    echo "ordinary Environment unexpectedly received haco-host control proxy" >&2
    exit 1
  fi
done

printf '%s\n' '=== Seed maintenance ==='
"$haco" plugin oci seed gc --json | jq .
"$haco" plugin oci seed recover --json | jq .
"$haco" plugin oci seed unpin "$identity" --base "$base"
if "$haco" plugin oci seed pins --base "$base" | grep -Fq "$identity"; then
  echo "Seed image remained pinned after unpin" >&2
  exit 1
fi
"$haco" plugin oci image delete "$identity" --all-environments --json | jq .
reenable_json="$($haco plugin oci image reenable "$identity" --json)"
[[ "$(jq -r '.removed' <<<"$reenable_json")" == "true" ]] || {
  echo "exact image re-enable did not remove the deletion tombstone" >&2
  exit 1
}
final_current="$($haco plugin oci seed current --base "$base" --json)"
[[ "$(jq -r '.seed_revision' <<<"$final_current")" == "$seed_revision" ]] || {
  echo "maintenance unexpectedly changed current Seed revision" >&2
  exit 1
}

"$haco" delete "$env_a"
created_a=0
"$haco" delete "$env_b"
created_b=0

printf '%s\n' 'PASS: real haco-host/Incus/containerd/nerdctl Seed acceptance'

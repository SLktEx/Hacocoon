#!/usr/bin/env bash
set -euo pipefail

if [[ "${HACO_E2E_INCUS_SEED:-}" != "1" ]]; then
  echo "SKIP: set HACO_E2E_INCUS_SEED=1 on a supported Incus host"
  exit 0
fi

for command in go incus containerd nerdctl jq mktemp grep awk; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

root="$(mktemp -d)"
haco="$root/haco"
workspace_a="$root/workspace-a"
workspace_b="$root/workspace-b"
env_a="seed-real-a-$$"
env_b="seed-real-b-$$"
ref_a="haco-$env_a"
ref_b="haco-$env_b"
base="haco/ubuntu-26.04"
image_ref="docker.io/library/busybox:1.36"
export HACO_ROOT="$root/haco-root"
export HACO_PLUGIN_OCI=nerdctl
export HACO_NERDCTL_BINARY="${HACO_NERDCTL_BINARY:-$(command -v nerdctl)}"
created_a=0
created_b=0

cleanup() {
  set +e
  if [[ "$created_a" == "1" ]]; then
    "$haco" delete "$env_a" >/dev/null 2>&1 || incus delete "$ref_a" --project hacocoon --force >/dev/null 2>&1 || true
  fi
  if [[ "$created_b" == "1" ]]; then
    "$haco" delete "$env_b" >/dev/null 2>&1 || incus delete "$ref_b" --project hacocoon --force >/dev/null 2>&1 || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

mkdir -p "$workspace_a" "$workspace_b"
printf 'seed-a\n' >"$workspace_a/host.txt"
printf 'seed-b\n' >"$workspace_b/host.txt"

go build -o "$haco" ./cmd/haco

printf '%s\n' '=== real-host versions ==='
uname -a
incus version
containerd --version
nerdctl --version
incus storage show default | sed -n '1,80p'
printf '%s\n' '=== immutable parent Base ==='
parent_json="$($haco base inspect "$base" --json)"
printf '%s\n' "$parent_json"
parent_revision="$(jq -r '.revision' <<<"$parent_json")"
[[ "$parent_revision" =~ ^sha256:[0-9a-f]{64}$ ]] || {
  echo "parent Base was not resolved to an immutable sha256 revision" >&2
  exit 1
}

printf '%s\n' '=== trusted Host OCI identity ==='
nerdctl pull "$image_ref" >/dev/null
digest="$(nerdctl images --format '{{.Digest}}' "$image_ref" | awk 'NF { print $1; exit }')"
[[ "$digest" =~ ^sha256:[0-9a-f]{64}$ ]] || {
  echo "failed to resolve busybox immutable digest from nerdctl" >&2
  nerdctl images --format '{{.Repository}}\t{{.Tag}}\t{{.Digest}}' >&2 || true
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
printf '%s\n' "$current_json"
[[ "$(jq -r '.seed_revision' <<<"$current_json")" == "$seed_revision" ]] || {
  echo "current Seed pointer does not match the successfully published Seed" >&2
  exit 1
}
[[ "$(jq -r '.parent.revision' <<<"$current_json")" == "$parent_revision" ]] || {
  echo "current Seed parent revision drifted from the immutable parent" >&2
  exit 1
}

printf '%s\n' '=== published Seed builder boundary ==='
if incus list --project hacocoon --format csv -c n | grep -E '^haco-(seed|tooling)-build-' >/dev/null; then
  echo "temporary Seed/Tooling builder remained after successful publication" >&2
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
    echo "Docker compatibility daemon was unexpectedly already active in $env" >&2
    exit 1
  }
  docker_server="$($haco exec "$env" -- docker info --format '{{.ServerVersion}}')"
  [[ -n "$docker_server" ]] || {
    echo "Docker Engine API did not return a server version in $env" >&2
    exit 1
  }
  "$haco" exec "$env" -- systemctl is-active --quiet hacocoon-docker.service
  "$haco" exec "$env" -- test -S /run/docker.sock
  printf '%s docker-server=%s\n' "$env" "$docker_server"
done

for ref in "$ref_a" "$ref_b"; do
  config="$(incus config show "$ref" --expanded --project hacocoon)"
  if grep -Eq 'source: /(var/)?run/docker\.sock' <<<"$config"; then
    echo "Host Docker socket is exposed to $ref" >&2
    exit 1
  fi
done

printf '%s\n' '=== real-host maintenance operations ==='
"$haco" plugin oci seed gc --json | jq .
"$haco" plugin oci seed recover --json | jq .
"$haco" plugin oci seed unpin "$identity" --base "$base"
if "$haco" plugin oci seed pins --base "$base" | grep -Fq "$identity"; then
  echo "Seed image remained pinned after unpin" >&2
  exit 1
fi

"$haco" plugin oci image delete "$identity" --all-environments --json | jq .
reenable_json="$($haco plugin oci image reenable "$identity" --json)"
printf '%s\n' "$reenable_json"
[[ "$(jq -r '.removed' <<<"$reenable_json")" == "true" ]] || {
  echo "exact image re-enable did not remove the deletion tombstone" >&2
  exit 1
}

printf '%s\n' '=== final current Seed remains immutable ==='
final_current="$($haco plugin oci seed current --base "$base" --json)"
[[ "$(jq -r '.seed_revision' <<<"$final_current")" == "$seed_revision" ]] || {
  echo "maintenance unexpectedly changed the current Seed revision" >&2
  exit 1
}

"$haco" delete "$env_a"
created_a=0
"$haco" delete "$env_b"
created_b=0

printf '%s\n' 'PASS: real Incus/containerd/nerdctl Seed acceptance'

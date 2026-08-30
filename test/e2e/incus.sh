#!/usr/bin/env bash
set -euo pipefail

if [[ "${HACO_E2E_INCUS:-}" != "1" ]]; then
  echo "SKIP: set HACO_E2E_INCUS=1 on a supported Incus host"
  exit 0
fi

for command in go incus mktemp grep; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

incus version >/dev/null

root="$(mktemp -d)"
workspace="$root/workspace"
haco="$root/haco"
environment="e2e-$$"
runtime_ref="haco-$environment"
export HACO_ROOT="$root/haco-root"
created=0

cleanup() {
  set +e
  if [[ "$created" == "1" ]]; then
    "$haco" delete "$environment" >/dev/null 2>&1 || \
      incus delete "$runtime_ref" --project hacocoon --force >/dev/null 2>&1 || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

mkdir -p "$workspace"
printf 'from-host\n' > "$workspace/host.txt"

go build -o "$haco" ./cmd/haco

"$haco" create --workspace "$workspace" "$environment" >/dev/null
created=1

status_output="$("$haco" status "$environment")"
grep -Fq "state: running" <<<"$status_output" || {
  echo "Hacocoon status did not report running state" >&2
  printf '%s\n' "$status_output" >&2
  exit 1
}
grep -Fq "runtime: $runtime_ref" <<<"$status_output" || {
  echo "Hacocoon status did not report expected Incus runtime ref" >&2
  printf '%s\n' "$status_output" >&2
  exit 1
}

pid1="$("$haco" exec "$environment" -- cat /proc/1/comm)"
[[ "$pid1" == "systemd" ]] || {
  echo "expected systemd as PID 1, got $pid1" >&2
  exit 1
}
"$haco" exec "$environment" -- systemctl is-active --quiet systemd-journald.service

read_back="$("$haco" exec "$environment" -- cat /workspace/host.txt)"
[[ "$read_back" == "from-host" ]] || {
  echo "workspace host->environment read mismatch: $read_back" >&2
  exit 1
}

"$haco" exec "$environment" -- sh -c "printf 'from-environment\\n' > /workspace/environment.txt"
[[ "$(cat "$workspace/environment.txt")" == "from-environment" ]] || {
  echo "workspace environment->host write mismatch" >&2
  exit 1
}

stdout_file="$root/stdout"
stderr_file="$root/stderr"
set +e
"$haco" exec "$environment" -- sh -c "printf 'stdout-ok'; printf 'stderr-ok' >&2; exit 17" >"$stdout_file" 2>"$stderr_file"
exit_code=$?
set -e
[[ "$exit_code" == "17" ]] || {
  echo "expected remote exit 17, got $exit_code" >&2
  exit 1
}
[[ "$(cat "$stdout_file")" == "stdout-ok" ]] || {
  echo "stdout propagation mismatch" >&2
  exit 1
}
grep -q "stderr-ok" "$stderr_file" || {
  echo "stderr propagation mismatch" >&2
  exit 1
}

printf 'exit\n' | "$haco" shell "$environment" >/dev/null

status_output="$("$haco" status "$environment")"
grep -Fq "state: running" <<<"$status_output" || {
  echo "Hacocoon status changed unexpectedly after exec/shell" >&2
  printf '%s\n' "$status_output" >&2
  exit 1
}

config_file="$root/incus-config"
incus config show "$runtime_ref" --expanded --project hacocoon >"$config_file"
grep -Fq "$workspace" "$config_file" || {
  echo "requested workspace is not mounted in Incus config" >&2
  exit 1
}
for forbidden in \
  "${HOME:-}/.ssh" \
  "${HOME:-}/.aws" \
  "${HOME:-}/.config/gh" \
  "/var/lib/incus/unix.socket" \
  "/var/lib/incus/unix.socket.user"; do
  [[ "$forbidden" == "/.ssh" || "$forbidden" == "/.aws" || "$forbidden" == "/.config/gh" ]] && continue
  if grep -Fq "$forbidden" "$config_file"; then
    echo "unexpected credential/authority exposure in Incus config: $forbidden" >&2
    exit 1
  fi
done

"$haco" delete "$environment"
created=0

if incus info "$runtime_ref" --project hacocoon >/dev/null 2>&1; then
  echo "environment still exists after haco delete" >&2
  exit 1
fi

if [[ -e "$HACO_ROOT/state/environments.json" ]] && grep -Fq "\"$environment\"" "$HACO_ROOT/state/environments.json"; then
  echo "environment metadata still exists after haco delete" >&2
  exit 1
fi

echo "PASS: Hacocoon Core CLI -> real Incus lifecycle E2E"

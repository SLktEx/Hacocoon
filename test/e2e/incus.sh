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
haco_host="$root/haco-host"
controller="$root/haco-controller"
controller_log="$root/controller.log"
control_socket="$root/control.sock"
environment="e2e-$$"
runtime_ref="haco-$environment"
trusted_host_ref="haco-host"
trusted_control_socket="/var/lib/hacocoon-control.sock"
export HACO_ROOT="$root/haco-root"
export HACO_CONTROL_SOCKET="$control_socket"
created=0
trusted_host_created=0
controller_pid=""

cleanup() {
  set +e
  if [[ "$created" == "1" ]]; then
    "$haco" delete "$environment" >/dev/null 2>&1 || \
      incus delete "$runtime_ref" --project hacocoon --force >/dev/null 2>&1 || true
  fi
  if [[ "$trusted_host_created" == "1" ]]; then
    marker="$(incus config get "$trusted_host_ref" user.hacocoon.role --project hacocoon 2>/dev/null || true)"
    if [[ "$marker" == "trusted-host" ]]; then
      incus delete "$trusted_host_ref" --project hacocoon --force >/dev/null 2>&1 || true
    fi
  fi
  if [[ -n "$controller_pid" ]]; then
    kill "$controller_pid" >/dev/null 2>&1 || true
    wait "$controller_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

mkdir -p "$workspace"
printf 'from-host\n' > "$workspace/host.txt"

go build -o "$haco" ./cmd/haco
go build -o "$haco_host" ./cmd/haco-host
go build -o "$controller" ./cmd/haco-controller

HACO_STORAGE_PRIVILEGE_MODE=direct "$controller" >"$controller_log" 2>&1 &
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

"$haco" host ensure
trusted_host_created=1
[[ "$(incus config get "$trusted_host_ref" user.hacocoon.role --project hacocoon)" == "trusted-host" ]] || {
  echo "trusted haco-host ownership marker mismatch" >&2
  exit 1
}
[[ "$(incus config get "$trusted_host_ref" environment.HACO_CONTROL_SOCKET --project hacocoon)" == "$trusted_control_socket" ]] || {
  echo "trusted haco-host control environment mismatch" >&2
  exit 1
}
[[ "$(incus list "$trusted_host_ref" --project hacocoon --format csv -c s)" == "RUNNING" ]] || {
  echo "trusted haco-host is not running after first ensure" >&2
  exit 1
}

for pair in \
  "type=proxy" \
  "bind=instance" \
  "listen=unix:$trusted_control_socket" \
  "connect=unix:$control_socket" \
  "mode=0600" \
  "uid=0" \
  "gid=0"; do
  key="${pair%%=*}"
  want="${pair#*=}"
  got="$(incus config device get "$trusted_host_ref" haco-control "$key" --project hacocoon)"
  [[ "$got" == "$want" ]] || {
    echo "trusted haco-host proxy $key mismatch: got '$got' want '$want'" >&2
    exit 1
  }
done

incus exec "$trusted_host_ref" --project hacocoon -- test -x /usr/local/bin/haco-host
first_doctor="$(incus exec "$trusted_host_ref" --project hacocoon -- /usr/local/bin/haco-host doctor)"
grep -Fq "Hacocoon logical Host client" <<<"$first_doctor" || {
  echo "haco-host doctor did not identify the logical Host client" >&2
  exit 1
}
grep -Fq "controller: $trusted_control_socket" <<<"$first_doctor" || {
  echo "haco-host doctor did not use the projected controller socket" >&2
  exit 1
}
grep -Eq '^protocol-version: [0-9]+$' <<<"$first_doctor" || {
  echo "haco-host doctor did not complete the controller protocol round trip" >&2
  exit 1
}

# Re-entry must be idempotent and a stopped trusted host must be restarted.
"$haco" host ensure
incus stop "$trusted_host_ref" --project hacocoon
[[ "$(incus list "$trusted_host_ref" --project hacocoon --format csv -c s)" == "STOPPED" ]] || {
  echo "trusted haco-host did not stop for restart acceptance" >&2
  exit 1
}
"$haco" host ensure
[[ "$(incus list "$trusted_host_ref" --project hacocoon --format csv -c s)" == "RUNNING" ]] || {
  echo "trusted haco-host was not restarted by ensure" >&2
  exit 1
}
incus exec "$trusted_host_ref" --project hacocoon -- /usr/local/bin/haco-host doctor >/dev/null

trusted_host_config="$root/trusted-host-config"
incus config show "$trusted_host_ref" --expanded --project hacocoon >"$trusted_host_config"
for forbidden in \
  "/var/lib/incus/unix.socket" \
  "/var/lib/incus/unix.socket.user"; do
  if grep -Fq "$forbidden" "$trusted_host_config"; then
    echo "trusted haco-host unexpectedly exposes Incus control socket: $forbidden" >&2
    exit 1
  fi
done

"$haco" create --workspace "$workspace" "$environment" >/dev/null
created=1

if incus config device list "$runtime_ref" --project hacocoon | grep -Fxq haco-control; then
  echo "ordinary Environment unexpectedly received the trusted controller proxy" >&2
  exit 1
fi
if [[ "$(incus config get "$runtime_ref" environment.HACO_CONTROL_SOCKET --project hacocoon 2>/dev/null || true)" == "$trusted_control_socket" ]]; then
  echo "ordinary Environment unexpectedly received the trusted controller endpoint" >&2
  exit 1
fi

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

echo "PASS: trusted haco-host -> Hacocoon UDS -> Physical Host controller + Incus E2E"

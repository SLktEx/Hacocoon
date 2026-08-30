#!/usr/bin/env bash
set -euo pipefail

if [[ "${HACO_E2E_INCUS:-}" != "1" ]]; then
  echo "SKIP: set HACO_E2E_INCUS=1 on a supported real Incus host"
  exit 0
fi

for command in go git incus mktemp ssh ssh-keygen grep awk sed; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

root="$(mktemp -d)"
repo="$root/repo"
worktrees="$root/worktrees"
worktree="$worktrees/task"
other_worktree="$worktrees/other"
client_home="$root/client-home"
identity="$client_home/.ssh/id_ed25519"
haco_vscode="$root/haco-vscode"
environment="vscode-e2e-$$"
runtime_ref="haco-$environment"
managed_config="$client_home/.ssh/hacocoon/$environment.conf"
open_one="$root/open-one.txt"
open_two="$root/open-two.txt"
created=0

export HACO_ROOT="$root/haco-root"

cleanup() {
  set +e
  HOME="$client_home" "$haco_vscode" delete --name "$environment" >/dev/null 2>&1 || true
  if incus info "$runtime_ref" --project hacocoon >/dev/null 2>&1; then
    incus delete "$runtime_ref" --project hacocoon --force >/dev/null 2>&1 || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

mkdir -p "$repo" "$worktrees" "$client_home/.ssh"
chmod 700 "$client_home/.ssh"

git -C "$repo" init -q
git -C "$repo" config user.name "Hacocoon E2E"
git -C "$repo" config user.email "e2e@invalid.example"
printf 'fixture\n' >"$repo/fixture.txt"
printf 'main-only\n' >"$repo/main-only.txt"
git -C "$repo" add fixture.txt main-only.txt
git -C "$repo" commit -qm 'initial fixture'

git -C "$repo" worktree add -q -b e2e/task "$worktree"
git -C "$repo" worktree add -q -b e2e/other "$other_worktree"
printf 'selected-worktree\n' >"$worktree/selected.txt"
printf 'other-worktree\n' >"$other_worktree/other.txt"

git -C "$worktree" rev-parse --git-dir | grep -Fq '/worktrees/' || fail "fixture is not a standard linked Git worktree"
[[ -f "$worktree/.git" ]] || fail "linked worktree .git file is missing"
[[ ! -e "$repo/selected.txt" ]] || fail "selected worktree fixture leaked into main checkout"
[[ ! -e "$other_worktree/selected.txt" ]] || fail "selected worktree fixture leaked into unrelated worktree"

go build -o "$haco_vscode" ./cmd/haco-vscode
ssh-keygen -q -t ed25519 -N '' -f "$identity"
chmod 600 "$identity"

HOME="$client_home" "$haco_vscode" open \
  --no-launch \
  --name "$environment" \
  --identity "$identity" \
  "$worktree" >"$open_one"
created=1

grep -Fq "environment: $environment" "$open_one" || fail "haco-vscode did not report the expected Environment"
grep -Fq "workspace: /workspace" "$open_one" || fail "haco-vscode did not report /workspace"
[[ -f "$managed_config" ]] || fail "managed SSH config was not created"
[[ "$(stat -c '%a' "$managed_config")" == "600" ]] || fail "managed SSH config must be mode 0600"
grep -Fq 'HostName 127.0.0.1' "$managed_config" || fail "managed SSH config is not loopback-only"
grep -Fq "IdentityFile $identity" "$managed_config" || fail "managed SSH config does not retain the client-side identity path"

alias_name="$(awk '$1 == "Host" {print $2; exit}' "$managed_config")"
[[ -n "$alias_name" ]] || fail "managed SSH alias is missing"

ssh_cmd=(ssh -F "$client_home/.ssh/config" -o BatchMode=yes -o ConnectTimeout=2 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$alias_name")
ssh_ready=0
for _ in $(seq 1 60); do
  if "${ssh_cmd[@]}" 'test -d /workspace' >/dev/null 2>&1; then
    ssh_ready=1
    break
  fi
  sleep 0.5
done
[[ "$ssh_ready" == "1" ]] || {
  echo 'managed SSH config:' >&2
  sed -E 's#(IdentityFile ).*#\1<redacted>#' "$managed_config" >&2 || true
  incus info "$runtime_ref" --project hacocoon >&2 || true
  fail "real SSH connection to the Environment did not become ready"
}

remote_workspace="$("${ssh_cmd[@]}" 'cd /workspace && pwd')"
[[ "$remote_workspace" == "/workspace" ]] || fail "remote workspace mismatch: $remote_workspace"
[[ "$("${ssh_cmd[@]}" 'cat /workspace/selected.txt')" == "selected-worktree" ]] || fail "SSH client did not reach the selected worktree"
if "${ssh_cmd[@]}" 'test -e /workspace/other.txt'; then
  fail "unrelated worktree content was exposed through /workspace"
fi

# A linked worktree is the real product input, not a copied checkout. Git must
# therefore remain usable from the Environment itself.
"${ssh_cmd[@]}" 'git -C /workspace status --short >/dev/null'
"${ssh_cmd[@]}" 'git -C /workspace rev-parse --is-inside-work-tree' | grep -Fxq true || fail "Git does not recognize /workspace as a worktree"

"${ssh_cmd[@]}" "printf 'from-environment\\n' > /workspace/from-environment.txt"
[[ "$(cat "$worktree/from-environment.txt")" == "from-environment" ]] || fail "Environment write was not reflected in the selected host worktree"
[[ ! -e "$repo/from-environment.txt" ]] || fail "Environment write leaked into main checkout"
[[ ! -e "$other_worktree/from-environment.txt" ]] || fail "Environment write leaked into unrelated worktree"

incus exec "$runtime_ref" --project hacocoon -- test ! -e /root/.ssh/id_ed25519 || fail "client private key leaked into the Environment"
incus exec "$runtime_ref" --project hacocoon -- grep -Fq "$(cat "$identity.pub")" /root/.ssh/authorized_keys || fail "expected public key was not installed for SSH access"

first_port="$(awk '$1 == "Port" {print $2; exit}' "$managed_config")"
[[ "$first_port" =~ ^[0-9]+$ ]] || fail "managed SSH port is invalid"

HOME="$client_home" "$haco_vscode" open \
  --no-launch \
  --name "$environment" \
  --identity "$identity" \
  "$worktree" >"$open_two"
second_port="$(awk '$1 == "Port" {print $2; exit}' "$managed_config")"
[[ "$second_port" == "$first_port" ]] || fail "reopen unexpectedly rotated a reusable SSH connection"
"${ssh_cmd[@]}" 'test -f /workspace/from-environment.txt' || fail "reconnected SSH client lost the selected workspace"

HOME="$client_home" "$haco_vscode" delete --name "$environment"
created=0
[[ ! -e "$managed_config" ]] || fail "adapter-owned SSH config remained after delete"
if incus info "$runtime_ref" --project hacocoon >/dev/null 2>&1; then
  fail "Environment remained after haco-vscode delete"
fi
[[ -d "$worktree" && -f "$worktree/selected.txt" ]] || fail "cleanup removed or damaged the selected Git worktree"
[[ -d "$repo/.git" ]] || fail "cleanup removed or damaged the repository"
[[ -f "$client_home/.ssh/config" ]] || fail "cleanup removed the client SSH config"

echo "VS Code user-journey E2E passed: linked worktree -> Environment -> real loopback SSH -> /workspace -> reconnect -> cleanup"

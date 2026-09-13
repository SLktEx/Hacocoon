#!/usr/bin/env bash
# Used only by the disposable-runner helpers after their runner/ownership checks.
# A failed list is unknown state, never an empty inventory or proof of absence.
ci_delete_project() {
  local project="$1" instances instance projects observed
  # A remote-qualified name must never redirect deletion to another daemon.
  [[ "$project" == hacocoon || "$project" =~ ^haco-e2e-[a-z0-9-]+$ ]] || return 1
  instances="$(incus list --project "$project" --format csv -c n)" || return 1
  while IFS= read -r instance; do
    [[ -n "$instance" ]] || continue
    if [[ ! "$instance" =~ ^haco-[a-zA-Z0-9-]+$ ]]; then
      echo 'CI cleanup refused unexpected instance ownership' >&2
      return 1
    fi
  done <<< "$instances"
  printf 'yes\n' | incus project delete "$project" --force || return 1
  projects="$(incus project list --format csv -c n)" || return 1
  [[ -n "$projects" ]] || return 1
  while IFS= read -r observed; do
    [[ "$observed" =~ ^[a-zA-Z0-9][a-zA-Z0-9_.-]*$ ]] || return 1
  done <<< "$projects"
  if grep -Fxq -- "$project" <<< "$projects"; then
    echo 'CI cleanup could not confirm project absence' >&2
    return 1
  fi
}

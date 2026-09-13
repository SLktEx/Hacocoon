#!/usr/bin/env bash
# Used only by the disposable-runner helpers after their runner/ownership checks.
# A failed list is unknown state, never an empty inventory or proof of absence.
ci_delete_project() {
  local project="$1" instances instance projects
  case "$project" in hacocoon|haco-e2e-*) ;; *) return 1 ;; esac
  instances="$(incus list --project "$project" --format csv -c n)" || return 1
  while IFS= read -r instance; do
    [[ -n "$instance" ]] || continue
    case "$instance" in
      haco-*) ;;
      *) echo 'CI cleanup refused unexpected instance ownership' >&2; return 1 ;;
    esac
  done <<< "$instances"
  printf 'yes\n' | incus project delete "$project" --force || return 1
  projects="$(incus project list --format csv -c n)" || return 1
  if grep -Fxq -- "$project" <<< "$projects"; then
    echo 'CI cleanup could not confirm project absence' >&2
    return 1
  fi
}

#!/usr/bin/env bash
# Used only by the disposable-runner helpers after their runner/ownership checks.
# A failed list is unknown state, never an empty inventory or proof of absence.
ci_project_names() {
  local projects
  # CSV is presentation output: Incus decorates the selected project's name
  # with a localized "(current)" suffix. JSON retains canonical names.
  projects="$(incus project list --format=json)" || return 1
  python3 -c '
import json, re, sys
try:
    raw = sys.stdin.read(1048577)
    if len(raw) > 1048576:
        raise ValueError()
    rows = json.loads(raw)
    if not isinstance(rows, list) or not 1 <= len(rows) <= 1000:
        raise ValueError()
    names = [row["name"] for row in rows]
    if any(not isinstance(n, str) or not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9_.-]*", n) for n in names):
        raise ValueError()
    if "default" not in names or len(set(names)) != len(names):
        raise ValueError()
    print("\n".join(names))
except (ValueError, TypeError, KeyError):
    print("CI cleanup project inventory is invalid", file=sys.stderr)
    sys.exit(1)
' <<< "$projects"
}

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
  projects="$(ci_project_names)" || return 1
  [[ -n "$projects" ]] || return 1
  while IFS= read -r observed; do
    [[ "$observed" =~ ^[a-zA-Z0-9][a-zA-Z0-9_.-]*$ ]] || return 1
  done <<< "$projects"
  if grep -Fxq -- "$project" <<< "$projects"; then
    echo 'CI cleanup could not confirm project absence' >&2
    return 1
  fi
}

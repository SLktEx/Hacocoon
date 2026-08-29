#!/usr/bin/env bash
set -euo pipefail

tag="${1:-}"
event_ref="${2:-${GITHUB_REF:-}}"
default_branch="${3:-${DEFAULT_BRANCH:-}}"
source_sha="${4:-${GITHUB_SHA:-}}"
remote="${5:-origin}"

if [[ -z "$tag" ]]; then
  echo "Refusing release: requested tag is unavailable" >&2
  exit 1
fi
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "Refusing release: invalid version tag: $tag" >&2
  exit 1
fi
if [[ -z "$default_branch" ]]; then
  echo "Refusing release: default branch is unavailable" >&2
  exit 1
fi
if [[ "$event_ref" != "refs/heads/$default_branch" ]]; then
  echo "Refusing release: trusted release dispatch must run from refs/heads/$default_branch, got ${event_ref:-<empty>}" >&2
  exit 1
fi
if [[ ! "$source_sha" =~ ^[0-9a-fA-F]{40}$ ]]; then
  echo "Refusing release: invalid source SHA: ${source_sha:-<empty>}" >&2
  exit 1
fi

head_commit="$(git rev-parse --verify 'HEAD^{commit}')"
if [[ "$head_commit" != "$source_sha" ]]; then
  echo "Refusing release: checked-out source $head_commit does not match dispatch source $source_sha" >&2
  exit 1
fi

remote_tracking_ref="refs/remotes/${remote}/${default_branch}"
if ! git fetch --no-tags --force "$remote" \
  "+refs/heads/${default_branch}:${remote_tracking_ref}"; then
  echo "Refusing release: unable to fetch ${remote}/${default_branch}" >&2
  exit 1
fi
if ! git merge-base --is-ancestor "$source_sha" "$remote_tracking_ref"; then
  echo "Refusing release: dispatch source $source_sha is outside trusted ${remote}/${default_branch} history" >&2
  exit 1
fi

printf 'trusted release request: OK (%s -> %s on %s)\n' "$tag" "$source_sha" "$event_ref"

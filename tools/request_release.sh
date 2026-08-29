#!/usr/bin/env bash
set -euo pipefail

tag="${1:-}"
repository="${HACO_RELEASE_REPOSITORY:-SLktEx/Hacocoon}"

if [[ -z "$tag" ]]; then
  echo "usage: $0 vX.Y.Z" >&2
  exit 2
fi
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid release tag: $tag" >&2
  exit 2
fi
if ! command -v gh >/dev/null 2>&1; then
  echo "GitHub CLI (gh) is required" >&2
  exit 1
fi
if ! gh auth status >/dev/null 2>&1; then
  echo "GitHub CLI authentication is required" >&2
  exit 1
fi

gh api --method POST "repos/$repository/dispatches" \
  --raw-field event_type=release \
  --raw-field "client_payload[tag]=$tag"

printf 'Requested trusted Hacocoon release %s from the repository default branch.\n' "$tag"

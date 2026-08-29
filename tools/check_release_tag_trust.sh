#!/usr/bin/env bash
set -euo pipefail

tag="${1:-${GITHUB_REF_NAME:-}}"
remote="${2:-origin}"
default_branch="${3:-${DEFAULT_BRANCH:-main}}"

if [[ -z "$tag" ]]; then
  echo "Refusing release: tag name is unavailable" >&2
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

# Resolve the tag object explicitly. The ^{commit} suffix safely peels both
# annotated and lightweight tags to the commit that would be released.
tag_ref="refs/tags/$tag"
if ! git show-ref --verify --quiet "$tag_ref"; then
  echo "Refusing release: tag does not exist locally: $tag" >&2
  exit 1
fi
if ! tag_commit="$(git rev-parse --verify "${tag_ref}^{commit}")"; then
  echo "Refusing release: tag does not resolve to a commit: $tag" >&2
  exit 1
fi

# Fetch only the authoritative default branch and do not trust a similarly
# named local branch from the tagged source tree.
remote_tracking_ref="refs/remotes/${remote}/${default_branch}"
if ! git fetch --no-tags --force "$remote" \
  "+refs/heads/${default_branch}:${remote_tracking_ref}"; then
  echo "Refusing release: unable to fetch ${remote}/${default_branch}" >&2
  exit 1
fi

if ! git merge-base --is-ancestor "$tag_commit" "$remote_tracking_ref"; then
  echo "Refusing release: tag $tag resolves to $tag_commit, which is outside trusted ${remote}/${default_branch} history" >&2
  exit 1
fi

printf 'release tag trust: OK (%s -> %s on %s/%s)\n' \
  "$tag" "$tag_commit" "$remote" "$default_branch"

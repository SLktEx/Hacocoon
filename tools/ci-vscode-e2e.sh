#!/usr/bin/env bash
set -euo pipefail

[[ "${GITHUB_ACTIONS:-}" == "true" ]] || {
  echo "ERROR: VS Code real-Incus E2E helper only runs inside GitHub Actions" >&2
  exit 1
}
[[ "${HACO_CI_RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || {
  echo "ERROR: VS Code real-Incus E2E helper requires a GitHub-hosted runner" >&2
  exit 1
}
[[ "$(uname -s)" == "Linux" ]] || {
  echo "ERROR: VS Code real-Incus E2E helper requires Linux" >&2
  exit 1
}

# Keep both privileged integration opt-in and Incus client authority behind the
# reviewed helper. tools/ci-incus.sh creates this TLS-only client under
# RUNNER_TEMP. The VS Code E2E deliberately changes HOME to isolate client-side
# SSH configuration, so INCUS_CONF must remain explicit instead of following
# that temporary HOME.
export HACO_E2E_INCUS=1
export INCUS_CONF="${HACO_CI_INCUS_CONF:-${RUNNER_TEMP:-/tmp}/haco-incus-client}"
[[ -r "$INCUS_CONF/config.yml" ]] || {
  echo "ERROR: reviewed Incus TLS client configuration is missing: $INCUS_CONF/config.yml" >&2
  exit 1
}

exec bash test/e2e/vscode.sh

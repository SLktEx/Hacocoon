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

# Keep the privileged integration opt-in out of workflow YAML. This helper is
# the narrow reviewed boundary, matching the existing tools/ci-incus.sh model.
export HACO_E2E_INCUS=1
exec bash test/e2e/vscode.sh

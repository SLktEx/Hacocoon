#!/usr/bin/env python3
from __future__ import annotations

import check_workflow_policy_base as _base

# Windows Server 2025 is a disposable GitHub-hosted runner, just like the
# supported Ubuntu 26.04 runner. Keep the existing conservative policy logic
# unchanged and extend only the explicit hosted-runner allowlist needed by the
# optional Windows/WSL acceptance job.
_base.APPROVED_RUNNERS.add("windows-2025")

Violation = _base.Violation
check_text = _base.check_text
check_workflow_file = _base.check_workflow_file
discover = _base.discover


def main(argv: list[str] | None = None) -> int:
    return _base.main(argv)


if __name__ == "__main__":
    raise SystemExit(main())

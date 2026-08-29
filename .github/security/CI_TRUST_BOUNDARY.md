# GitHub Actions Trust Boundary

Hacocoon treats workflow configuration as part of the repository security boundary.

Normal pull-request CI is intentionally constrained to disposable GitHub-hosted runners with read-only repository authority, no repository/environment secrets, no persistent cache bridge, and no real privileged Incus or EC2 execution.

`tools/check_workflow_policy.py` encodes defense-in-depth invariants for every file under `.github/workflows/`. It currently requires:

- no `pull_request_target` trigger;
- no `workflow_run` trust bridge;
- only explicitly approved GitHub-hosted runner labels;
- immutable full-SHA pins for external Actions;
- `actions/checkout` with `persist-credentials: false` in PR workflows;
- read-only/none permissions in PR workflows, including no OIDC write authority;
- no repository/environment secret injection into PR workflows;
- no `actions/cache` use in untrusted PR workflows;
- no cross-run/external artifact downloads from PR workflows;
- `actions/setup-go` caching disabled in PR workflows;
- real Incus E2E and experimental EC2 disabled in normal PR CI.

The checker is deliberately small and conservative. If a new workflow pattern is required, update the checker and add a positive/negative regression test in the same reviewed change instead of weakening the policy implicitly.

This is defense in depth, not the primary trust boundary. A malicious pull request can modify both a workflow and this checker. Repository/ruleset controls must still require trusted review for workflow/policy changes, protect `main`, keep fork PR authority read-only/no-secret, and prevent public PRs from selecting persistent privileged self-hosted runners.

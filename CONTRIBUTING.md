# Contributing

Hacocoon optimizes for replacement, deletion, clear responsibility boundaries, operational simplicity, and explicit security boundaries.

## Current contribution policy

Hacocoon is public, but external Pull Requests are intentionally **not accepted yet**. The upstream repository currently uses a solo-maintainer trust model and keeps Pull Request creation limited to collaborators.

External users are welcome to:

- read and fork the repository;
- open Issues for bugs, security-relevant design concerns, questions, and proposals;
- experiment in their own forks.

Do not interpret public visibility as an open-PR policy. The contribution boundary will be revisited when Hacocoon is ready to add another trusted maintainer or accept external Pull Requests safely.

Before changing code, read `AGENTS.md`, `docs/README.md`, `docs/IMPLEMENTATION_STATUS.md`, and the specification/design document relevant to the subsystem you are changing. For version/build work, also read `docs/reference/build-release-identity.md` and `docs/status/checkpoints.yaml`.

Hacocoon is still **pre-1.0**. Breaking changes are allowed when they make the system smaller, safer, or more coherent. Existing behavior on `main` is not automatically a permanent compatibility contract.

## Documentation and change classification

Follow [documentation ownership and writing rules](docs/DOCUMENTATION_STYLE_GUIDE.md).
For checkpoint/release work, use [the numbering policy](docs/status/versioning-and-release-status.md)
and [build identity](docs/reference/build-release-identity.md). Select exactly one
classification in [the PR template](.github/pull_request_template.md).

## Build entry points

From the repository root with the Go version required by [go.mod](go.mod):

```bash
go build -o ./bin/haco ./cmd/haco-product
go build -o ./bin/hacoq ./cmd/haco
```

The second binary is the temporary legacy CLI; it is not the product command.
A standalone CLI build is not an installed Host. Use [installer architecture](docs/design/installer.md)
and [release instructions](docs/guides/releasing.md) for the complete package.
Client-specific development lives in [VS Code Agent Workspace](clients/vscode-agent-workspace/README.md)
and [VS Code notifications](clients/vscode-notify/README.md).

## Design checks

A change should answer these questions:

1. Is this a stable Hacocoon concept, or a concrete provider/tool detail?
2. Does it fit the current architecture boundary, or is it genuinely new product scope that needs a design decision first?
3. Can the implementation be deleted/replaced without rewriting Core?
4. Does the change preserve the trust boundary between the untrusted Environment workload and host/external-service authority?
5. Does any long-lived parent credential, host HOME, Incus control socket, or Hacocoon control state enter the Environment? If yes, redesign it.
6. Is a conditional encoding a real product rule, or should an adapter/strategy own the behavior?
7. Can failure and cleanup be retried or diagnosed without guessing external state?
8. What happens under concurrent calls, cancellation, partial failure, process death, and retry?
9. Does the change accidentally turn an implementation detail into a compatibility promise?
10. If the change is breaking, is the incompatibility deliberate and documented rather than incidental churn?

## Architecture rule

Prefer the current vocabulary: Workspace, WorkspaceLease, Environment, Execution, CapabilityRequest, PolicyDecision, and ApprovalRequest.

Historical `Session`, Runtime/Storage-centric, and plugin-heavy code may remain only where current implementation value justifies it. New public APIs and architecture should not deepen old couplings.

Provider-specific concepts stay outside Core. Incus, Git/GitHub, IDE brands, storage backends, OCI tooling, and future cloud-provider models belong behind their respective boundaries.

## Compatibility rule

Do not preserve accidental behavior solely because it already exists.

A breaking change can be the correct choice when it:

- fixes an unsafe or ambiguous contract;
- removes authority leakage;
- makes recovery/failure semantics explicit;
- separates provider-specific behavior from Core;
- deletes an abstraction that no longer earns its complexity.

When breaking behavior, document operator-visible impact, avoid silent data loss, update tests, and provide migration guidance when a supported migration path exists.

## Security review

For security-sensitive changes, review the code adversarially rather than only validating the happy path. Follow `.github/security/ADVERSARIAL_AUDIT.md` for a full hostile audit.

At minimum consider:

- path traversal / symlink / TOCTOU;
- shell, argument, and option injection;
- authorization and confused-deputy failures;
- secret exposure through argv/env/log/tempfiles;
- concurrent and duplicate operations;
- timeout/cancellation/process crash;
- destructive retry and partial cleanup;
- hostile backend/provider/guest output.

## Definition of done

As applicable, add positive tests, negative/security tests, cleanup/retry tests, concurrency/race coverage, no-secret logging checks, and regression tests for retained behavior.

Run the maintained local CI entry point:

```bash
bash tools/ci-local.sh
```

It mirrors the jobs in `.github/workflows/test.yml` and can also run one job at a time:

```bash
bash tools/ci-local.sh docs
bash tools/ci-local.sh workflow-policy
bash tools/ci-local.sh release-config
bash tools/ci-local.sh systemd
bash tools/ci-local.sh test
bash tools/ci-local.sh race
bash tools/ci-local.sh e2e
```

The local run uses the `go` binary currently on `PATH`; GitHub Actions still exercises the supported Go-version matrix separately. `release-config` requires the same local tooling used by that CI job, including GoReleaser, PowerShell, and `systemd-analyze`. It refuses to invoke `goreleaser release --clean` when `dist/` already exists so local artifacts are not silently deleted.

Run relevant provider-backed coverage as well. Do not claim real Incus or other environment-dependent acceptance unless those provider-backed tests actually ran.

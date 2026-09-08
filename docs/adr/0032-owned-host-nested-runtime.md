# ADR 0032: Enable nested runtimes on the owned Host

Status: accepted; owned-provider nesting verified; OCI runtime acceptance pending
Date: 2026-09-08

## Decision

The maintained local OCI integration enables Incus `security.nesting=true`
during ordinary setup after the canonical Host source becomes ready. This
permits nested runtime operations inside the unprivileged trusted `haco-host`.
It changes the container isolation configuration; it does not grant Incus
management authority or make the Host a privileged container.

The provider shares the Host start/copy lock and verifies the exact project,
instance name, local ownership and source-readiness markers, container type,
empty profiles, running state, absent copy journal, unprivileged configuration,
source-volume ownership and sole attachment at the managed path. Bounded daemon
configuration verification is also required. Missing or ambiguous evidence
refuses the mutation. The setting is read back with the same checks; uncertain
completion returns recovery-required without removing resource ownership.

Already enabled, verified instances are reused. The setting persists through
Host restart and normal setup. It is not temporarily reverted after each command.
No syscall intercept or privileged-container settings are enabled by this change.
Raw Incus sockets and unrelated Physical Host data remain outside the Host.
Environment isolation and credential boundaries remain governed by their own
provider contract.

## Rejected alternatives

- Changing every instance or trusting its name alone can modify unrelated users'
  resources.
- Inherited role markers or profiles cannot establish this locally owned Host.
- Unconditionally enabling privileged mode grants authority beyond nesting.
- Treating a successful setter as verified state hides ambiguous provider output.

## Acceptance

Adversarial component tests cover foreign/inherited ownership, other consumers,
wrong mounts, profiles, privileged or paused instances, pending copies and failed
or truncated observations. A real owned-project E2E checks enabling/reuse and a
nested mount namespace, then removes its owned fixture. Actual Docker/nerdctl
image recovery after area copying is a separate acceptance requirement; namespace
success alone must not be reported as runtime compatibility.

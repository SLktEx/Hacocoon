# Project setup

[日本語](project-setup.ja.md) | English

Status: **implemented explicit recipe slice; roadmap C4 acceptance is partial**.
Installed GHA acceptance is pending.

## Ordinary use

Use the existing setup command with an explicit Environment target:

```bash
haco setup --script ./dev-setup.sh dev
haco setup dev
haco setup --clear-script dev
```

The first command saves a bounded snapshot and executes it inside dev. The second
replays that snapshot; clear removes it without execution. Recipes belong to the
canonical Workspace identity, so a new Environment for the same Workspace can
reuse them with one setup command. The original script file may be versioned in
the repository, but editing it does not silently change the saved snapshot.

Omitting the Environment keeps the existing trusted Host setup behavior.
Environment project setup must never inherit a Host recipe, credential or
management channel. No repository hook is discovered or executed automatically.
Base tooling and optional OCI content remain separate from project dependencies.

## Ownership and execution

Reuse the existing private atomic recipe storage through a neutral internal
component. Host and Workspace recipe roots, execution adapters and failure
boundaries remain separate. Script input is explicit UTF-8, at most 1 MiB, with
no NUL; symlink/FIFO and unsafe storage protections remain enforced.

Resolve the target through the canonical Environment/Workspace catalog. Execute
under a lifecycle guard that verifies the expected Workspace identity before
provider work, preventing name reuse from redirecting a saved recipe to another
Workspace. A conflicting lifecycle operation must wait or report busy.

Scripts travel as bounded stdin, never Host argv or Host shell code. The provider
must explicitly support that contract. The local adapter runs in /workspace
inside the owned Environment with a fixed transient unit and independent bounded
runtime/descendant cleanup. No Host credential, controller socket, Windows drive
or Windows process path is introduced. Existing DNS and egress Policy still apply.

Saving is durable before execution. Failure keeps the recipe and Environment for
inspection/retry and must not report success. Clearing a recipe does not undo
previous file changes. Execution output is a bounded command result, never a
structured log field; error/audit boundaries must not expose script bytes.

## Required acceptance

Cover save/replay/update/clear, reuse in a second Environment with the same
Workspace, refusal of another Workspace or recycled name, concurrent setup and
inverse lifecycle operations, nonzero status, cancellation and bounded cleanup.
Keep malformed stdin/storage and logging regressions at the lowest faithful
layer. Installed GHA should run an explicit nonce recipe through ordinary haco
commands; package installation must use scoped Policy and be reported separately.
Physical/VPN-dependent gaps remain explicit SKIP items, not inferred success.

See [Base boundaries](base-images-and-custom-environments.md#project-setup-boundary)
and [trusted Host setup](trusted-host.md).

Component tests pass for Workspace-scoped save/replay/clear, retained recipes
after failure, identity-bound start refusal, strict controller requests and
bounded stdin transfer. Installed Windows GHA now includes explicit
save/replay/nonzero/update/clear acceptance. At `c05528a`, the harness failed
before setup execution because a CRLF script reached Bash. The harness now
normalizes scripts to LF; acceptance awaits a rerun.
Package installation, cancellation descendant cleanup and reuse after actual
Environment recreation remain unverified at the provider acceptance layer.

At `5f824b4`, Windows DNS and VS Code acceptance passed again, but project setup
failed after the harness correction. The production Incus command decorators
dropped the optional stdin interface, so the runtime rejected setup as unsupported.
Both decorators now preserve stdin for Incus exec only; management commands stay
on their existing ownership-checked route. A regression uses the production
decorator chain and checks stdin/result forwarding, unsupported backends and
management-operation refusal. Installed setup acceptance still awaits a rerun.

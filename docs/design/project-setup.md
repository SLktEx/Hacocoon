# Project setup

[日本語](project-setup.ja.md) | English

Status: **planned roadmap C4**. This contract is not an available command yet.

## Ordinary use

Extend the existing setup command with an explicit Environment target:

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

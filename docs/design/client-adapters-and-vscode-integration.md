# Client adapters and VS Code integration

Normal product development uses `haco ssh setup` and `haco open`.
The [client adapter contract](../reference/client-adapter.md) owns reusable APIs;
[this access design](client-and-interactive-access.md) owns connection authority.
IDE UX and orchestration remain outside Core.

## Retained standalone adapter

`haco-vscode open <workspace>` and `haco-vscode delete <workspace>` are separate
adapter commands for an external-path Workspace in the execution host.
They create/reuse an exact Workspace/access-mode match, reserve loopback SSH,
write an owned client config fragment and launch standard Remote-SSH.
This is not the managed repo/Workspace product tutorial.

Options include `--name`, `--identity`, `--host-port`, `--read-only`,
`--no-launch` and `--code`. The matching public key is supplied to Hacocoon;
the private key remains client-side. The adapter adds an Include for
`~/.ssh/hacocoon/*.conf`, preserves unrelated SSH entries and removes only its
own fragment on deletion. WSL execution targets the Windows client SSH home.

Per-session routing uses the separate
[Agent Host adapter](vscode-remote-agent-host-adapter.md).
Future JetBrains/code-server/other clients may reuse the generic contract;
their names are not implemented product commands.

## Runtime-selected ports

Product `haco env ssh --key <public-key-file> <name>` passes default port zero to
the runtime; `--port` is optional. The Incus adapter probes on the Physical Host,
then reserves the proxy before installing a key. A competing bind fails without
granting a key; arbitrary provisioning failure is not retried. The trusted Host's
different network namespace must not choose the Physical Host listener.

## Desktop SSH setup and VS Code opening

Status: **implemented; scoped installed Windows/editor acceptance is recorded in [evidence](../status/acceptance-evidence.md#development)**.

```sh
haco ssh setup dev
ssh haco-dev
haco open dev
haco open --client ssh dev
```

If exactly one Environment exists, omit `dev`. With multiple Environments,
an interactive terminal lists names with Workspace IDs and access modes, then
accepts a number in the same invocation. Blank input cancels before desktop or
connection setup. Noninteractive callers must supply a name and their stdin is
not consumed. The selected creation/runtime identity, Workspace and access mode
are rechecked during ordinary setup; a stale selection asks the user to select
again. This check does not replace controller lifecycle validation. `haco open`
performs the same setup then launches the installed VS Code on `/workspace`
through Remote-SSH; if its extension is absent, the client installs it with the
installed VS Code CLI before launch. Installation failure is reported as failure.
SSH setup itself remains editor-neutral. `haco open` defaults to VS Code, while
`--client ssh` opens a terminal in `/workspace` without requiring an editor.
Remote-SSH needs its requested dynamic forwarding: generated settings use
`ClearAllForwardings no` and `GatewayPorts no`; agent forwarding remains disabled.

On first access to a Base without `sshd`, the existing SSH preparation installs
`openssh-server` through the Env's Policy-controlled package transport. Default deny
can prevent that preparation; a generic connection failure does not prove either
a package failure or pending approval. The CLI routes inspection to
`haco env status`, `haco doctor`, read-only `haco approve --list` and `haco config`.
Review the actual Base's package endpoints and current Env scope before changing
Policy. After a failed attempt, inspect connections before disconnecting or
trying again; preparation is not permission to grant network access automatically.

In WSL (including trusted haco-host), the client resolves the Windows profile and
uses Windows ssh-keygen. On Linux it uses the local client home. The private key
stays under that client's `~/.ssh/hacocoon/identity`; only its public key reaches
the controller. A global Include is prepended to the existing UTF-8 SSH config.
Managed entries and host-key pins use separate files below `.ssh/hacocoon`.
Existing unrelated SSH configuration is preserved. Symlink/hardlink config files,
non-private Linux managed directories, malformed provider data and host-key changes
for the same runtime fail closed. Writes use a confined filesystem root, a setup
lock and atomic replacement. Windows uses the client's inherited filesystem ACLs
and native key-generation permissions; no client directory is exposed to workloads.

A stopped Environment is resumed through the canonical start operation. Setup
reuses a matching live connection and restores its managed files from the pinned
metadata. New host-key files are keyed by runtime identity and public key.
If connection preparation succeeds but local installation fails, its ID is reported
as recovery-required and retained for inspection rather than silently discarded.
Use the existing `haco env disconnect <name> <connection-id>` after inspection.

Repository regressions cover real ssh-keygen, preservation, reuse/resume and hostile
file/config/provider input. The maintained Windows acceptance additionally exercises
the default client home on the disposable GHA user, native SSH, and stopped resume.
Local manual runs of that fixture SKIP home modification; they continue the existing
explicit-key SSH test. VS Code process launch alone is not proof of editor/server
connection, terminal or debugger acceptance.

### Disposable Windows editor acceptance

Status: **implemented validation with candidate-bound acceptance; see the evidence record above**. The existing Windows
installer GHA prepares a SHA-256-pinned portable VS Code before ordinary installation
captures the Windows PATH. The SSH fixture then calls ordinary `haco open` and a
UI-only observer checks the exact Environment authority, reads the Workspace marker,
writes and opens a remote document, executes a remote terminal command, and removes
both probes. A launch exit code or SSH configuration alone cannot pass this check.
The detached GUI does not inherit command-output pipes, so the calling shell can return.

The disposable portable profile downloads the Remote-SSH server on Windows and
transfers it through SSH. Its Workspace trust prompt is disabled only in that test
profile; normal user trust prompts and local desktop acceptance remain unverified.
The observer is a test artifact, never a product extension or workload dependency.
Only bounded stage/result metadata is reported; remote logs and server tokens are
not uploaded. Failure and timeout fail the job. The local package-egress approval
question does not grant permission to change local policy through this fixture.

Windows editor discovery prefers the exact `code.cmd` in the trusted Host's captured
Windows PATH. A native child can have a different PATH; extension preparation uses
the selected CLI path as encoded data rather than resolving a second executable.
Native Windows PATH discovery remains the fallback when the captured PATH has no CLI.

The disposable editor fixture also saves the known Linux platform for its exact
SSH alias, matching Remote-SSH's first-use platform choice. That choice and the
Workspace trust prompt are outside automated acceptance; the fixture still uses
the installed product command, SSH transport and remote filesystem/terminal.

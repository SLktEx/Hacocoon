# v0.8 Client Adapters & VS Code Integration

**Status:** implementation gate introduced on 2026-08-29  
**Compatibility:** pre-1.0; interfaces and commands may change incompatibly.

## Goal

v0.8 makes the intended Hacocoon user experience concrete without turning Hacocoon into an IDE or an AI orchestration product.

The primary local workflow is:

```text
VS Code UI / built-in or installed AI UI
                |
        Hacocoon client adapter
                |
        Hacocoon Environment
                |
       loopback-only SSH access
                |
          Incus Environment
                |
      /workspace + developer tools
```

A user should be able to open a normal local Workspace through Hacocoon, reconnect VS Code through standard Remote-SSH, and then use the normal VS Code terminal, debugger, source-control UI, and AI/coding-agent UI inside the Environment.

Hacocoon does **not** provide a replacement chat interface, model selector, task planner, or editor. The IDE remains the client.

## Product boundary

v0.8 formalizes **Client Adapter** as an integration layer outside Core.

A Client Adapter may:

- select or create a Hacocoon Environment for a Workspace;
- request standard Hacocoon client access such as SSH;
- translate Hacocoon connection data into client-native configuration;
- launch or reconnect a client;
- surface Environment status or Hacocoon security approval state;
- clean up adapter-owned connection metadata.

A Client Adapter must not:

- move VS Code, JetBrains, Daintree, or another concrete client into Core;
- own AI task decomposition, model routing, retry policy, or budgets;
- bypass Hacocoon Policy/Capability boundaries for host or external-service authority;
- copy broad host credentials into the Environment for convenience;
- expose SSH or application ports broadly when the Hacocoon client-access contract is loopback-only.

## First adapter: `haco-vscode`

v0.8 ships a thin Go helper named `haco-vscode` as the first concrete Client Adapter.

The MVP command is:

```bash
haco-vscode open .
```

The adapter performs this sequence:

```text
resolve Workspace
  -> derive/select Environment name
  -> create or reuse matching Hacocoon Environment
  -> prepare loopback-only SSH access
  -> create adapter-owned SSH host config
  -> launch:
       code --remote ssh-remote+<alias> /workspace
```

The corresponding cleanup command is:

```bash
haco-vscode delete .
```

Useful options include:

```text
--name <environment>
--identity <private-key-path>
--host-port <port>
--read-only
--no-launch
--code <code-cli-command>
```

The private SSH key remains on the client side. Hacocoon reads only the matching `.pub` file when preparing Environment access.

## SSH configuration ownership

The adapter must not replace or regenerate the user's SSH configuration.

It adds one include directive when absent:

```sshconfig
Include ~/.ssh/hacocoon/*.conf
```

Hacocoon-owned host entries live under:

```text
~/.ssh/hacocoon/
```

Each Environment receives a separate adapter-owned file. Deleting an Environment through `haco-vscode delete` removes that adapter-owned host entry while leaving unrelated SSH configuration untouched.

## Windows + WSL behavior

The default local Hacocoon deployment may run inside WSL while desktop VS Code runs on Windows.

Those are different filesystem and SSH-client contexts. Therefore `haco-vscode` treats the VS Code client-side SSH home separately from the Linux execution host:

```text
Windows VS Code
  -> Windows OpenSSH config / key
  -> 127.0.0.1:<loopback port>
  -> WSL / Hacocoon
  -> Incus Environment:22
```

When executed inside WSL, the adapter resolves the Windows user profile and manages the Windows-side `.ssh` configuration rather than incorrectly writing only the WSL user's `.ssh/config`.

The adapter still runs as an ordinary user-space integration helper. It does not make Windows, WSL, or VS Code part of Hacocoon Core.

## AI YOLO boundary

The motivating workflow is to let a coding agent operate with broad freedom **inside the isolated Environment** while preserving a hard boundary around host/external authority.

```text
VS Code AI / Codex / Copilot / Claude / other agent
                         |
                         v
                 Incus Environment
                "YOLO is acceptable"
                         |
              ---- trust boundary ----
                         |
                     Hacocoon
             Policy / Capability / Audit
                         |
              GitHub / AWS / Host / etc.
```

Commands such as package installation, builds, tests, code edits, or destructive changes inside the disposable Environment may be intentionally permissive.

Operations that require authority outside the Environment remain mediated by Hacocoon capabilities. Client convenience must never silently convert Environment freedom into host freedom.

## Other clients

VS Code is the first adapter, not the product boundary.

Future integrations may use the same generic Hacocoon Environment/client-access contract, for example:

```text
haco-vscode
haco-jetbrains
haco-daintree
haco-web
custom orchestrator adapters
```

These names are examples, not commitments to implement every adapter in v0.8.

A future VS Code extension may provide buttons, status, notifications, or approval UX, but it should remain a thin adapter over the same Hacocoon interfaces. It must not become a required transport or duplicate Remote-SSH.

## Daintree / orchestrator boundary

Client adapters and AI orchestrators remain distinct concepts.

A tool such as Daintree may own tasks, worktrees, agent selection, parallelism, retries, and development review. Hacocoon owns the isolated Environment and security boundary underneath it.

```text
Daintree / other orchestrator
          |
       Workspace
          |
      Hacocoon
          |
  Environment + access
          |
       Incus
```

The same Workspace may instead be opened by a human through `haco-vscode`. Neither path requires Hacocoon Core to understand Daintree or VS Code.

## v0.8 acceptance gate

The v0.8 implementation gate requires:

1. A client-adapter boundary is documented without adding VS Code-specific concepts to Core.
2. `haco-vscode` builds as a separate binary.
3. `haco-vscode open <workspace>` can create or reuse the matching Environment.
4. SSH access remains loopback-only and uses the existing hardened Hacocoon SSH path.
5. The adapter keeps the private key outside the Environment and supplies only the public key to Hacocoon.
6. Adapter-owned SSH config is isolated from unrelated user SSH entries.
7. WSL execution targets the Windows client-side SSH configuration when desktop VS Code is the client.
8. The adapter launches standard VS Code Remote-SSH rather than implementing a proprietary remote protocol.
9. `haco-vscode delete` removes the Hacocoon Environment and its adapter-owned SSH host entry.
10. Existing Hacocoon Policy/Capability/Audit behavior remains the boundary for authority outside the Environment.
11. Unit tests cover deterministic Environment naming and SSH configuration management.
12. Repository build, vet, race, docs-consistency, and existing tests remain green.

Real Incus + Windows VS Code Remote-SSH acceptance remains an environment-dependent acceptance test and must be reported separately from unit/CI success.

## Non-goals

v0.8 does not require:

- a custom AI chat UI;
- a VS Code-specific AI agent;
- a replacement for Remote-SSH;
- a VS Code extension as a mandatory component;
- Daintree or another orchestrator as a dependency;
- IDE concepts in Core;
- automatic Git branch/worktree management;
- broad LAN/public SSH exposure;
- weakening the Capability boundary so agents can obtain host credentials.

## One-sentence definition

> **v0.8 lets standard developer clients—starting with VS Code Remote-SSH—enter a Hacocoon Environment with minimal glue while keeping IDE UX and AI orchestration outside Core.**

## Automatic SSH ports in the product client

Status: **implemented**. `haco env ssh --key <public-key-file> <name>`
defaults to runtime-selected loopback access; `--port` is optional. Port zero
travels unchanged through the controller and client adapter. The Incus integration
probes on the Physical Host, then reserves its proxy before installing credentials.
The probe alone does not guarantee a bind: a competing bind returns failure
without changing guest keys. No arbitrary provisioning failure is retried.
This prevents trusted haco-host from selecting ports in its own network namespace.

Product SSH setup and VS Code launch are implemented below; installed acceptance
remains pending. The temporary run-and-remove UX follows usable VS Code connectivity.

## Desktop SSH setup and VS Code opening

Status: **implemented product commands; installed Windows and editor acceptance pending**.

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

Status: **implemented validation; first editor run pending**. The existing Windows
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

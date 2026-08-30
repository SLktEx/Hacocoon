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

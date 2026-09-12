# Client and interactive access

Status: **implemented**. Client-specific acceptance is tracked in
[implementation status](../IMPLEMENTATION_STATUS.md).

Clients use standard SSH or a constrained local connection. Hacocoon owns Environment
identity and connection lifecycle; clients retain private keys and IDE configuration.
No IDE brand, remote UI or agent router is a Core requirement.

## Ordinary product route

From trusted Host, `haco ssh setup [env]` installs/reuses the client key and pinned
configuration; `haco open --client ssh [env]` opens a shell in `/workspace`.
Plain `haco open [env]` defaults to VS Code. Stopped Envs resume under the canonical
lifecycle. An ambiguous selection requires a terminal choice or an explicit name.

[Getting started](../guides/getting-started.md) owns the complete user procedure.
[Windows SSH reference](../reference/windows-environment-ssh.md) covers manual keys,
loopback transport and strict host-key pinning. Product commands are
`haco env status`, `haco env ssh`, `ssh-config` and `disconnect`, not historical
root-level status/forward/unforward commands.

## Connection authority

SSH receives only a structurally validated OpenSSH public key. The private key never
enters the Environment. Reserve the loopback proxy before installing a connection-scoped
managed key; a failed reservation cannot leave a grant behind. Later setup failure
cleans the reserved proxy. Revocation removes the managed key before the proxy;
unrelated authorized keys are preserved. It does not stop sshd merely because other
clients may use it.

The Incus adapter derives native connections from owned proxy devices and reconciles
them without a second connection-state database. Results are revalidated as loopback-only;
an omitted protocol normalizes to TCP. Broad/LAN/public exposure is not this contract.
Guest-supplied address/key/connection data never establishes provider ownership.

Service authentication remains the service/client's responsibility. Code-server or
another web application can run inside an Env; use [restricted preview](development-preview.md)
for supported HTTP access rather than opening an unrestricted network listener.

## Reusable client contract

`pkg/clientadapter` composes ensure/reuse, state, Workspace discovery, SSH/TCP
connect, revoke/delete and interaction events. Reuse requires exact Workspace
identity and access mode; a matching name alone is insufficient. VS Code,
code-server and future clients can reuse that boundary.

See [client adapter API](../reference/client-adapter.md),
[controller transport](controller-client-transport.md) and
[VS Code integration](client-adapters-and-vscode-integration.md).
Legacy adapter CLI recipes remain on temporary `hacoq`; they do not expand product
`haco` availability. Neither a successful repository test nor a loopback address
alone proves real Windows/WSL or IDE acceptance.

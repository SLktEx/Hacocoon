# CODEX START HERE

You are implementing Hacocoon from an existing repository that may contain code based on a historical, oversized v0.1 design.

## Your first job

Do **not** implement v0.1-v0.7 at once.

1. Read the current architecture and v0.1 spec.
2. Inventory the repository and current tests.
3. Copy `docs/IMPLEMENTATION_STATUS_TEMPLATE.md` to the repository's working documentation and fill it in.
4. Preserve known-good code where possible.
5. Extract concrete dependencies behind the current architecture boundaries.
6. Make the v0.1 acceptance suite pass.
7. Stop at the v0.1 gate unless explicitly instructed to proceed.

## The order is authoritative

```text
0.1 Local Foundation
0.2 Developer Workspace
0.3 Security Framework + Git
0.4 External Capabilities (GitHub/AWS access/Registry)
0.5 Local GUI + IDE
0.6 Local Web + Interaction
0.7 Remote + EC2/EBS
```

AWS capability access is **not** postponed to v0.7. AWS delegated credentials and Packer/Terraform compatibility belong to v0.4. Only EC2-host/runtime/deployment and EBS-specific behavior belong to v0.7.

## Hard architecture rules

- Core describes Hacocoon concepts and orchestration, not Incus/Btrfs/QCOW2/AWS/GitHub/VS Code/WSLg implementation details.
- A Session is untrusted; Manager/host authority and parent credentials stay outside it.
- Security Framework is the authorization authority. Plugins do not approve themselves.
- Do not mount host HOME, `~/.ssh`, `~/.aws`, GitHub tokens, Incus sockets, or Manager state into a Session as a convenience shortcut.
- Do not build a Hacocoon wrapper for every developer CLI. Integrate at standard provider/protocol/credential boundaries.
- Do not build a proprietary IDE/editor/chat surface. Existing IDEs and browser clients are adapters.
- Do not add remote/EC2 conditionals throughout Core. Remote/EC2 arrives behind Runtime/Host/Storage/Access seams in v0.7.
- Every privileged failure is fail-closed.
- Every implementation issue needs positive, negative/security, cleanup/retry, and regression tests where applicable.

When uncertain, choose the solution that preserves: trust boundary -> no credential leakage -> Session-local freedom -> standard protocol -> tiny Core -> replaceability -> simplicity.

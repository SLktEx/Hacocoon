# Hacocoon

**Pronounced: はこーん (ha-kōn)**

Hacocoon is an OSS disposable Linux development-session runtime for humans and coding agents.

The project is intentionally built around a tiny vendor-neutral Core. Concrete infrastructure such as Incus, Btrfs, QCOW2, GitHub, AWS, VS Code and WSLg lives behind replaceable modules/plugins instead of leaking into Core.

## Current implementation target

This repository is currently implementing **v0.1 Local Foundation** only. The authoritative roadmap is:

1. v0.1 Local Foundation
2. v0.2 Developer Workspace
3. v0.3 Security Framework + Git
4. v0.4 External Capabilities
5. v0.5 Local GUI + IDE
6. v0.6 Local Web + Interaction
7. v0.7 Remote + EC2/EBS

See `CODEX_START_HERE.md` and `docs/` before extending the implementation.

## Design rules

- Session is untrusted; Manager/host authority stays outside it.
- Core does not import Incus/Btrfs/QCOW2/AWS/GitHub implementation packages.
- Storage shrink is inner-to-outer: filesystem first, backing image second.
- Host credentials, Incus socket and block-control interfaces are never mounted into a Session for convenience.
- Standard tools/protocols remain the UX where practical.
- Implement one release gate at a time.

## Build and test

```bash
go test ./...
go build ./cmd/haco
```

## v0.1 CLI surface

```text
haco init
haco doctor
haco new [name]
haco list
haco status <session>
haco start <session>
haco stop <session>
haco rm <session>
haco exec <session> -- <command...>
haco shell <session>
haco storage status
haco storage grow --to <size>
haco storage plan-shrink --to <size>
haco storage shrink --to <size>
haco storage compact
```

The real Incus/Btrfs/QEMU paths require their host tools. Core unit tests do not.

# Hacocoon CLI reset and temporary `hacoq`

Hacocoon is rebuilding its product-facing CLI from the basic user workflow outward.

## Command names during the migration

- `haco` is the product-facing CLI. New user workflows are added here only when they are ready to be part of the supported product experience.
- `hacoq` is the previous `haco` implementation under a temporary migration-only name.

`hacoq` is **not** an advanced or expert CLI and is **not** a permanent public interface. No new product features should be added to it. It exists only so existing low-level functionality remains reachable while equivalent product workflows are rebuilt or intentionally removed.

The intended end state is to delete `hacoq` completely.

## Initial product CLI surface

The first reset slice intentionally exposes only standalone commands:

```text
haco --version
haco version
haco version --json
haco help
haco --help
```

These commands do not require Incus, the Hacocoon controller, `haco-host`, Hacocoon state directories, network access, or root privileges.

Unimplemented product commands fail clearly instead of falling back to the legacy runtime stack.

## Compatibility boundary

Release archives and installers temporarily contain both `haco` and `hacoq`.

The trusted `haco-host` also receives both names:

- `/usr/local/bin/haco` — the new product CLI
- `/usr/local/bin/hacoq` — the temporary legacy controller/runtime CLI

The same `haco` name therefore refers to the same product surface on the Physical Host, inside trusted `haco-host`, and through the Windows launcher.

The Windows `haco` launcher keeps Windows-owned maintenance operations such as `haco maintenance compact` on Windows and delegates normal product CLI commands to `/usr/local/bin/haco` in the dedicated Hacocoon WSL distribution.

## Development rule

Do not implement new workflows by adding commands to `hacoq`, and do not make the new `haco` shell out to `hacoq` for ordinary product operations. Shared behavior should move behind reusable Go packages or controller APIs as each product command is rebuilt.

The WSL interactive-login shim is a temporary migration exception: it can use `hacoq` only to preserve entry into the trusted `haco-host` until that bootstrap path is moved behind the new product surface.

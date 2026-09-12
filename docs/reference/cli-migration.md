# Temporary legacy CLI migration

Product `haco` is built from `cmd/haco-product`; temporary `hacoq` is the previous
implementation from `cmd/haco`. They are not aliases. Use [getting started](../guides/getting-started.md)
and the [current CLI reference](cli.md) for ordinary development.

`hacoq` exists only for retained migration functionality, not as a permanent expert
CLI. New features must use product/controller contracts; the intended end state is
its deletion. Product commands do not silently fall back to legacy composition.

## Current equivalents

| Historical spelling / use | Current route |
|---|---|
| `haco create/list/status/delete` | `haco env create/list/status/delete`; managed Workspace preparation is explicit |
| Direct `haco ssh` with a name | `haco ssh setup [env]` then `haco open --client ssh [env]`; manual public key route is `haco env ssh` |
| `haco exec/shell` on a retained Env | Ordinary SSH; `haco run -- ...` for a new temporary Env |
| Host-side Git worktree capability | Managed `repo clone` → `workspace create` → `env create` → `git connect` and ordinary guest Git |
| `haco host ensure` / old bootstrap | `haco setup`; `hacoq host ensure` now refuses before composition |
| Seed/Docker plugin commands, raw event export, resource-budget flags | Retained legacy surfaces below; no assumed product equivalent |
| `haco env switch-base`, old OCI distribution | Disabled/removed public behavior; preserve data with the [ordinary recreation lifecycle](../guides/data-lifetime.md) |

<a id="host-entry"></a>

## Temporary native Ubuntu Host entry

Windows interactive `wsl -d Hacocoon` enters trusted Host directly through the product
login alias. Native Ubuntu leaves its login shell unchanged and currently uses this
temporary command **on the Physical Host**:

```bash
hacoq host shell
```

Once inside, use product `haco`. Fresh trusted Host provisioning contains product
`haco` and client-only `haco-host`, not legacy guest `hacoq`.
There is no product `haco host shell` command and no native Windows `haco.exe`.

## Retained legacy operations

Run legacy commands only in a deliberately configured Physical Host/development
composition with its own required Policy/runtime configuration. Do not start a second
legacy broker alongside an installed controller as a normal setup step.

- `hacoq plugin git ...`: [legacy Git capability](legacy-git.md); separate from managed ordinary Git.
- `hacoq plugin oci seed ...`: [Seed implementation](../design/oci-seed-and-cow.md) and [recommendation](../design/oci-seed-recommendation.md).
- `hacoq plugin oci docker ...`: [Docker compatibility](../design/docker-compatibility-plugin.md).
- `hacoq create/run --cpu/--memory/--pids/--root-size ...`: [resource-budget contract](../design/sandbox-resource-limits.md). These flags are absent from product create/run.
- `hacoq events --json [--since-offset <offset>]`: audit-derived legacy event export below.
- Legacy client adapters may expose create/connect/revoke APIs directly; [client contracts](client-adapter.md) describe that integration rather than product command availability.

Optional legacy OCI selection is `HACO_PLUGIN_OCI=nerdctl` or `docker`; unset leaves
Core usable without these tools. Persistent OCI Stores use the current product
workflow and are not synonymous with Seed construction.

## Legacy event cursor

`hacoq events --json` emits one audit-derived JSON record at a time with request
correlation, safe policy fields and `next_offset`; opaque capability parameters
are excluded. Memory is bounded per record, though a full read still performs O(N) I/O.
Persist the last successfully consumed offset and pass it to `--since-offset`.

Zero starts the current file; other offsets must be record boundaries and cannot
exceed its size. Do not advance past undelivered events. Appending preserves cursors;
rotation/replacement requires an explicit reset to zero. Raw offsets cannot detect
replacement by a file at least as large. Corruption stops after trustworthy preceding
records and returns an error; byte offsets are absolute, resumed line numbers relative.
Observation never grants approval. Current notifications use [interaction events](interaction-events.md).

## Packages and compatibility

Linux/WSL release packages temporarily include both binaries. A standalone legacy
binary is not an installer or a supported old-state migration tool. Pre-1.0 arbitrary
installer/state compatibility is not promised; failure must retain evidence rather
than silently adopting old authority. Check the exact installed build identity.

Shared behavior should move into reusable packages/controller APIs, never new
product-to-`hacoq` subprocess wrappers. Help/version remain usable without Incus,
controller state or root.

# Incus PID record boot identity

Status: accepted.

## Context

Incus 6.0.5 persists subprocess PIDs in network and proxy metadata. After a WSL
restart a saved dnsmasq PID can identify an Incus worker thread. Replaying
SIGKILL terminates the daemon and blocks the controller's startup dependency.
The trace and package identity are in [implementation status](../IMPLEMENTATION_STATUS.md).
Kernel boot ID alone is insufficient: WSL can restart a distribution's PID
namespace while its shared kernel remains alive.

## Decision

The Ubuntu Incus integration installs a root-only ExecStartPre guard. It records
kernel boot UUID, namespace init start ticks and PID namespace inode under
`/var/lib/incus/.hacocoon-boot-guard`. An unchanged namespace retains all records,
including on ordinary service restarts. On a changed namespace, processes from
the old namespace cannot survive as processes of the new namespace. The guard
archives only `networks/*/dnsmasq.pid` and `devices/*/proxy.*` before Incus reads
them. Contents are not interpreted and no process is signalled. Configuration,
instances, disks, Workspaces and resource ownership remain unchanged.

First installation adopts the current namespace only with a root Incus daemon
at the fixed socket in the same PID namespace; the installer first requires
`incus info` readiness. Unmarked existing records cannot be retired speculatively.
Initialization cannot overwrite a different namespace marker. Fresh empty state
can initialize during ExecStartPre. An already stuck older installation must
first regain daemon readiness; this installer does not force-kill or delete state.

The helper opens all path components without symlink traversal. Metadata must
be root-owned and not group/other-writable; files must be single-link regular
files. FIFO opens are nonblocking. A root-owned flock serializes executions.
Preflight checks every selected record before any move. Archive directory
entries and moved files are synced before atomic marker replacement. On
interruption, the old marker remains and a retry archives the remaining files.
Running-daemon conflicts, corrupt state, changing files and filesystem failures
refuse startup. The CLI has fixed paths, isolated Python, no force option and
fixed failure messages without metadata contents.

This is provider integration, not a Core repair API. Clients and workloads gain
no access to Incus state. The guard runs for native Ubuntu and WSL alike.

## Rejected alternatives

- A longer login timeout does not prevent the incorrect SIGKILL.
- Core/client manipulation of provider state crosses ownership boundaries.
- Removing PID files on every service restart loses live helper ownership.
- PID/name/command matching alone leaves reuse and spoofing races. Archiving
  across proven namespace boots avoids signalling altogether.
- Rebuilding the entire Incus distribution adds a separate provider supply
  chain and compatibility obligation for this bounded fix.

## Limits and verification

This prevents cross-namespace replay of network/proxy records, not every upstream
process ownership bug. Same-namespace helper death/PID reuse and other optional
device families still need provider-native process identity. Archives are
retained; automatic garbage collection is not included.

Component tests cover reused PIDs, WSL/native boots, same-namespace restart,
adoption, concurrency, interrupted moves and hostile metadata. The packaged
Windows restart gate verifies current marker identity and retained dnsmasq records
before an installer rerun. Repository tests, hosted provider acceptance and
installation on an existing user's machine remain separate claims.

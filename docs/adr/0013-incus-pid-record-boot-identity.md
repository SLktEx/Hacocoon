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
archives only existing `networks/*/dnsmasq.pid` and `devices/*/proxy.*` before Incus
reads them. If normal provider shutdown already removed the records, the guard
advances the namespace marker without creating an empty or fabricated archive. Contents are not interpreted and no process is signalled. Configuration,
instances, disks, Workspaces and resource ownership remain unchanged.

First installation adopts the current namespace only with the root Incus daemon
matching systemd's MainPID in the same PID namespace; the installer first requires
`incus info` readiness. Unmarked existing records cannot be retired speculatively.
Initialization cannot overwrite a different namespace marker. Fresh empty state
can initialize during ExecStartPre. An already stuck older installation must
first regain daemon readiness; this installer does not force-kill or delete state.

The systemd activation socket is not a daemon-liveness signal: PID 1 owns the
listener before and after Incus starts. The guard checks actual executable and
namespace identity through procfs, including replaced executables. Incus
processes without a matching managed daemon refuse retirement and adoption.

The exact daemon allowlist covers Ubuntu's `/usr/libexec/incus/incusd` and the
Zabbly LTS package's `/opt/incus/bin/incusd`. The latter is launched through the
vendor systemd wrapper, whose pathname is not accepted as the daemon itself.
Both paths require root ownership of the process, the current PID namespace and
the matching systemd MainPID. A nonzero MainPID with an unrecognized executable
or namespace is an ambiguous running service and refuses retirement; it must
never be treated as proof that Incus is absent. The shared 7.0 installer exposed
the missing vendor pathname during fresh Ubuntu acceptance (PR #583).

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
Windows restart gate verifies changed namespace/current marker identity, ordinary
Host entry, doctor, network and data retention before an installer rerun. It takes
read-only snapshots around the ordinary WSL stop. Any new dnsmasq archive must
contain the exact pre-stop record bytes; existing archives must stay unchanged.
An unrelated historical archive is not evidence for this restart.

[Incus stop cleanup](https://github.com/lxc/incus/commit/80774bd96b31737acf35b1bf6d1bb8592ba33a67)
now removes the dnsmasq record, while
[process identity](https://github.com/lxc/incus/commit/5472f997327b4c991a821f37ebaf004bc32e6850)
records `pid`, `start_time` and `boot_id`. On supported 7.0 LTS servers at 7.0.2 or
later, no new archive is accepted only with positive pre/post process identity:
canonical current boot UUID, procfs start ticks at or after namespace init,
matching PID namespace, the packaged `/usr/sbin/dnsmasq` executable, and its live
command using the fixed `haco-host0` interface/configuration. The process identity
must change. Bracketed rereads reject changing records, processes, archives,
server versions or markers; failed reads do
not mean absence. Legacy identityless records still require an exact new archive.
No process UID of root is assumed because dnsmasq drops privileges. These checks
prove current safe ownership, not which actor removed the old record. They never
write provider metadata, fabricate archives, signal processes or repair startup.
Component tests retain stale-record/no-signalling coverage and cover marker
advancement after clean provider stop. Repository tests, hosted provider acceptance
and installation on an existing user's machine remain separate claims.

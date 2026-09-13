# ADR 0069: Bounded process streams through the canonical run lifecycle

Status: accepted for the development candidate; native stdin/TTY acceptance pending.

Date: 2026-09-13

## Context

Issue #589 requires pipe input and terminal use in a disposable Environment.
The existing captured-run stream treats socket input or EOF as cancellation.
Reinterpreting those bytes would break the old cancellation contract. A new
transport must preserve cleanup ownership, separate output channels and actual
exit status without giving an ordinary Env management authority.

## Decision

Add the separately negotiated `run.process` method. Its controller adapter uses
the existing run service for creation, recovery and cleanup. Streamed execution
pins the exact ephemeral lease identity under the common lifecycle lock, as in
[ADR 0068](0068-ephemeral-run-creation-ownership.md). The provider router forwards
an optional process contract; Incus options and private PTY handling stay in the
Incus adapter. Other providers may refuse support. Core gains no Incus or client
dependency and no arbitrary Host environment transfer.

The shared control transport frames stdin, stdout, stderr, input EOF, input
credit, input-stop and one opaque result. Data frames and outstanding input credit
are limited to 32 KiB; the final receipt is limited to 64 KiB. Consuming stdin
returns credit. The wire reader remains free to observe disconnect while a process
ignores stdin. No unbounded input queue, whole-input capture or input-stall timeout
is used. Frame writes and final input drainage have a 30-second bound.

After the operation and canonical cleanup return, stop further input consumption
and tell the client to stop sending. The client serializes its final EOF after
any in-flight write. Drain that EOF before sending the final receipt and closing.
An early-exit regression demonstrated that closing a Unix socket with unread
input can reset the client and discard an otherwise valid result. The final
writer seal also forbids late credit/output after completion.

The client accepts the receipt only after frame completion and the separately
negotiated session completion both succeed. Truncated, oversized, unknown,
duplicate or out-of-order frames, excess input credit, bytes after input EOF and
transport loss fail closed. Never retry execution automatically. Input-copy
teardown cannot override a confirmed early process exit; missing confirmation
still reports unknown cleanup.

Process requests contain at most 256 arguments and 32 KiB of argument bytes, no
NULs, and at most 64 KiB of request JSON. Arguments follow the provider's explicit
`--` separator. The run working directory is `/workspace`. TTY mode forwards only
validated terminal metadata and the existing separately scoped resize control;
it reuses private PTY sizing and client terminal restoration. No reusable Host
credentials, control sockets or protected state are passed to the Env.

## User behavior and alternatives

`-i` streams pipe input with separate stdout/stderr. `-t`/`-it` require a real
terminal and enable input; a pipe uses `-i`. The guest PTY combines its output
channels, and typed control bytes belong to the guest. Physical terminal EOF ends
that PTY. Captured execution remains the default and retains `--json`; combining
streaming with `--json` is rejected before creation. Selected Workspace/OCI data
survive cleanup. Global OS locale and presentation-language hints are not Env
execution settings.

Reject raw half-close as the sole EOF signal: it cannot distinguish finished
input from a disconnected owner. Reject a blocking stdin pipe as the wire reader:
it hides disconnection when a process stops reading. Reject name-only execution,
duplicated transport-owned create/delete and a successful exit as proof of cleanup.

Repository regressions cover binary input, separate output, EOF, backpressure,
early exit/reset, malformed frames, failed completion, cancellation cleanup,
generation pinning and literal provider arguments. The maintained disposable
Incus fixture adds real pipe and PTY editing/resize/restore acceptance. Those
checks do not establish Windows terminal acceptance, VPN behavior or
large-repository performance.

Incus execution modes follow the [official execution contract](https://linuxcontainers.org/incus/docs/main/instance-exec/).

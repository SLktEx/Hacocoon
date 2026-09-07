# ADR 0027: Revision-bound Policy editing

Status: accepted; repository implementation, installed acceptance pending.

Ordinary trusted users need to inspect and edit saved approval choices without
entering a Physical Host recovery shell. `haco config` exposes a snapshot of the
existing Policy, including administrator rules and saved decisions. `--edit`
uses the operator's editor; `--file` applies the same snapshot format.

The existing management socket grants administrator authority. Configuration
read/replace is registered only there, never on guest Git sockets or the
presentation-only notification bridge. Policy management is not a workload
Capability and must not become an Environment permission. No new Core provider,
browser dependency, raw management socket projection or credentials are added.

A revision hashes the exact file bytes and distinguishes absence. Replacement
must match that revision while holding the same private advisory lock used by
approval saving. One canonical writer validates the whole document, bounds its
size, rejects unsafe files, writes a private temporary file, synchronizes,
rechecks the observed contents and atomically replaces the file. Cooperating
writers cannot erase each other's updates. Out-of-band administrator editors
must coordinate with the lock; the extra byte check is not a transaction with
an arbitrary process that ignores the lock.

Audit records intent before mutation and completion after persistence. Events
contain operation identity and old/new revisions, never configuration contents.
Pre-write audit failure prevents replacement. Post-write audit failure returns
recovery-required without a successful receipt; the operator must inspect the
current configuration before retrying. A stale edit is retained, never silently
rebased or retried.

Rejected: separate approval/config files, last-writer-wins import, implicit
merging of rules, assuming that a queued update proves persistence, exposing
Policy details through minimized interaction events, and approving config
changes through the very Policy they are replacing.

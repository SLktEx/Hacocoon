# ADR 0059: Start Environments through the guarded lifecycle

Status: accepted
Date: 2026-09-11

## Context

An isolated WSL acceptance run created an Environment, then observed a later
WSL PID-namespace boot with that instance running and its source guard absent.
`haco env start` refused it; ordinary stop/start restored the missing guard while
stopped and then succeeded. The instance had no `boot.autostart` setting.
[Incus 6.0.5](https://github.com/lxc/incus/blob/v6.0.5/cmd/incusd/instances.go)
automatically starts an instance when this setting is true, or unset and its
previous power state was running. Incus cannot prepare Hacocoon's volatile
source-identity guards before that automatic start.

## Decision

Use Incus's `boot.autostart=false` for untrusted Environments. New instances
receive the setting at initialization, before any later configuration step.
The shared new/restored sandbox configuration verifies it before guest start.
The existing guarded resume path sets and verifies it after ownership and network
validation, before starting or accepting an already running Environment.
Configuration failure or a mismatched/truncated readback fails closed.

Incus continues to own instance startup and shutdown. Hacocoon prepares and checks
its security boundary through the existing lifecycle API. No boot coordinator,
automatic Environment reconstruction, rollback backup, new CLI option or Core
state is introduced. Trusted `haco-host` has a separate infrastructure lifecycle.

## Existing installations

Before the next Physical Host/WSL restart, stop and start each retained legacy
Environment using the updated controller: `haco env stop NAME`, followed by
`haco env start NAME`. This preserves its Workspace, OCI Store and generation
while applying the setting. Untouched legacy instances are not silently migrated.
An already running instance with a missing source guard still requires stop/start;
setting the boot option must not bypass that refusal. An incompatible instance
can instead be deleted and recreated through the normal retained-data workflow.

After a host boot, resume work with the existing `haco env start NAME`. A prior
running state alone does not authorize startup before security preparation.

## Validation scope

Provider regressions cover initialization, guarded resume, write failure and
readback mismatch. Real Incus/WSL acceptance must separately verify cold-boot
behavior; a unit-test pass or a config value alone does not prove that acceptance.

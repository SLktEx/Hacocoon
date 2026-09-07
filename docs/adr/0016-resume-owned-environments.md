# ADR 0016: Resume the owned Environment under a lifecycle lock

Status: accepted
Date: 2026-09-07

## Decision

`haco env start <name>` resumes the existing runtime through the canonical
Workspace service. Create, start, stop and delete acquire an Environment-name
lock before inspecting ownership and retain it through provider completion.
The Linux/WSL controller uses cancellable cross-process locks in an owner-only
directory; name locks precede Workspace locks. Non-Linux client tests use keyed
process locks; this does not establish support for a Windows-native controller.

Start requires the active lease to match the persisted Environment's exact
runtime, Workspace, access mode and persistent-resource identity. It never
releases or reconstructs ownership, including after timeout or uncertain start.
The Incus sandbox provider verifies the Environment marker, owned dedicated
bridge, port isolation, MAC/source guard and shared transport firewall before
starting. Unknown states, missing guards and drift fail closed. Verification
after startup must succeed; otherwise bounded cleanup attempts to stop the
instance without releasing its lease. An already running, valid instance is a
successful repeat request.

## Rejected alternatives

- Direct CLI-to-Incus start bypasses provider routing and ownership validation.
- A check followed by an unlocked start can target a replacement after deletion.
- Delete/create for reconnect discards the Environment root filesystem and changes
  identity unnecessarily.
- Starting first and restoring isolation afterwards permits an unguarded interval.

## Limits and validation

This is the C reconnect / E reuse foundation. SSH configuration automation,
post-Host-reboot reconstruction of absent source guards, snapshots and full backup
remain separate work. A missing guard is an error, never an isolation downgrade.
Unit/component regressions cover ownership mismatch, cancellation, concurrent
delete, idempotence and pre/post-start network failures. The maintained Incus E2E
also exercises product stop/start through the trusted Host and retained Workspace
content; adding that test does not itself establish a real-host run.

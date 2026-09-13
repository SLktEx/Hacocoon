# ADR 0064: Keep WSL login bootstrap out of automatic Host entry

Status: accepted
Date: 2026-09-13

## Context

Native Windows restart acceptance at `8c645317` reproduced `Host setup is busy`:
initial install and entry passed, but entry after termination failed. WSL starts
`/bin/login -f` on a private PTY to initialize a systemd user session, separately
from the user shell launched by `/init`. Both shells satisfy an isatty check.
See [WSL's implementation](https://github.com/microsoft/WSL/blob/eaa69e766cf375d96053207a4ba8858f54ea1536/src/linux/init/config.cpp#L2726)
and [our failure evidence](../status/acceptance-evidence.md#ci-reliability).

## Decision

The Linux login alias checks its immediate parent command before automatic
Host entry. A `login`-managed shell starts ordinary login Bash on the Physical
Host. The real WSL interactive shell retains controller-backed trusted Host
entry. Explicit shell arguments and nonterminal calls retain their existing
Physical Host behavior. Failure to inspect the parent returns an error.

Parent command identity selects the user interface; it is not authorization.
It can be imitated by a caller without granting authority: direct Physical Host
shell entry already exists, Bash retains the caller's identity, and controller
peer authorization is unchanged. Untrusted Environment processes gain neither
Host credentials nor management sockets from this decision.

## Rejected alternatives

- TTY presence alone also selects WSL's background bootstrap PTY.
- Sleeping before entry or retrying rejected Host setup hides the race and can
  repeat a mutation whose completion is unknown.
- Relaxing setup exclusion permits overlapping provisioning/customization.
- A CI override or service pre-start would make the test differ from cold user
  entry and leave the product defect in place.

## Verification

A Linux child-process regression gives a login-named parent a real PTY and
requires Bash completion without any controller. Separate parent identities
exercise ordinary entry selection. This does not emulate PAM or establish native
WSL acceptance. The unchanged packaged Windows terminate/ordinary-entry journey
must verify the fix; its earlier failure remains recorded.

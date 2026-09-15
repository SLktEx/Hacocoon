# Fixed local WSL process transport

Status: accepted; partial implementation candidate. [日本語](0092-wsl-process-transport.ja.md)

## Decision

The Windows client selects a validated local distribution and calls a fixed
installed product operation through hidden System32 wsl.exe. Notification and
control launches share the argument/environment constructor. No command text,
WSL user override, inherited credential environment or caller-selected socket
passes this boundary. The Linux bridge retains ordinary UDS authorization;
being on the same PC is not authorization. Normal Envs receive no new endpoint.

Anonymous pipes need explicit application EOF to preserve TCP half-close.
Bounded framing carries data and EOF independently from child death. Raw pipe
closure is transport failure. Separate control connections retain the existing
session completion/cancel protocol and controller-owned target identity checks.
A child is reaped on cancellation or connection close, and explicit pipe ownership
allows unread stdout to drain after normal child exit. Deadlines fail closed.

## Rejected alternatives and remaining work

A TCP management daemon is unnecessary for same-PC Windows/WSL. Plain stdin EOF
as the only control cannot distinguish half-close from process loss. Shell
command strings, PATH-selected WSL executables and root fallback widen the
launch boundary. The installer helper and notification helper keep their existing
responsibilities; a public Windows listener needs its own client integration.

Public companion packaging and automatic WSL entry delegation are implementation
candidates; [ADR 0093](0093-windows-tunnel-delegation.md) owns the latter. Installed Windows acceptance remains pending. Native
fixture transport success is not proof of installed controller or Incus access.
See the [owning contract](../design/controller-client-transport.md#windows-process-transport).

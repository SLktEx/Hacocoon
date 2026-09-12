# ADR 0058: Give SSH sessions the current managed egress environment

> Implementation/acceptance statements below describe the stage when this decision was recorded. See the [current contract and scope](../status/acceptance-evidence.md#development) for subsequent implementation and remaining limits. The decision and rejected alternatives are retained.

Status: accepted; real installed acceptance pending.

Incus environment.* settings reach Incus exec processes. They do not configure
new OpenSSH sessions. Installed transfer acceptance at 7517c27 prepared source SSH
successfully but package installation over SSH failed with exit 100. The code had
no SSH session proxy configuration. Moving package work out of SSH would leave
the ordinary development path incomplete.

During existing SSH preparation, the Incus adapter writes an OpenSSH SetEnv
drop-in containing only the six HTTP(S)/NO_PROXY variants already used by the
managed sandbox profile. Values come from current controller-owned endpoint
constants, never from the caller environment or saved guest configuration.
Validate sshd configuration, reload the service, and only then finish publishing
the connection. Failure removes the new loopback proxy through existing cleanup.

OpenSSH SetEnv applies to interactive and command sessions. It grants no network
permission: source identification, current generation, Policy, approval and direct
egress refusal remain authoritative. No credentials, management socket, arbitrary
Host variables, client AcceptEnv expansion or general environment-copy facility
is introduced. Imported rootfs receives current settings on SSH preparation.

The drop-in belongs to disposable Env configuration. There is no catalog change,
backup or new recovery state. Existing Workspace/OCI/snapshot protection is unchanged.

See [OpenSSH SetEnv](https://man.openbsd.org/sshd_config#SetEnv) and
[Windows SSH](../reference/windows-environment-ssh.md).
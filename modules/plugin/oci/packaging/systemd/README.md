# OCI plugin systemd packaging

These unit templates belong to the optional Hacocoon OCI plugin. Hacocoon Core does not install, enable, or require Docker, nerdctl, containerd, or these units.

## Docker compatibility units

`hacocoon-docker.socket` and `hacocoon-docker.service` implement an optional Docker Engine compatibility profile for Environments whose Base/Seed explicitly includes it.

The profile assumes the guest image already provides:

- a standalone `/usr/bin/containerd` with `containerd.service`;
- the genuine Docker CLI;
- `/usr/bin/dockerd` when Engine API compatibility is requested;
- a `docker` group for users explicitly allowed to access the Environment-local Docker API.

A nerdctl-oriented OCI plugin profile may additionally install `/usr/bin/nerdctl`; that is a plugin/profile choice, not a Core requirement.

Base/Seed provisioning for the Docker compatibility profile should install these units after package installation, disable vendor `docker.service` / `docker.socket` auto-start, and enable only `hacocoon-docker.socket`.

Do not enable `hacocoon-docker.service` at boot. Do not use these units to expose or proxy a Host Docker socket.

See [`../../../../../docs/OCI_RUNTIME_AND_DOCKER_COMPAT.md`](../../../../../docs/OCI_RUNTIME_AND_DOCKER_COMPAT.md).

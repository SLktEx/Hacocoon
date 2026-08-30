# Hacocoon systemd packaging

This directory contains unit templates intended to be installed into immutable Hacocoon development Bases/Seeds.

## Docker compatibility units

`hacocoon-docker.socket` and `hacocoon-docker.service` implement the Docker Engine compatibility path described in [`../../docs/OCI_RUNTIME_AND_DOCKER_COMPAT.md`](../../docs/OCI_RUNTIME_AND_DOCKER_COMPAT.md).

They assume the guest image already provides:

- `/usr/bin/containerd` with `containerd.service`;
- `/usr/bin/nerdctl`;
- the genuine Docker CLI;
- `/usr/bin/dockerd` when Engine API compatibility is enabled;
- a `docker` group for users explicitly allowed to access the Environment-local Docker API.

Base/Seed provisioning should install these units after package installation, disable vendor `docker.service` / `docker.socket` auto-start, and enable only `hacocoon-docker.socket`.

Do not enable `hacocoon-docker.service` at boot. Do not use these units to expose or proxy a Host Docker socket.

The current repository does not yet contain the Base/Seed publisher that performs this bake-in automatically; these files are packaging inputs for that path.

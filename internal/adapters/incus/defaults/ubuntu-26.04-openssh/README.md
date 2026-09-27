# Ubuntu 26.04 + OpenSSH Base

This managed build context creates `ubuntu-26.04-openssh` from `haco/ubuntu-26.04` and ensures `openssh-server`, the full `nerdctl` v2.3.5 distribution, and Docker 28.5.2 are installed.

The `nerdctl-full` archive provides the bundled container runtime and build tooling, including `nerdctl`, `containerd`, `runc`, BuildKit, and CNI plugins. Docker is installed from the official static Linux archive with its CLI at `/usr/local/bin/docker` and its engine-side dependencies isolated under `/usr/local/lib/hacocoon/docker`, so they do not replace the `nerdctl-full` containerd/runc binaries.

Both upstream archives are installed with pinned SHA-256 digests for amd64 and arm64. The containerd, BuildKit, and Docker systemd services are enabled for subsequent boots. Docker uses the `vfs` storage driver so the default Base remains usable in Hacocoon's nested unprivileged Environment model.

From trusted `haco-host`, run:

```bash
~/base-builds/ubuntu-26.04-openssh/build.sh
```

Then inspect the published Base with:

```bash
haco base inspect ubuntu-26.04-openssh
```

After creating an Environment from the Base, the expected tooling includes:

```bash
ssh --version
nerdctl --version
docker --version
```

`haco setup` reconciles this directory, so copy it elsewhere before making local customizations.

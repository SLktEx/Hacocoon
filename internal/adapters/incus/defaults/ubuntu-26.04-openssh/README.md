# Ubuntu 26.04 + OpenSSH Base

This managed build context creates `ubuntu-26.04-openssh` from `haco/ubuntu-26.04` and ensures `openssh-server` plus the full `nerdctl` v2.3.5 distribution are installed.

The `nerdctl-full` archive provides the bundled container runtime and build tooling, including `nerdctl`, `containerd`, `runc`, BuildKit, and CNI plugins. The official upstream archive is installed with a pinned SHA-256 digest for amd64 and arm64, and the containerd and BuildKit systemd services are enabled for subsequent boots.

From trusted `haco-host`, run:

```bash
~/base-builds/ubuntu-26.04-openssh/build.sh
```

Then inspect the published Base with:

```bash
haco base inspect ubuntu-26.04-openssh
```

`haco setup` reconciles this directory, so copy it elsewhere before making local customizations.

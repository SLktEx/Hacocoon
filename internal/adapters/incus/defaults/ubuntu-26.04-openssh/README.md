# Ubuntu 26.04 + OpenSSH Base

This managed build context creates `ubuntu-26.04-openssh` from `haco/ubuntu-26.04` and ensures `openssh-server` plus `nerdctl` v2.3.5 are installed.

The `nerdctl` binary is installed from the official upstream release archive with a pinned SHA-256 digest for amd64 and arm64.

From trusted `haco-host`, run:

```bash
~/base-builds/ubuntu-26.04-openssh/build.sh
```

Then inspect the published Base with:

```bash
haco base inspect ubuntu-26.04-openssh
```

`haco setup` reconciles this directory, so copy it elsewhere before making local customizations.

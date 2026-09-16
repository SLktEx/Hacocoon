# Ubuntu 26.04 + OpenSSH Base

This managed build context creates `ubuntu-26.04-openssh` from `haco/ubuntu-26.04` and ensures `openssh-server` is installed.

From trusted `haco-host`, run:

```bash
~/base-builds/ubuntu-26.04-openssh/build.sh
```

Then inspect the published Base with:

```bash
haco base inspect ubuntu-26.04-openssh
```

`haco setup` reconciles this directory, so copy it elsewhere before making local customizations.

# Installer architecture

Hacocoon supports Ubuntu only for the Linux Physical Host. Windows uses a dedicated Ubuntu 26.04 WSL 2 distribution.

## Phase boundary

Installation is intentionally split into environment-specific `pre`, shared `main`, and environment-specific `post` phases.

```text
Windows / WSL
install-windows.ps1 pre
        |
        v
     install.sh
        |
        v
install-windows.ps1 post

Native Ubuntu
install-ubuntu.sh pre
        |
        v
     install.sh
        |
        v
install-ubuntu.sh post
```

`install.sh` contains only the work common to Ubuntu running natively and Ubuntu running under WSL: shared dependencies, Incus, Hacocoon binaries, the Physical Host controller, trusted `haco-host` reconciliation, and the controller round trip.

WSL lifecycle and login integration stay in PowerShell. Native-Ubuntu-only checks and post-install behavior stay in `install-ubuntu.sh`.

The shared phase installs bundled `incus-boot-guard.py` using isolated Python
and an Incus service drop-in. First adoption requires the existing daemon to be
ready. Subsequent namespace boots archive stale network/proxy PID records before
Incus starts; same-namespace restarts retain them. See
[ADR 0013](adr/0013-incus-pid-record-boot-identity.md).

## Architecture-specific installer bundles

Release packaging produces separate amd64 and arm64 bundles. Each installer carries exactly one matching Linux release archive.

```text
hacocoon-windows-amd64.zip
  install-windows.bat
  install-windows.ps1
  install.sh
  incus-boot-guard.py
  haco_linux_amd64.tar.gz
  checksums.txt
  VERSION

hacocoon-windows-arm64.zip
  ...
  haco_linux_arm64.tar.gz

hacocoon-ubuntu-amd64.tar.gz
  install-ubuntu.sh
  install.sh
  incus-boot-guard.py
  haco_linux_amd64.tar.gz
  checksums.txt
  VERSION

hacocoon-ubuntu-arm64.tar.gz
  ...
  haco_linux_arm64.tar.gz
```

The normal installer path consumes the bundled architecture-matched archive. It does not download the Hacocoon binary archive a second time.

## E2E boundary

Installer E2E does not test GitHub Release publication or downloading from the Release page. Those are release-pipeline responsibilities.

For each supported user path, E2E:

1. compiles the candidate commit with the same pinned GoReleaser configuration used by the release workflow;
2. packages the result with the same installer packager used by the release workflow;
3. extracts the locally-built release-equivalent installer bundle;
4. starts from the user-facing installer entry point;
5. runs through installation; and
6. verifies that Hacocoon is actually usable, including `haco doctor` and the trusted `haco-host` controller round trip.

The locally-built E2E candidate is not a published release and therefore has no publication attestation. Provenance/publication checks remain a separate release gate; installer E2E is responsible for the install-to-usable-Hacocoon path.

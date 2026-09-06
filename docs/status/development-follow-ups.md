# Development follow-ups

Status: current residual work from the second-stage PoC. These are local issue
notes, not additional acceptance requirements for B1–B3/B5/B6.

- **Required B4 acceptance, blocked:** approve runtime installation and
  `security.nesting=true` only for owned `haco-host` and test `b-dev`. Prepare a
  small explicitly identified image, run normal one-way distribution and
  guest start/change/stop, and prove Host container state remains independent.
  Record Docker and nerdctl separately. Automatic execution review refused
  the setup and a possible transfer of an unverified image; neither ran.
- **Windows integration:** existing [#275](https://github.com/SLktEx/Hacocoon/issues/275).
  Only C was available. Additional real drives, detach/reconnect, WSL socket
  changes and broad Windows executable compatibility remain unverified.
- **SSH package proxy convenience:** existing [#469](https://github.com/SLktEx/Hacocoon/issues/469).
  Generated SSH configuration reduces transcription; automatic proxy export
  and broader IDE launch remain future improvements.
- **Git interruption/results:** existing [#470](https://github.com/SLktEx/Hacocoon/issues/470).
  Interrupted approvals, ambiguous external push completion, large packs,
  additional auth methods, LFS, submodules and multiple refs remain follow-up.
- **Collections/Base lifecycle:** membership editing, interrupted preparation,
  concurrent switch/acquisition and general recovery remain unverified.
  Base switching intentionally recreates rootfs and SSH host keys while keeping
  repository data; automatic package restoration is not implemented.
- **OCI lifecycle:** crash-left private archives, broad image compatibility,
  automatic updates, running state/volume migration and large image performance
  remain unverified. No container live state or Host credentials are copied.
- **Distribution acceptance:** branch BAT application passed on the existing
  WSL installation. Fresh reinstall, upgrade interruptions, wider Windows/WSL
  matrices, forwarding/release-config full jobs and comprehensive E2E/performance
  remain outside the checks recorded here. Open CI-only PRs #462/#464/#465
  were inspected initially and left untouched.

Implementation and observed commit-bound results are owned by
[implementation status](../IMPLEMENTATION_STATUS.md). C is not implemented by
this work; macOS remains entirely out of scope.

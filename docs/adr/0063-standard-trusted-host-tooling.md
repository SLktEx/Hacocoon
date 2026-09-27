# ADR 0063: Standard tools in the trusted Host

Status: accepted
Date: 2026-09-13
Amended: 2026-09-27

## Decision

The maintained local composition provisions Git/gh, pinned rootful
containerd/nerdctl/BuildKit and a pinned Docker Engine/CLI after canonical Host
storage preparation and nesting verification, before user customization. Core
gains no executable requirement or tool installation API. The same composition
serves native Ubuntu and Windows/WSL.
The [trusted Host contract](../design/trusted-host.md#standard-host-tools) owns
sources, versions, commands and retry behavior.

Use the existing managed Host OCI area and independent Store copy implementation.
Native snapshots and containerd's transfer unpack configuration must agree, so an
ordinary pull works without special flags. BuildKit cache and Docker's managed
data root remain inside that area; process state and sockets remain Host-local.
Docker's official static archive is digest-pinned per architecture, its engine
dependencies are isolated from the standard containerd binaries, and Hacocoon does
not force a Docker storage driver; the daemon selects a supported driver at startup. The
receiving Environment supplies its own runtime. Registry credentials stay outside
the copied area and the existing authenticated-registry credential lifecycle is
unchanged.

Provisioning holds the existing Host operation lock and revalidates the exact
ready source, sole attachment, locally owned unprivileged container and nesting.
Fixed guest systemd units bound execution and overlap after controller loss.
Existing copy journals still block setup; copying still pauses all Host writers.
Each file is published atomically after authenticated archive verification and
selected-entry validation. Repeat setup preserves installed files and OCI data;
foreign configuration and unknown binaries fail closed. Failed installation is
retried explicitly and never causes resource/lease release.

## Rejected alternatives

- Per-user recipes for required tools make fresh setup incomplete and give common
  provisioning no stable failure/retry contract.
- Installing Docker from distro packages or sharing the standard containerd
  executable set would couple unrelated upgrade/service lifecycles. The managed
  Host instead uses a separately pinned official static Docker bundle while keeping
  Docker optional to Core and ordinary Environment runtime selection.
- Installing the full upstream archive blindly accepts unrelated executable code,
  links and service definitions. Only fixed regular payloads are published.
- Sharing Host sockets or credentials with an Environment grants Host authority.
  Independent area copies continue to contain data only.
- Image export/import would replace the existing area-level CoW contract and lose
  runtime metadata/cache; installing tools is not a reason to change that contract.

## Verification

Component tests exercise foreign ownership, unsupported isolation, pending copies,
malformed archives, links, digest failures, repeat publication and bounded failures.
The dedicated standard-tooling Incus fixture checks public nerdctl/Docker
pull/build/run, stop/start/repeat setup, retained runtime data and offline
independent Store use. Released installer, private registry, arm64 runtime and
custom existing-installation acceptance must be reported separately; repository
support is not evidence those checks ran.

The main integration removes the obsolete preliminary Git/gh apt helper (#659).
Only canonical Standard tooling preparation runs after OCI/client readiness.
Duplicated provisioning can fail before that owner's validation and recovery
contract is reached; it must not be reintroduced as a compatibility path.

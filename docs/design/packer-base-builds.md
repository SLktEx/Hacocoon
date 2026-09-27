# Packer Base builds

[日本語](packer-base-builds.ja.md) | English

Status: implemented. The nested Incus implementation replaces the historical
ordinary-Environment Packer adapter. The real Incus E2E passed on a dedicated WSL fixture; hosted CI and TB-scale
images remain unverified (amd64 acceptance scope). Evidence is recorded separately in
[acceptance evidence](../status/acceptance-evidence.md#nested-packer).

## Build a reusable tool

Run in trusted `haco-host` after normal `haco setup`:

```bash
haco image build --name my-tools examples/packer
haco image inspect my-tools
haco open --new my-tools --name dev --client none
haco exec dev my-tool
```

The tool prints `hello-from-packer`. Options precede the directory.
The [example](../../examples/packer/base.pkr.hcl) is standard HCL2 using the real
[Incus plugin v1.0.5](https://github.com/bketelsen/packer-plugin-incus/tree/v1.0.5)
and an external `setup.sh`. Its documented shell `remote_folder` is `/root` to
avoid boot-time `/tmp` mount/cleanup races. Packer is pinned to 1.16.0 and the plugin constraint
is exactly `= 1.0.5`. `image`, `output_image`, `container_name`,
`launch_config`, and `publish_properties` follow that upstream version.

Packer, plugins, `shell-local` and local post-processors execute in trusted
`haco-host` with normal Packer semantics. HCL and plugins are trusted build code.
Provisioners run in a separate nested Incus instance. Neither that instance nor
an ordinary Environment receives Physical Host Incus sockets, state or credentials.
The Physical Host never evaluates HCL or runs plugins.

## Dependencies and source files

Normal Host setup installs the checksum-pinned amd64/arm64 Packer distribution.
It uses the shared signed Incus LTS installer inside `haco-host`: 7.0.x,
minimum 7.0.1, with ordinary patch updates and no automatic downgrade from newer
series. Existing incompatible or foreign nested storage is refused without deletion.
Python, package tooling and Incus are Host tooling, not build-target prerequisites.

The nested daemon is local-only and uses its own `/var/lib/incus`. Setup creates
an owned directory storage pool `haco-packer` and NAT bridge `haco-packer0`;
both survive builds and repeated setup. Existing trusted-Host
`security.nesting=true` is reused without privileged mode or Physical Host mounts.
Host setup also installs `nftables` for nested networking. The build profile sets `security.idmap.size=65536` to bound the UID/GID range delegated to its unprivileged nested instance.
Image compression is `none` for the existing uncompressed archive import contract.
Each build creates an exclusive, randomly identified Incus project with private
images/profiles and an ownership property.

The CLI stages at most 128 regular files totaling 512 KiB; at least one root
`.pkr.hcl` is required. Hidden entries are skipped; unsafe paths, links, special
files and oversized contexts are rejected. These trusted scripts run on the Host,
so do not put credentials in build contexts. Linux/WSL is the supported entry.

The template receives `HACO_PACKER_BUILD_ID`. It must write that value to the
output image property `user.hacocoon.packer-build`; exactly one private container
image must match. Packer variables retain ordinary HCL/`auto.pkrvars.hcl` semantics.
Choose the source image in HCL. `--from` and `--builder` belong to JSON definitions.

There is no configured image-size cap or overall deadline by default. Use `--max-image-size 2TiB` to impose an optional cap, or `--max-image-size unlimited` to state the default explicitly; `haco image import` accepts the same option. Explicit caps apply to export, upload and controller validation. Signed file-offset representation and filesystem/storage capacity still apply. Transfers use bounded memory and do not load the image all at once. The import Env root disk quota scales to twice the artifact size with a 64 GiB minimum; if that exceeds Core's finite quota representation, only the disk uses its existing unlimited mode. CPU, memory and PID budgets remain finite. Disk-full errors retain the existing failure/recovery semantics. TB-scale image duration and disk consumption remain unverified.

## Artifact and publication

`fmt -> init -> validate -> build` runs in a bounded systemd service/cgroup in
`haco-host`. The worker verifies the nested project owner and image fingerprint,
exports the native unified container image, and records its size, SHA-256 and CPU
architecture. Packer's exit status is insufficient: the worker positively checks
instance absence, then deletes exact image fingerprints in its owned project and
removes that project and temporary context before starting import.

The CLI uploads the artifact through the existing bounded `base.import` stream.
The controller checks the operation ID, digest, size and supported local CPU,
captures the complete input, and reuses native archive validation, ownership,
resource limits and the canonical Base import/publication lifecycle. The temporary
ordinary import Env is not a Packer runner. The controller consumes only an image.

The existing immutable revision and alias contract is unchanged. Old Environments
keep their pinned revision; a successful rebuild selects a new revision only for
future Environments. JSON definitions remain a separate build path.
See [Base archive import](base-images-and-custom-environments.md#import-a-container-image-archive).

## Results and failure

Each attempt has a durable private receipt under
`/var/lib/hacocoon-packer/builds/<build-id>/receipt.json`. It records the exact
Base name, nested project, image fingerprint, artifact digest/size, stage and controller
import builder `build-<build-id>`. Native publication records this builder name as
`build_environment` in `haco image list --all --json`, retaining correlation even when a
completed import loses its reply and removes its temporary Env. This is diagnostic
metadata; deletion still requires the existing fingerprint and immutable build-instance identity. `--json` reports state and retained identity.
No source, subprocess output or credentials enter controller logs.

Invalid HCL, initialization/plugin/provisioner failure, cancellation, export
failure or nested cleanup failure never starts import. Unknown mutation results
retain the receipt and resources as recovery-required. Interrupted upload or an
unconfirmed import reply retains the artifact and exact controller identity;
there is no automatic retry, alias rollback or guessed resource deletion.

After confirmed import, transport cleanup removes only the exact operation's
artifact and receipt. As with existing archive import, a complete published Base
survives a subsequent cleanup failure; that outcome is reported as
recovery-required, never success. Native publication uncertainty retains the
canonical image/build evidence. A lost acknowledgement cannot prove the pointer
did not move; inspect the receipt and native image rather than replaying import.

Packer stages and image export have no wrapper deadline. The worker cgroup kills
descendants on exit; a canceled CLI requests stop of its exact service. Bounded
management, stalled-I/O and cleanup waits remain. If the caller is killed abruptly, the worker
may continue: inspect the exact receipt and service, and stop that service if
needed. Durable evidence does not authorize speculative resource deletion.

[ADR 0114](../adr/0114-trusted-host-nested-packer.md) supersedes ADR 0089.

# Guest-only Packer provisioning

Status: accepted; implementation candidate. [日本語](0089-guest-packer-provisioning.ja.md)

## Decision

Evaluate actual Packer HCL2 and execute every plugin/provisioner/post-processor
inside the ordinary, canonically owned temporary builder Env. The optional Packer
adapter receives only a lease-bound execution callback. Core gains no Packer SDK,
HCL interpreter or plugin-management responsibility. Composition selects the adapter.

Packer's built-in null builder reaches that same Env over loopback SSH using
guest-created ephemeral keys. Download and dependency setup retain ordinary Env
network policy. The CLI sends bounded regular-file bytes over the private control
channel, never a Host mount. No private Workspace/OCI data, reusable Host secret,
Host control socket or Incus management authority is exposed. Guest root can
change its own output and forge its own success; it cannot turn that into Host
authority. The Base remains untrusted on later ordinary creates.

Hacocoon owns creation, exact identity/lease persistence, stopping, publication
and cleanup through the existing Base service. Packer owns only guest provisioning.
Native Incus image ownership and alias verification remain authoritative; no
second image catalog is created. Context and ephemeral credentials are removed
before publishing. Cleanup/publication ambiguity keeps existing recovery semantics.

## Rejected alternatives

Running HCL on the privileged Host exposes Host authority through `shell-local`,
plugins and post-processors even if the main shell provisioner targets a guest.
Giving the guest an Incus socket or reusable credential breaks the same boundary.
The existing community Incus Packer builder directly creates and publishes
instances; using it unchanged would bypass Hacocoon's canonical ownership.
Translating HCL into the historical JSON `run` field would lose actual Packer
semantics. A new Host builder/catalog duplicates lifecycle decisions unnecessarily.

This decision covers same-PC Linux/WSL development, not AMI/QEMU/cloud portability.
See the [command, input and failure contract](../design/packer-base-builds.md).

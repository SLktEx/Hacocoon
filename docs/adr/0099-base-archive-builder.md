# ADR 0099: Import Base archives through the temporary builder lifecycle

Status: accepted
Date: 2026-09-15

## Decision

Capture a bounded uncompressed native container image upload in the existing
controller-owned unnamed staging mechanism, shared with Env transfer. Incus alone
validates and interprets native image contents and replaces image metadata with
fresh temporary transport ownership. Create an independent temporary Env through
canonical archive creation, preserving exact ownership before configuration.
Use the same guest-local Workspace, finite resource limits, current network guards
and credential boundaries as a definition/Packer builder. Reuse the existing
cleanup, stop, immutable image publication and atomic Base pointer update.

## Rejected alternatives

Promoting the temporary transport image violates its cleanup and ownership contract.
Host extraction or build script execution gives untrusted image content Host authority.
A special privileged importer, loose network policy, Host mounts or reused secrets
would bypass ordinary Env isolation. A second builder/cleanup journal would duplicate
canonical lifecycle ownership and native publication receipts. An archive cannot
supply devices, profiles, policies, credentials or a controller-side file path.

## Consequences

The source is retained and every import gets independent storage ownership. Unknown
creation or publication retains recovery evidence; it is not automatically retried.
Existing Env revisions do not follow logical alias changes. Input templates are not
replayed. The initial contract supports uncompressed unified Incus container images,
not Env bundles, VM images, compression or arbitrary external acquisition. Private
staging and ordinary Env transfer retain their existing tests and behavior.

See [the Base contract](../design/base-images-and-custom-environments.md#import-a-container-image-archive).

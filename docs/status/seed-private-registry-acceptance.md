# Seed private-registry acceptance

Status: **historical acceptance of the retired Seed path**. The Seed acquisition implementation, fixture and manual workflow job are removed. The earlier Host-owned Basic-auth result below is retained; it is not current persistent Store acceptance.

## What is proven

The manual `authenticated-private-registry` job in the `incus-core-e2e` workflow runs the production Incus Seed acquisition path against a real containerd daemon and nerdctl. The test starts an authenticated OCI Distribution-compatible endpoint on loopback, publishes an OCI manifest/config/layer set with immutable SHA-256 identities, and asks `SandboxProvider.exportSeedImages` to acquire the exact `reference@sha256:...` identity into the trusted Host `hacocoon-seed` namespace.

The acceptance requires all of the following:

- a Docker-compatible Host credential config containing the correct Basic-auth credential allows the exact immutable image to be acquired;
- the exact digest remains inspectable in the trusted Host Seed namespace after acquisition;
- the exported Seed archive does not contain the username/password credential sentinel;
- an invalid Host credential fails acquisition instead of falling back to guest egress or unauthenticated access;
- the test uses the same `exportSeedImages` path used by Seed construction rather than a separate acceptance-only pull implementation.

The successful reference run used Ubuntu 24.04, the runner-provided containerd service, and nerdctl 2.3.5 pinned by release-asset SHA-256.

## Transport scope

The acceptance registry is loopback HTTP because nerdctl treats loopback registries as local/insecure endpoints. This test proves the Host-owned authentication and immutable-identity boundary; it does **not** claim to validate a production registry's TLS PKI or custom CA configuration. Production registry transport trust remains an operator/containerd/nerdctl configuration concern.

## Historical replay and remaining scope

The removed fixture and manual job remain in Git history at `aaa4aa5760cd48aec5a4fc26cbdfd612119bdc82`.
They are not current execution instructions. Normal PR runs skipped this manual-only
scenario; removing the retired job does not turn those skips into passes.

The original evidence covered Host-owned Basic-auth acquisition only. Complete
Seed/CoW and failure-injection acceptance was never established by this fixture.
Current persistent Store credential compatibility needs its own product-path
acceptance; old-data evacuation/restore/comparison remains incomplete.

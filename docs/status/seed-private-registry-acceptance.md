# Seed private-registry acceptance

Status: accepted on a real GitHub-hosted Ubuntu runner for the Host-owned Basic-auth credential path.

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

## Re-run

Use the `incus-core-e2e` workflow through `workflow_dispatch`; the `authenticated-private-registry` job runs only for that manual trigger. It requires no repository secret because the registry and one-time credentials are generated inside the isolated runner.

The Go acceptance test is additionally gated by `HACO_E2E_PRIVATE_REGISTRY=1` so normal unit/PR CI does not silently claim a real containerd acceptance run.

## Remaining v0.17 acceptance

This acceptance covers the Host-owned authenticated registry path only. Real Incus Seed end-to-end acceptance, physical Btrfs COW measurement, and real-host failure injection remain tracked separately.

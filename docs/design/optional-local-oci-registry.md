# Optional Local OCI Registry

Status: **deferred optional infrastructure; not a roadmap milestone and not required for normal OCI pulls or Seed construction.**

Hacocoon does not require every Environment-side `nerdctl pull` to traverse a Hacocoon-managed registry. The default direction is normal configured upstream pulls plus trusted Host-side cache/Seed acquisition.

## Default behavior

```text
Environment -> nerdctl pull -> configured upstream registry
```

Docker Hub, GHCR, private registries, and other configured upstreams remain reachable only according to Hacocoon network policy.

## Authentication boundary

- Host credentials must not be silently copied into coding Environments.
- Environment-owned credentials may use normal nerdctl/Docker-compatible credential configuration.
- A credential-broker plugin may provide scoped/short-lived credentials without requiring a Local Registry.
- OCI Seed acquisition uses trusted Host-side credentials independently of Environment credentials.

## When a registry may still be useful

A future operator may choose a Local Registry/proxy when measurements or policy justify it, for example:

- many independent Environments repeatedly pull the same non-seeded images;
- upstream rate/bandwidth limits matter;
- a centralized OCI policy/audit point is required;
- Internet access is intentionally restricted while an internal distribution endpoint is allowed.

This does **not** reserve a product milestone. If a future implementation becomes an independently useful Hacocoon feature, it can take the then-current next minor version.

## Requirements if enabled later

- use an existing OCI Distribution-compatible implementation unless there is a strong reason not to;
- do not expose it publicly by default;
- keep reusable upstream credentials on the trusted side when proxying authenticated registries;
- when mediation is mandatory, registry failure must not silently become unrestricted direct-Internet fallback;
- do not grant arbitrary push authority into shared names by default;
- treat tags as mutable and pin digests where immutable identity matters;
- keep GC conservative when references cannot be proven unused.

## Relationship to OCI Seed

OCI Seed does not depend on a Local Registry. The v0.17 Seed Builder remains offline and receives OCI content from the trusted Host rather than pulling from a registry itself.

# v0.13 — Managed Sandbox Network

Status: **implemented on `main`; real Incus networking acceptance remains host-dependent.**

v0.13 gives Incus-backed Hacocoon Environments a Hacocoon-owned sandbox networking substrate instead of silently inheriting the Incus `default` profile.

## Implemented behavior

- Hacocoon creates/verifies the managed `haco-sandbox0` bridge.
- Hacocoon creates/verifies the `haco-sandbox-egress` default-deny ACL substrate.
- Hacocoon creates/verifies the `haco-sandbox` Incus profile.
- New local sandbox Environments use `haco-sandbox` by default.
- Profile/network/ACL drift must fail closed instead of silently falling back to broader networking.
- Anti-spoofing and port-isolation settings stay adapter-owned.
- Root-disk handling remains compatible with the v0.12 resource-budget path.

## Boundary

This milestone provides the network substrate only. Incus ACLs are IP/CIDR/address-set based; Hacocoon does not pretend they provide domain-name authorization.

Domain-aware allow/ask policy belongs in a higher-layer proxy/broker/plugin and must not be faked with stale DNS-to-IP assumptions in Core.

## Security requirements

- no silent fallback to the Incus `default` profile;
- managed objects are verified before use;
- drift is an error, not a request to weaken isolation;
- coding Environments receive no Incus/Hacocoon control-plane authority;
- higher-level egress authorization remains explicit and separate.

## Acceptance

Repository unit/static coverage verifies creation, selection, and drift rejection. Real supported-Incus acceptance must separately verify bridge/profile/ACL behavior and effective isolation.

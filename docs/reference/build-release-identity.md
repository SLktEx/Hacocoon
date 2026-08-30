# Build, checkpoint, release, and support identity

[**日本語**](build-release-identity.ja.md) | English

Hacocoon deliberately separates development progression from published software identity and from evidence that a Host/backend combination is supported.

## Development checkpoint

A **development checkpoint** is the fast-moving pre-1.0 `v0.N` sequence maintained in [`../status/versioning-and-release-status.md`](../status/versioning-and-release-status.md).

It answers: **what coherent product / implementation / operator / observability / acceptance slice has landed on `main`?**

A checkpoint is not:

- a published Git tag;
- a GitHub Release;
- a compatibility guarantee;
- proof that every older host-dependent acceptance item is complete.

Checkpoint numbers may therefore advance much faster than software releases.

## Software version / release tag

The **software version** identifies a built or published artifact.

- Local or ordinary `go build` binaries report `version: dev` unless a release build injects another value.
- GoReleaser injects its software version, commit SHA, and build date into `haco` with linker flags.
- An official GitHub Release is authorized and published according to [`../RELEASE_SECURITY.md`](../RELEASE_SECURITY.md).

A release tag such as `v0.8.0` does not imply that the development checkpoint is `v0.8`, and a development checkpoint such as `v0.26` does not imply that a `v0.26.0` release exists.

## Acceptance / support status

**Acceptance/support status** is evidence for a concrete execution boundary such as a Host baseline, Incus behavior, storage path, WSL flow, or client environment.

Use [`../IMPLEMENTATION_STATUS.md`](../IMPLEMENTATION_STATUS.md) for current repository reality and named acceptance gaps. A checkpoint may be implemented while some real-host acceptance remains explicitly host-dependent.

## Runtime identity

`haco version` reports these identities separately:

```text
Hacocoon
  checkpoint: <development checkpoint>
  version: <software/release version or dev>
  commit: <source commit or unknown>
  built: <release build timestamp or unknown>
```

For tooling:

```bash
haco version --json
```

For a compact human-readable string that does not initialize Incus or Host state:

```bash
haco --version
```

The checkpoint compiled into `haco` comes from a generated build input synchronized with the authoritative checkpoint documents by `tools/bump-milestone`; it is not a release SemVer constant.

## Advancing a checkpoint

Use:

```bash
tools/bump-milestone v0.N "Gate Name"
```

The helper requires exactly the next `v0.N`, refuses authority mismatches, updates English/Japanese current-checkpoint declarations and version tables, updates the generated build input, and then runs the documentation consistency check.

After the mechanical bump, refine the implementation-status text and owning design/reference documentation so the new checkpoint describes actual code reality rather than only a number.

## Pull request classification

Every maintained PR should classify itself as exactly one of:

- a new development checkpoint;
- work inside the existing checkpoint (feature/hardening/acceptance);
- release/packaging-only work;
- docs/test/refactor/maintenance-only work.

A new-checkpoint PR updates the checkpoint authorities in the same change. Release-only work must not silently advance the development checkpoint.

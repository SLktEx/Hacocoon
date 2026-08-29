# Release security and provenance

[**日本語**](RELEASE_SECURITY.ja.md) | English

Hacocoon release verification has distinct integrity, provenance, and publication-control layers. They protect against different failures and should not be confused.

## Trusted release authority

A pushed release tag is **not** allowed to choose the publication workflow.

`.github/workflows/release.yml` is triggered by a `repository_dispatch` event named `release`. GitHub runs `repository_dispatch` workflows from the repository default branch, so the workflow definition and `GITHUB_SHA` come from trusted default-branch history rather than from a commit selected by a release tag.

The request contains only a version string such as `v0.13.0`. It cannot select a source branch or source SHA. The build source is the trusted default-branch `GITHUB_SHA` for that dispatch.

Request a release with:

```bash
bash tools/request_release.sh v0.13.0
```

The helper is only an operator convenience wrapper around the GitHub repository-dispatch API. It is not the trust root. The trusted workflow independently validates the version, event ref, source SHA, default-branch ancestry, and release-tag state.

The release tag must not already exist. The read-only build creates only a local tag so GoReleaser can derive the requested version. The privileged publisher later creates the GitHub Release with `--target "$GITHUB_SHA"`; because the remote tag is absent, GitHub creates the release tag at that trusted SHA. The workflow resolves the resulting tag back to a commit and refuses to publish the draft unless it equals `GITHUB_SHA`.

```text
repository_dispatch(release, version only)
        |
        v
trusted default-branch workflow + SHA
        |
        v
read-only build job
  validate dispatch context
  reject existing remote tag
  test / vet / package
        |
        v
same-run workflow artifact
        |
        v
release Environment
minimal publish job
  attest exact payload
  create tag + draft at trusted SHA
  verify tag -> trusted SHA
  publish draft
```

The build job has only `contents: read`. The publisher is the only job granted `id-token: write`, `attestations: write`, and `contents: write`; it does not check out or execute repository source. Release jobs are globally serialized to avoid racing two version requests.

The publish job references the `release` GitHub Environment. Before public release, configure that Environment with the intended reviewers/deployment restrictions. Merely naming an Environment in YAML does not substitute for configuring its protection rules.

Also protect `main` and release-related repository settings. A compromised write-capable actor may be able to request a release of trusted `main`; branch/ruleset protection, tag/release policy, and release-environment review remain important defense in depth.

## 1. SHA-256 integrity

GoReleaser publishes `checksums.txt`, and `scripts/install.sh` verifies the selected archive against it before extracting or installing binaries.

This detects corruption and mismatched downloads. It does **not** independently authenticate the publisher because the archive and `checksums.txt` are assets of the same GitHub Release. An authority able to replace both could make a malicious archive match a malicious checksum file.

## 2. GitHub/Sigstore build provenance

For a public Hacocoon repository, `.github/workflows/release.yml` generates a GitHub artifact attestation for the exact release payload before publishing the GitHub Release. The attestation uses GitHub Actions OIDC and Sigstore and carries SLSA build provenance.

The release workflow pins `actions/attest`, `actions/upload-artifact`, and `actions/download-artifact` to full commit SHAs.

Because the trusted release workflow runs from `main`, the attestation source ref is `refs/heads/main`. The release tag created by the workflow points to the same source commit. Verification should therefore bind both the trusted source ref and the exact commit resolved from the release tag.

### Verify an archive

Use a current GitHub CLI:

```bash
source_sha="$(gh api repos/SLktEx/Hacocoon/commits/v0.13.0 --jq .sha)"

gh attestation verify ./haco_linux_amd64.tar.gz \
  --repo SLktEx/Hacocoon \
  --signer-workflow SLktEx/Hacocoon/.github/workflows/release.yml \
  --source-ref refs/heads/main \
  --source-digest "$source_sha" \
  --deny-self-hosted-runners
```

This verifies the artifact digest, expected repository and signer workflow, GitHub Actions OIDC identity, trusted `main` source ref, and the exact source commit associated with the release tag.

### Installer behavior

The installer always requires the SHA-256 check to succeed. If a GitHub CLI with artifact-attestation support is available, it resolves the actual release tag (including when `latest` was requested), resolves that tag to its source commit, and attempts provenance verification before extraction.

To require provenance and fail closed when it cannot be verified:

```bash
HACO_REQUIRE_PROVENANCE=1 ./install.sh v0.13.0
```

Strict provenance mode requires the GitHub CLI/API calls needed to resolve and verify the release identity.

## Private repository limitation

GitHub artifact attestations are available for public repositories on current GitHub plans. Private/internal repository attestations require GitHub Enterprise Cloud.

While Hacocoon remains a private repository, the release workflow skips the public artifact-attestation step rather than silently pretending that SHA-256 provides publisher authenticity. Once the repository is public, the attestation step is mandatory: if attestation generation fails, release creation does not run.

A private Enterprise Cloud deployment may replace this visibility gate with an explicitly reviewed policy once private attestations are enabled.

## Immutable Releases

Artifact provenance makes an asset replacement detectable: a replacement file will not match the signed subject digest. GitHub Immutable Releases provide an additional server-side control that prevents published release assets and their associated tags from being changed or deleted.

Enable this repository setting before the first public release:

```text
Repository Settings
  -> Releases
  -> Enable release immutability
```

This setting requires repository administration authority and is intentionally **not** automated with a long-lived admin token in the release workflow.

The Hacocoon workflow is compatible with Immutable Releases: it creates a populated draft, verifies the generated tag points to the trusted source SHA, and only then publishes the draft. Immutability applies after publication.

When release immutability is enabled, a current GitHub CLI can also verify GitHub's release-level attestation:

```bash
gh release verify v0.13.0 --repo SLktEx/Hacocoon
gh release verify-asset v0.13.0 ./haco_linux_amd64.tar.gz --repo SLktEx/Hacocoon
```

## Security properties

| Check | Detects corruption | Binds expected repository/workflow | Binds source | Prevents post-publication replacement |
|---|---:|---:|---:|---:|
| `checksums.txt` | yes | no | no | no |
| Artifact attestation | yes | yes | `main` + exact commit digest | replacement fails verification |
| Trusted release dispatch | n/a | workflow comes from default branch | source fixed to dispatch `GITHUB_SHA` | n/a |
| Immutable Release | yes | GitHub release identity | associated release tag | yes |

For public Hacocoon releases, protect `main`, configure the `release` Environment and tag/release rules, use artifact attestation as the publisher/build provenance layer, and enable Immutable Releases as defense in depth.

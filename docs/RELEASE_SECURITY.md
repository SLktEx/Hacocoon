# Release security and provenance

[**日本語**](RELEASE_SECURITY.ja.md) | English

Hacocoon release verification has two distinct layers. They protect against different failures and should not be confused.

## 1. SHA-256 integrity

GoReleaser publishes `checksums.txt`, and `scripts/install.sh` verifies the selected archive against it before extracting or installing binaries.

This detects corruption and mismatched downloads. It does **not** independently authenticate the publisher because the archive and `checksums.txt` are assets of the same GitHub Release. An authority able to replace both could make a malicious archive match a malicious checksum file.

## 2. GitHub/Sigstore build provenance

For a public Hacocoon repository, `.github/workflows/release.yml` generates a GitHub artifact attestation for the exact release payload before publishing the GitHub Release. The attestation uses GitHub Actions OIDC and Sigstore and carries SLSA build provenance.

The workflow deliberately separates build authority from publication/signing authority:

```text
trusted tag on main
       |
       v
read-only build job
  test / vet / package
       |
       v
same-run workflow artifact
       |
       v
minimal publish job
  attest exact payload
  create draft release
  attach all assets
  publish draft
```

The build job has only `contents: read`. The publisher receives the staged payload through GitHub Actions artifact storage and is the only job granted `id-token: write`, `attestations: write`, and `contents: write`. It does not check out or execute repository source.

The release workflow pins `actions/attest`, `actions/upload-artifact`, and `actions/download-artifact` to full commit SHAs.

### Verify an archive

Use a current GitHub CLI:

```bash
gh attestation verify ./haco_linux_amd64.tar.gz \
  --repo SLktEx/Hacocoon \
  --signer-workflow SLktEx/Hacocoon/.github/workflows/release.yml \
  --source-ref refs/tags/v0.8.0 \
  --deny-self-hosted-runners
```

Change the tag to the release you downloaded. This verifies the artifact digest plus the expected repository, signer workflow, tag ref, GitHub Actions OIDC identity, and supported provenance signature chain.

If verifying `latest` without pinning a tag, omit `--source-ref`; pinning an explicit release tag is stronger.

### Installer behavior

The installer always requires the SHA-256 check to succeed. If a GitHub CLI with artifact-attestation support is available, it also attempts provenance verification before extraction.

To require provenance and fail closed when it cannot be verified:

```bash
HACO_REQUIRE_PROVENANCE=1 ./install.sh v0.8.0
```

Use an explicit version when using strict provenance mode so the installer can enforce the expected `refs/tags/<version>` source ref.

## Private repository limitation

GitHub artifact attestations are available for public repositories on current GitHub plans. Private/internal repository attestations require GitHub Enterprise Cloud.

While Hacocoon remains a private repository, the release workflow skips the public artifact-attestation step rather than silently pretending that SHA-256 provides publisher authenticity. Once the repository is public, the attestation step is mandatory: if attestation generation fails, the publish step does not run.

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

The Hacocoon workflow is already compatible with Immutable Releases: it creates a draft, attaches the complete asset set, and only then publishes the draft. Immutability applies after publication.

When release immutability is enabled, a current GitHub CLI can also verify GitHub's release-level attestation:

```bash
gh release verify v0.8.0 --repo SLktEx/Hacocoon
gh release verify-asset v0.8.0 ./haco_linux_amd64.tar.gz --repo SLktEx/Hacocoon
```

## Security properties

| Check | Detects corruption | Binds expected repository/workflow | Binds artifact digest | Prevents post-publication replacement |
|---|---:|---:|---:|---:|
| `checksums.txt` | yes | no | yes, same authority | no |
| Artifact attestation | yes | yes | yes, signed provenance | replacement fails verification |
| Immutable Release | yes | GitHub release identity | yes, release attestation | yes |

For public Hacocoon releases, use artifact attestation as the publisher/build provenance layer and enable Immutable Releases as defense in depth.

# Release security and provenance

[**日本語**](RELEASE_SECURITY.ja.md) | English

Hacocoon release verification has separate integrity, source-authorization, privilege-separation, and provenance layers. They protect against different failures.

The current repository model is deliberately **solo-maintainer and contribution-closed**. External pull requests are disabled and the repository owner is the only trusted write authority. See [Public repository security checklist](PUBLIC_RELEASE_CHECKLIST.md).

## 1. SHA-256 integrity

GoReleaser publishes `checksums.txt`, and `scripts/install.sh` verifies the selected archive before extracting or installing binaries.

This detects corruption and mismatched downloads. It does **not** independently authenticate the publisher because the artifact and its checksum are published by the same release authority.

## 2. Official release source authorization

The release workflow is not triggered directly by a pushed `v*` tag.

`.github/workflows/release.yml` runs from the trusted default branch through a `repository_dispatch` event. The trusted control-plane checkout validates the requested tag with `tools/check_release_tag_trust.sh`.

An official release tag must resolve to the **current remote default-branch HEAD**. The resolved release SHA must also equal the dispatcher `GITHUB_SHA`.

This rejects both:

- detached commits that never entered trusted `main`; and
- historical-main rollback releases under a fresh version tag.

Only after authorization does the build job checkout the exact approved release SHA.

```text
repository_dispatch(release, tag=vX.Y.Z)
       |
       v
trusted current-main workflow + trust checker
       |
       +--> require tag -> current remote main HEAD
       +--> require release SHA == dispatcher GITHUB_SHA
       |
       v
checkout exact authorized release commit
       |
       v
read-only build / test / package
       |
       v
same-run release payload
       |
       v
GitHub Environment: release
       |
       v
minimal privileged publisher
  revalidate main + tag identity
  attest exact payload
  publish GitHub Release
```

Immediately before signing and publication, the publisher re-checks that:

1. current default-branch HEAD still equals the authorized release SHA; and
2. the release tag, after peeling annotated tags, still resolves to the same SHA.

If either identity changed after the build, publication fails closed.

## 3. Solo-maintainer authorization model

The `publish` job references the GitHub Environment named `release`, but the current repository does **not** require an independent Environment reviewer.

With one trusted maintainer there is no second human capable of supplying an independent approval. Requiring one would deadlock release operations or force a bypass, neither of which creates a real trust boundary.

The current authorization boundary instead relies on all of these together:

- external pull-request creation remains disabled (`collaborators_only`);
- there are no non-owner direct collaborators;
- `main` is protected and changes require required CI through a PR;
- release tags cannot be moved or deleted after creation;
- a release tag is accepted only at current trusted `main` HEAD;
- build/test execution is separated from the write-capable publisher;
- publication revalidates tag/main identity;
- published artifacts are attested.

The `release` Environment remains a named privilege boundary and a future protection point.

If a second trusted maintainer or any other write-capable collaborator is added, add an independent required reviewer and prevent self-review where supported **before** treating the new multi-maintainer model as equally trusted.

To request a release:

```bash
gh api --method POST repos/SLktEx/Hacocoon/dispatches \
  -f event_type=release \
  -F 'client_payload[tag]=v0.8.0'
```

In the current solo-maintainer model, dispatch authority and final human authority belong to the same trusted owner. The workflow still minimizes the amount of code that receives publication credentials.

## 4. GitHub/Sigstore attestations

For a public Hacocoon repository, the privileged publisher creates two attestations for the exact release payload:

1. standard GitHub/Sigstore build provenance tying artifact digest to the expected repository/workflow;
2. a Hacocoon release-binding attestation (`https://hacocoon.dev/attestations/release/v1`) recording release tag, release commit SHA, trusted control ref/SHA, and the `release` Environment identity.

The build job has only `contents: read`. The publisher alone receives the write/OIDC/attestation permissions needed for publication. The publisher does not checkout repository source or execute repository tests/build scripts.

Trusted Actions remain pinned to immutable full commit SHAs.

### Verify an archive

Use a current GitHub CLI:

```bash
gh attestation verify ./haco_linux_amd64.tar.gz \
  --repo SLktEx/Hacocoon \
  --signer-workflow SLktEx/Hacocoon/.github/workflows/release.yml \
  --source-ref refs/heads/main \
  --deny-self-hosted-runners
```

For an explicit release binding:

```bash
gh attestation verify ./haco_linux_amd64.tar.gz \
  --repo SLktEx/Hacocoon \
  --signer-workflow SLktEx/Hacocoon/.github/workflows/release.yml \
  --source-ref refs/heads/main \
  --predicate-type https://hacocoon.dev/attestations/release/v1 \
  --deny-self-hosted-runners \
  --format json \
  --jq '.[].verificationResult.statement.predicate'
```

### Installer behavior

The Linux installer requires SHA-256 integrity and trusted GitHub/Sigstore provenance by default. `latest` is first resolved to an explicit version tag and the release binding is verified before installation.

`HACO_REQUIRE_PROVENANCE=0` is only an explicit private/development escape hatch.

The Windows installer resolves `latest`, verifies downloaded release inputs against trusted provenance, and invokes the Linux installer with provenance verification enabled.

## 5. Immutable Releases

Artifact provenance makes replacement detectable. GitHub Immutable Releases add a server-side control that prevents supported mutation of published release assets and associated tags.

Enable release immutability before relying on official releases as a stable distribution channel.

When enabled, a current GitHub CLI can also verify release-level attestations:

```bash
gh release verify v0.8.0 --repo SLktEx/Hacocoon
gh release verify-asset v0.8.0 ./haco_linux_amd64.tar.gz --repo SLktEx/Hacocoon
```

## Security properties

| Check | Main property |
|---|---|
| `checksums.txt` | transport/file integrity relative to the same release authority |
| current-main tag authorization | rejects detached commits and historical-main rollback releases |
| contribution-closed solo-maintainer policy | keeps trusted repository write authority with the owner |
| build/publish job split | minimizes code executing with release write authority |
| pre-publish main/tag revalidation | detects default-branch or tag movement between build and publication |
| standard artifact attestation | binds artifact digest to expected repository/workflow execution |
| release-binding attestation | signs release tag, commit, control identity, and release Environment identity |
| Immutable Release | prevents supported post-publication release/tag mutation server-side |

These layers are complementary. Re-audit the model before enabling external PRs, adding another write-capable actor, or attaching self-hosted runners.

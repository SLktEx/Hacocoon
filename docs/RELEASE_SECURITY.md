# Release security and provenance

[**日本語**](RELEASE_SECURITY.ja.md) | English

Hacocoon release verification has distinct integrity, authorization, and provenance layers. They protect against different failures and should not be confused.

## 1. SHA-256 integrity

GoReleaser publishes `checksums.txt`, and `scripts/install.sh` verifies the selected archive against it before extracting or installing binaries.

This detects corruption and mismatched downloads. It does **not** independently authenticate the publisher because the archive and `checksums.txt` are assets of the same GitHub Release. An authority able to replace both could make a malicious archive match a malicious checksum file.

## 2. Official release authorization

The release workflow is **not** triggered by a pushed `v*` tag. A tag-controlled commit must never be able to replace the workflow or trust checker which decides whether that tag may publish an official release.

`.github/workflows/release.yml` is triggered with the `release` `repository_dispatch` event. GitHub runs `repository_dispatch` workflows from the repository default branch. The trusted control-plane checkout is pinned to the dispatcher `GITHUB_SHA` and validates the requested tag with the trusted `tools/check_release_tag_trust.sh`.

An official release tag must resolve to the **current remote default-branch HEAD**, not merely to any commit in trusted `main` history. The resolved release SHA must also equal the dispatcher `GITHUB_SHA`. This deliberately rejects historical rollback releases: a write-capable account cannot mint a new version tag on an older vulnerable `main` commit and obtain valid official provenance for it.

Only after that authorization does a second checkout load the release source by the exact authorized SHA.

```text
repository_dispatch(release, tag=vX.Y.Z)
       |
       v
trusted current main workflow + trust checker
       |
       +--> require tag -> current remote main HEAD
       +--> require release SHA == dispatcher GITHUB_SHA
       |
       v
checkout exact authorized release commit
       |
       v
read-only build job
  test / vet / package
       |
       v
same-run workflow artifact
       |
       v
GitHub Environment: release
  required human approval (repository setting)
       |
       v
minimal privileged publish job
  require main HEAD is still the same commit
  re-resolve tag and require same commit
  attest exact payload
  publish GitHub Release
```

The `publish` job references the dedicated GitHub Environment named `release`. **The environment reference in YAML is not sufficient by itself.** Before public release, configure that environment in GitHub repository settings with required reviewer protection and prevent self-review where supported. The reviewer set is part of the release trust root and should be narrower than ordinary repository write access.

GitHub documents that required reviewers are available for public repositories on current Free, Pro, and Team plans; they are not available for private repositories on those plans. Therefore Hacocoon must not treat the current private-repository workflow as satisfying the human authorization requirement merely because it names the `release` environment. The public-launch checklist must configure and verify the environment protection after public conversion and before any official public release.

Immediately before signing and publication, the publisher also queries the GitHub API and requires both of these facts to remain true:

1. the current default-branch HEAD still equals the authorized release SHA;
2. the release tag still resolves, after peeling annotated tags, to that same SHA.

If `main` advances, the tag moves, or either identity changes after the build, the release fails closed and must be requested again from the new current HEAD.

To request a release, an authorized maintainer/automation can send a repository dispatch whose payload contains the release tag, for example:

```bash
gh api --method POST repos/SLktEx/Hacocoon/dispatches \
  -f event_type=release \
  -F 'client_payload[tag]=v0.8.0'
```

Dispatch authority is **not** publication authority. The server-side `release` Environment approval is the separate human authorization boundary for the privileged publisher.

This workflow-level design does not replace GitHub repository controls. Before public release, protect `main`, restrict release-tag creation/update/deletion, minimize write/bypass actors, configure the `release` Environment reviewers, and enable the documented public-fork runner policy.

## 3. GitHub/Sigstore attestations

For a public Hacocoon repository, the privileged publisher creates two attestations for the exact release payload:

1. standard GitHub/Sigstore build provenance proving the artifact was signed by the expected Hacocoon release workflow running from trusted `main`;
2. a Hacocoon release-binding attestation (`https://hacocoon.dev/attestations/release/v1`) containing the authorized release tag, release commit SHA, trusted control ref, trusted control SHA, and the expected `release` authorization environment name.

The build job has only `contents: read`. The publisher alone receives `contents: write`, `id-token: write`, `attestations: write`, and `artifact-metadata: write`. The publisher does not checkout repository source or execute repository tests/build scripts. Environment protection must pass before the privileged publish job is sent to a runner.

`actions/attest`, `actions/upload-artifact`, and `actions/download-artifact` are pinned to full commit SHAs.

### Verify an archive

Use a current GitHub CLI. First verify that the artifact was attested by the trusted release workflow running from `main`:

```bash
gh attestation verify ./haco_linux_amd64.tar.gz \
  --repo SLktEx/Hacocoon \
  --signer-workflow SLktEx/Hacocoon/.github/workflows/release.yml \
  --source-ref refs/heads/main \
  --deny-self-hosted-runners
```

For an explicit version, also inspect the signed release binding:

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

The predicate records the signed `tag`, `commit`, trusted control identity, and expected authorization environment selected by the trusted release control plane.

### Installer behavior

The Linux installer always requires SHA-256 integrity **and trusted GitHub/Sigstore provenance by default**. `latest` is first resolved to an explicit `vX.Y.Z` release tag, then the archive is verified against the trusted release workflow, `refs/heads/main`, and the signed release-binding predicate for that resolved tag before installation.

`HACO_REQUIRE_PROVENANCE=0` exists only as an explicit private/development escape hatch. It is not the supported public-install trust model.

The Windows installer resolves `latest` to an explicit tag, verifies `checksums.txt`, `bootstrap-wsl.sh`, and `install.sh` independently against trusted provenance and the signed release binding before executing either downloaded script, and invokes the Linux installer with `HACO_REQUIRE_PROVENANCE=1`.

## Private repository limitation

GitHub artifact attestations are available for public repositories on current GitHub plans. Private/internal repository attestations require GitHub Enterprise Cloud.

While Hacocoon remains private, the workflow skips public artifact attestation rather than presenting SHA-256 as publisher authentication. Once the repository is public, both attestation steps are mandatory and publication stops if they fail.

The same publication boundary applies to human approval: on GitHub Free, Pro, and Team, required Environment reviewers are public-repository-only. Configure and verify required reviewers on the `release` Environment after public conversion and before publishing any official public release.

## Immutable Releases

Artifact provenance makes asset replacement detectable. GitHub Immutable Releases add a server-side control that prevents published release assets and their associated tags from being changed or deleted.

Enable this repository setting before the first public release:

```text
Repository Settings
  -> Releases
  -> Enable release immutability
```

This requires repository administration authority and is intentionally not automated with a long-lived admin token in the release workflow. The workflow creates a populated draft first and publishes it only after all assets and attestations are ready.

When release immutability is enabled, a current GitHub CLI can also verify GitHub's release-level attestation:

```bash
gh release verify v0.8.0 --repo SLktEx/Hacocoon
gh release verify-asset v0.8.0 ./haco_linux_amd64.tar.gz --repo SLktEx/Hacocoon
```

## Security properties

| Check | Main property |
|---|---|
| `checksums.txt` | transport/file integrity relative to the same release authority |
| current-main tag authorization | rejects detached commits and historical-main rollback releases |
| `release` Environment required reviewer | separates human publication approval from ordinary repository write/dispatch authority |
| pre-publish main/tag revalidation | detects default-branch or tag movement between build and publication |
| standard artifact attestation | binds artifact digest to expected repository/workflow and trusted-main workflow execution |
| release-binding attestation | signs the authorized release tag, commit identity, control identity, and expected authorization environment |
| Immutable Release | prevents supported post-publication release/tag mutation server-side |

These layers are complementary. None replaces protected `main`, restricted release tags, safe fork-PR settings, or runner isolation.

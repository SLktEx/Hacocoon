#!/usr/bin/env python3
from pathlib import Path
import sys

root = Path(__file__).resolve().parents[1]
workflow = (root / ".github/workflows/release.yml").read_text(encoding="utf-8")
installer = (root / "scripts/install.sh").read_text(encoding="utf-8")

checks = {
    "release workflow": [
        "repository_dispatch:\n    types: [release]",
        "permissions:\n  contents: read",
        "group: release",
        "build:",
        "publish:",
        "RELEASE_TAG: ${{ github.event.client_payload.tag }}",
        'bash tools/check_release_request.sh "$RELEASE_TAG" "$GITHUB_REF" "$DEFAULT_BRANCH" "$GITHUB_SHA" origin',
        'git ls-remote --exit-code --tags origin "refs/tags/$RELEASE_TAG"',
        'git tag "$RELEASE_TAG" "$GITHUB_SHA"',
        "environment: release",
        "contents: write",
        "id-token: write",
        "attestations: write",
        "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a",
        "actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c",
        "actions/attest@1e69f48acb82d1966a394da916b4c1698aa569d6",
        "if: ${{ !github.event.repository.private }}",
        "release-payload/haco_linux_amd64.tar.gz",
        "release-payload/haco_linux_arm64.tar.gz",
        "release-payload/checksums.txt",
        '--target "$GITHUB_SHA"',
        'release_sha="$(gh api "repos/$GITHUB_REPOSITORY/commits/$RELEASE_TAG" --jq .sha)"',
        'if [[ "$release_sha" != "$GITHUB_SHA" ]]',
        "--draft",
        "gh release edit",
        "--draft=false",
    ],
    "installer": [
        'SIGNER_WORKFLOW="$REPOSITORY/.github/workflows/release.yml"',
        'SOURCE_REF="refs/heads/main"',
        "HACO_REQUIRE_PROVENANCE",
        "gh attestation verify",
        '--repo "$REPOSITORY"',
        '--signer-workflow "$SIGNER_WORKFLOW"',
        'gh release view --repo "$REPOSITORY" --json tagName --jq .tagName',
        'gh api "repos/$REPOSITORY/commits/$release_tag" --jq .sha',
        '--source-ref "$SOURCE_REF"',
        '--source-digest "$source_sha"',
        "--deny-self-hosted-runners",
        "SHA-256 integrity verified, but provenance",
    ],
}

errors = []
for label, needles in checks.items():
    text = workflow if label == "release workflow" else installer
    for needle in needles:
        if needle not in text:
            errors.append(f"{label} missing required provenance contract: {needle}")

# A tag push or branch-selectable manual dispatch would let a non-default ref
# choose the workflow definition. Release authority must be sourced from the
# default-branch repository_dispatch context instead.
for forbidden in ("\n  push:\n", "\n  workflow_dispatch:\n"):
    if forbidden in workflow:
        errors.append(f"release workflow contains forbidden trigger: {forbidden.strip()}")

# The privileged publisher must not check out or execute repository source.
publish = workflow.split("\n  publish:\n", 1)[1]
for forbidden in ("actions/checkout@", "go test", "go vet", "goreleaser release"):
    if forbidden in publish:
        errors.append(f"privileged publish job must not execute repository build source: {forbidden}")

# The read-only build must not gain publication or signing authority.
build = workflow.split("\n  build:\n", 1)[1].split("\n  publish:\n", 1)[0]
for forbidden in ("contents: write", "id-token: write", "attestations: write"):
    if forbidden in build:
        errors.append(f"release build job must remain unprivileged: {forbidden}")

# Binary attestations do not need linked-artifact storage metadata authority.
if "artifact-metadata: write" in workflow:
    errors.append("release workflow grants unnecessary artifact-metadata: write permission")

# The installer must bind provenance to trusted main + the exact commit pointed
# to by the release tag, not expect the signing workflow itself to run on a tag.
if '--source-ref "refs/tags/$VERSION"' in installer:
    errors.append("installer still assumes artifact attestation is signed from the release tag ref")

if errors:
    print("RELEASE PROVENANCE CONTRACT FAILED")
    print("\n".join(errors))
    sys.exit(1)

print("RELEASE PROVENANCE CONTRACT OK")

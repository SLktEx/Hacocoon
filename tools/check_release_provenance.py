#!/usr/bin/env python3
from pathlib import Path
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
workflow = (root / ".github/workflows/release.yml").read_text(encoding="utf-8")
installer = (root / "scripts/install.sh").read_text(encoding="utf-8")
tag_checker = (root / "tools/check_release_tag_trust.sh").read_text(encoding="utf-8")

checks = {
    "release workflow": [
        "repository_dispatch:",
        "types: [release]",
        "permissions:\n  contents: read",
        "build:",
        "publish:",
        "environment: release",
        "ref: ${{ github.sha }}",
        "path: control",
        "bash control/tools/check_release_tag_trust.sh",
        "release_sha=",
        "tag must resolve to the dispatcher/current trusted main HEAD",
        "ref: ${{ steps.trust.outputs.release_sha }}",
        "path: source",
        "contents: write",
        "id-token: write",
        "attestations: write",
        "artifact-metadata: write",
        "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a",
        "actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c",
        "actions/attest@1e69f48acb82d1966a394da916b4c1698aa569d6",
        'gh api "repos/$GITHUB_REPOSITORY/branches/$DEFAULT_BRANCH"',
        "trusted default branch moved after release authorization",
        "gh api \"repos/$GITHUB_REPOSITORY/git/ref/tags/$RELEASE_TAG\"",
        "tag moved after build",
        "https://hacocoon.dev/attestations/release/v1",
        '"authorizationEnvironment": "release"',
        "predicate-path: release-binding.json",
        "release-payload/haco_linux_amd64.tar.gz",
        "release-payload/haco_linux_arm64.tar.gz",
        "release-payload/checksums.txt",
        "--draft",
        "gh release edit",
        "--draft=false",
    ],
    "release tag checker": [
        'trusted_head="$(git -C "$repo_dir" rev-parse --verify "${remote_tracking_ref}^{commit}")"',
        'if [[ "$tag_commit" != "$trusted_head" ]]; then',
        "older vulnerable commit",
    ],
    "installer": [
        'SIGNER_WORKFLOW="$REPOSITORY/.github/workflows/release.yml"',
        'SIGNER_SOURCE_REF="refs/heads/main"',
        'RELEASE_PREDICATE_TYPE="https://hacocoon.dev/attestations/release/v1"',
        'REQUIRE_PROVENANCE="${HACO_REQUIRE_PROVENANCE:-1}"',
        "resolve_latest_version()",
        'VERSION="$(resolve_latest_version)"',
        'gh attestation verify',
        '--repo "$REPOSITORY"',
        '--signer-workflow "$SIGNER_WORKFLOW"',
        '--source-ref "$SIGNER_SOURCE_REF"',
        '--predicate-type "$RELEASE_PREDICATE_TYPE"',
        "verificationResult.statement.predicate.tag",
        '--deny-self-hosted-runners',
        "trusted provenance verification requires",
        "trusted build provenance verification failed",
        "signed release binding verification failed",
    ],
}

texts = {
    "release workflow": workflow,
    "release tag checker": tag_checker,
    "installer": installer,
}
errors = []
for label, needles in checks.items():
    text = texts[label]
    for needle in needles:
        if needle not in text:
            errors.append(f"{label} missing required provenance/authorization contract: {needle}")

if "\n  push:\n" in workflow:
    errors.append("release workflow must not be triggered by tag push; authorization must come from default-branch repository_dispatch")

if workflow.count("actions/attest@1e69f48acb82d1966a394da916b4c1698aa569d6") < 2:
    errors.append("release workflow must create both build provenance and signed release-binding attestations")

if 'HACO_REQUIRE_PROVENANCE:-0' in installer:
    errors.append("installer provenance must fail closed by default; HACO_REQUIRE_PROVENANCE may only disable it explicitly")
if 'if [ "$VERSION" = "latest" ]; then\n    if [ "$REQUIRE_PROVENANCE" = "1" ]' in installer:
    errors.append("latest installs must resolve to an explicit tag instead of weakening signed release-binding verification")
if "merge-base --is-ancestor" in tag_checker:
    errors.append("official release authorization must require current default-branch HEAD, not any historical ancestor")

try:
    build = workflow.split("\n  build:\n", 1)[1].split("\n  publish:\n", 1)[0]
    publish = workflow.split("\n  publish:\n", 1)[1]
except IndexError:
    errors.append("release workflow must contain separate build and publish jobs")
    build = ""
    publish = ""

for forbidden in ("contents: write", "id-token: write", "attestations: write", "artifact-metadata: write"):
    if forbidden in build:
        errors.append(f"read-only build job must not receive privileged permission: {forbidden}")

if "environment: release" in build:
    errors.append("release approval environment belongs only on privileged publish job; read-only build should complete before approval")
if publish and "environment: release" not in publish:
    errors.append("privileged publisher must be protected by the dedicated release environment")

for forbidden in ("actions/checkout@", "go test", "go vet", "goreleaser release", "source/"):
    if forbidden in publish:
        errors.append(f"privileged publish job must not checkout or execute repository build source: {forbidden}")

if publish and "artifact-metadata: write" not in publish:
    errors.append("privileged publisher must scope artifact-metadata: write locally for actions/attest")

if errors:
    print("RELEASE PROVENANCE CONTRACT FAILED")
    print("\n".join(errors))
    sys.exit(1)

regression = subprocess.run(
    ["bash", str(root / "tools/test_install_provenance_fail_closed.sh")],
    cwd=root,
    check=False,
)
if regression.returncode != 0:
    print("RELEASE PROVENANCE CONTRACT FAILED: installer fail-closed regression test failed", file=sys.stderr)
    sys.exit(regression.returncode)

print("RELEASE PROVENANCE CONTRACT OK")

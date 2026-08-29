#!/usr/bin/env python3
from pathlib import Path
import sys

root = Path(__file__).resolve().parents[1]
workflow = (root / ".github/workflows/release.yml").read_text(encoding="utf-8")
installer = (root / "scripts/install.sh").read_text(encoding="utf-8")

checks = {
    "release workflow": [
        "permissions:\n  contents: read",
        "build:",
        "publish:",
        "contents: write",
        "id-token: write",
        "attestations: write",
        "artifact-metadata: write",
        "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a",
        "actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c",
        "actions/attest@1e69f48acb82d1966a394da916b4c1698aa569d6",
        "if: ${{ !github.event.repository.private }}",
        "release-payload/haco_linux_amd64.tar.gz",
        "release-payload/haco_linux_arm64.tar.gz",
        "release-payload/checksums.txt",
        "--draft",
        "gh release edit",
        "--draft=false",
    ],
    "installer": [
        'SIGNER_WORKFLOW="$REPOSITORY/.github/workflows/release.yml"',
        'HACO_REQUIRE_PROVENANCE',
        'gh attestation verify',
        '--repo "$REPOSITORY"',
        '--signer-workflow "$SIGNER_WORKFLOW"',
        '--source-ref "refs/tags/$VERSION"',
        '--deny-self-hosted-runners',
        'SHA-256 integrity verified, but provenance',
    ],
}

errors = []
for label, needles in checks.items():
    text = workflow if label == "release workflow" else installer
    for needle in needles:
        if needle not in text:
            errors.append(f"{label} missing required provenance contract: {needle}")

# The privileged publisher must not check out or execute repository source.
publish = workflow.split("\n  publish:\n", 1)[1]
for forbidden in ("actions/checkout@", "go test", "go vet", "goreleaser release"):
    if forbidden in publish:
        errors.append(f"privileged publish job must not execute repository build source: {forbidden}")

if errors:
    print("RELEASE PROVENANCE CONTRACT FAILED")
    print("\n".join(errors))
    sys.exit(1)

print("RELEASE PROVENANCE CONTRACT OK")

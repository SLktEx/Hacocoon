#!/usr/bin/env python3
from pathlib import Path
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
workflow = (root / ".github/workflows/release.yml").read_text(encoding="utf-8")
installer = (root / "scripts/install.sh").read_text(encoding="utf-8")
tag_checker = (root / "tools/check_release_tag_trust.sh").read_text(encoding="utf-8")

required_release_artifacts = (
    "release-payload/haco_linux_amd64.tar.gz",
    "release-payload/haco_linux_arm64.tar.gz",
    "release-payload/hacocoon-windows-amd64.zip",
    "release-payload/hacocoon-windows-arm64.zip",
    "release-payload/hacocoon-ubuntu-amd64.tar.gz",
    "release-payload/hacocoon-ubuntu-arm64.tar.gz",
    "release-payload/checksums.txt",
)

checks = {
    "release workflow": [
        "repository_dispatch:",
        "types: [release]",
        "permissions:\n  contents: read",
        'PYTHONDONTWRITEBYTECODE: "1"',
        "build:",
        "publish:",
        "environment: release",
        "ref: ${{ github.sha }}",
        "path: control",
        "bash control/tools/check_release_tag_trust.sh",
        "release_sha=",
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
        "trusted default branch moved after authorization",
        "gh api \"repos/$GITHUB_REPOSITORY/git/ref/tags/$RELEASE_TAG\"",
        "tag moved after build",
        "https://hacocoon.dev/attestations/release/v1",
        '"authorizationEnvironment": "release"',
        "predicate-path: release-binding.json",
        "python3 source/tools/package_installers.py",
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
        'BUNDLE_ROOT="${HACO_BUNDLE_ROOT:-$SCRIPT_DIR}"',
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
        'Using bundled %s',
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

for artifact in required_release_artifacts:
    if workflow.count(artifact) < 2:
        errors.append(f"release workflow must stage/attest/publish architecture-specific artifact: {artifact}")

for obsolete in (
    "release-payload/hacocoon-windows-installer.zip",
    "release-payload/install.sh",
    "release-payload/install-windows.ps1",
    "package_windows_installer.py",
):
    if obsolete in workflow:
        errors.append(f"release workflow retains obsolete standalone/single-package artifact: {obsolete}")

if "\n  push:\n" in workflow:
    errors.append("release workflow must not be triggered by tag push; authorization must come from default-branch repository_dispatch")
if workflow.count("actions/attest@1e69f48acb82d1966a394da916b4c1698aa569d6") < 2:
    errors.append("release workflow must create both build provenance and signed release-binding attestations")
if 'HACO_REQUIRE_PROVENANCE:-0' in installer:
    errors.append("installer provenance must fail closed by default")
if "WSL_DISTRO_NAME" in installer or "systemd=true" in installer or "hacocoon-login" in installer:
    errors.append("common install.sh must not absorb WSL pre/post behavior")
if "merge-base --is-ancestor" in tag_checker:
    errors.append("official release authorization must require current default-branch HEAD, not any historical ancestor")

try:
    build = workflow.split("\n  build:\n", 1)[1].split("\n  publish:\n", 1)[0]
    publish = workflow.split("\n  publish:\n", 1)[1]
except IndexError:
    errors.append("release workflow must contain separate build and publish jobs")
    build = ""
    publish = ""

try:
    source_checkout = workflow.split("      - name: Checkout authorized release source\n", 1)[1].split("\n      - ", 1)[0]
except IndexError:
    errors.append("release workflow must contain the authorized release source checkout")
    source_checkout = ""

if source_checkout and "fetch-depth: 0" not in source_checkout:
    errors.append("authorized release source checkout must fetch full history")
if source_checkout and "fetch-tags: true" not in source_checkout:
    errors.append("authorized release source checkout must fetch tags")

bat_eol = subprocess.run(
    ["git", "ls-files", "--eol", "scripts/install-windows.bat"],
    cwd=root,
    check=False,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
    text=True,
)
if bat_eol.returncode != 0 or "i/lf" not in bat_eol.stdout:
    detail = bat_eol.stderr.strip() or bat_eol.stdout.strip() or "no EOL metadata returned"
    errors.append("scripts/install-windows.bat must remain LF-normalized in the Git index: " + detail)

for forbidden in ("contents: write", "id-token: write", "attestations: write", "artifact-metadata: write"):
    if forbidden in build:
        errors.append(f"read-only build job must not receive privileged permission: {forbidden}")
if "environment: release" in build:
    errors.append("release approval environment belongs only on privileged publish job")
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

for command, label in (
    (["bash", str(root / "tools/test_install_provenance_fail_closed.sh")], "installer fail-closed regression"),
    ([sys.executable, str(root / "tools/test_installer_packages.py")], "installer package regression"),
):
    regression = subprocess.run(command, cwd=root, check=False)
    if regression.returncode != 0:
        print(f"RELEASE PROVENANCE CONTRACT FAILED: {label} failed", file=sys.stderr)
        sys.exit(regression.returncode)

print("RELEASE PROVENANCE CONTRACT OK")

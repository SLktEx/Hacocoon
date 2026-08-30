#!/usr/bin/env python3
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parents[1]
files = list(root.rglob("*.md"))

errors = []
checks = [
    (r"13_v0\.13_LOCAL_OCI_REGISTRY|13A_v0\.13_OCI_SEED_AND_COW|13B_v0\.13_SEED_AUTO_PROMOTION|13C_v0\.13_OCI_IMAGE_DELETION", "superseded v0.13 OCI filename"),
    (r"18_v0\.18_LOCAL_OCI_REGISTRY|19_v0\.19_OCI_SEED_AND_COW", "superseded OCI roadmap filename"),
    (r"\|\s*v0\.(?:13|18)\s*\|\s*(?:Optional )?Local OCI Registry\s*\|", "stale Local Registry milestone assignment"),
    (r"\|\s*v0\.19\s*\|\s*OCI Seed Builder", "stale Seed Builder milestone assignment"),
    (r"implemented milestones are contiguous through \*\*v0\.12\*\*", "stale milestone ceiling"),
    (r"v0\.13 Local OCI Registry is planned", "stale next milestone"),
    (r"EC2 remains experimental and disabled by default", "stale active EC2 claim"),
    (r"\|\s*EC2 Environment provider\s*\|\s*experimental", "stale active EC2 provider claim"),
]
for p in files:
    text = p.read_text()
    for pattern, label in checks:
        for match in re.finditer(pattern, text, flags=re.IGNORECASE):
            line = text[:match.start()].count("\n") + 1
            errors.append(f"{p.relative_to(root)}:{line}: {label}")

superseded = [
    "docs/13_v0.13_LOCAL_OCI_REGISTRY.md",
    "docs/13A_v0.13_OCI_SEED_AND_COW.md",
    "docs/13A_v0.13_OCI_SEED_AND_COW.ja.md",
    "docs/13B_v0.13_SEED_AUTO_PROMOTION.md",
    "docs/13B_v0.13_SEED_AUTO_PROMOTION.ja.md",
    "docs/13C_v0.13_OCI_IMAGE_DELETION.md",
    "docs/13C_v0.13_OCI_IMAGE_DELETION.ja.md",
    "docs/18_v0.18_LOCAL_OCI_REGISTRY.md",
    "docs/18_v0.18_LOCAL_OCI_REGISTRY.ja.md",
    "docs/19_v0.19_OCI_SEED_AND_COW.md",
    "docs/19_v0.19_OCI_SEED_AND_COW.ja.md",
]
for rel in superseded:
    if (root / rel).exists():
        errors.append(f"superseded documentation file still exists: {rel}")

obsolete_artifacts = [
    "CODEX_START_HERE.md",
    "Hacocoon_v0.1-v0.7_MASTER.md",
    "REFACTOR_NOTES.md",
    ".github/AGENTS.md",
    "docs/90_CODEX_IMPLEMENTATION_HANDOFF.md",
    "docs/91_IMPLEMENTATION_REFERENCE_NOTES.md",
    "docs/REMOTE_CLOUD_PROVISIONING.md",
    "docs/IMPLEMENTATION_STATUS_TEMPLATE.md",
    "tools/build_master.py",
]
for rel in obsolete_artifacts:
    if (root / rel).exists():
        errors.append(f"obsolete repository artifact still exists: {rel}")

required = [
    "README.md", "README.ja.md", "AGENTS.md",
    "docs/README.md", "docs/README.ja.md",
    "docs/00A_PLUGIN_ARCHITECTURE.md",
    "docs/00_REBASELINE_AND_ROADMAP.md",
    "docs/00D_VERSIONING_AND_RELEASE_STATUS.md",
    "docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md",
    "docs/IMPLEMENTATION_STATUS.md", "docs/IMPLEMENTATION_STATUS.ja.md",
    "docs/ARCHITECTURE_GUIDE.ja.md",
    "docs/CLIENT_ADAPTER_CONTRACT.md", "docs/CLIENT_ADAPTER_CONTRACT.ja.md",
    "docs/OCI_RUNTIME_AND_DOCKER_COMPAT.md", "docs/OCI_RUNTIME_AND_DOCKER_COMPAT.ja.md",
    "docs/13_v0.13_MANAGED_SANDBOX_NETWORK.md", "docs/13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md",
    "docs/14_v0.14_GIT_FETCH_PLUGIN.md", "docs/14_v0.14_GIT_FETCH_PLUGIN.ja.md",
    "docs/15_v0.15_OCI_SEED_RECOMMENDATION.md", "docs/15_v0.15_OCI_SEED_RECOMMENDATION.ja.md",
    "docs/16_v0.16_OCI_IMAGE_DELETION.md", "docs/16_v0.16_OCI_IMAGE_DELETION.ja.md",
    "docs/17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md", "docs/17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md",
    "docs/18_v0.18_OCI_SEED_AND_COW.md", "docs/18_v0.18_OCI_SEED_AND_COW.ja.md",
    "docs/OPTIONAL_LOCAL_OCI_REGISTRY.md", "docs/OPTIONAL_LOCAL_OCI_REGISTRY.ja.md",
]
for rel in required:
    if not (root / rel).exists():
        errors.append(f"missing required documentation: {rel}")


def require_text(path, items):
    text = (root / path).read_text()
    lower = text.lower()
    for item in items:
        if item.lower() not in lower:
            errors.append(f"{path} missing required text: {item}")

require_text("AGENTS.md", [
    "docs/README.md", ".github/security/ADVERSARIAL_AUDIT.md",
    "Do not add tool-specific handoff files", "bash tools/ci-local.sh",
])
require_text("docs/00D_VERSIONING_AND_RELEASE_STATUS.md", [
    "One independently useful product feature",
    "v0.13 | Managed Sandbox Network",
    "v0.14 | Git Fetch Plugin",
    "v0.15 | OCI Seed Recommendation",
    "v0.16 | OCI Image Deletion",
    "v0.17 | Docker Compatibility Plugin",
    "contiguous through **v0.17**",
    "v0.18 | OCI Seed Builder & Btrfs/COW",
    "first repository slice",
    "Local Registry infrastructure is deferred and unversioned",
    "cloud implementation is currently deferred",
])
require_text("docs/IMPLEMENTATION_STATUS.md", [
    "current code reality", "contiguous through **v0.17**",
    "Managed sandbox network", "haco plugin git fetch",
    "haco plugin oci seed sample", "haco plugin oci seed recommend",
    "haco plugin oci seed build", "haco plugin oci seed current",
    "auto_promote", "haco plugin oci image delete", "Docker compatibility",
    "haco plugin oci docker status", "prepare <environment>",
    "pkg/clientadapter", "haco ssh", "v0.18", "Optional Local OCI Registry", "unversioned optional",
    "cloud implementation is currently deferred", "HACO_PLUGIN_OCI=nerdctl|docker",
])
require_text("docs/00_REBASELINE_AND_ROADMAP.md", [
    "Hacocoon is a **Secure Workspace Runtime**",
    "v0.13 | Managed Sandbox Network", "v0.14 | Git Fetch Plugin",
    "v0.15 | OCI Seed Recommendation", "v0.16 | OCI Image Deletion",
    "v0.17 | Docker Compatibility Plugin", "contiguous through **v0.17**",
    "v0.18 | OCI Seed Builder & Btrfs/COW", "first repository slice",
    "Local OCI Registry is not a roadmap milestone",
    "cloud implementation is currently deferred", "Historical note",
])
require_text("docs/README.md", [
    "Source-of-truth order", "Numbering rule",
    "13_v0.13_MANAGED_SANDBOX_NETWORK.md", "14_v0.14_GIT_FETCH_PLUGIN.md",
    "15_v0.15_OCI_SEED_RECOMMENDATION.md", "16_v0.16_OCI_IMAGE_DELETION.md",
    "17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md", "18_v0.18_OCI_SEED_AND_COW.md",
    "OPTIONAL_LOCAL_OCI_REGISTRY.md", "CLIENT_ADAPTER_CONTRACT.md", "pkg/clientadapter",
    "haco base list", "haco plugin oci", "haco plugin oci docker prepare", "contiguous through **v0.17**",
])
require_text("docs/CLIENT_ADAPTER_CONTRACT.md", [
    "pkg/clientadapter", "public key", "private key", "loopback-only", "/workspace",
    "haco ssh", "pkg/interaction", "VS Code", "JetBrains", "code-server",
])
require_text("docs/00A_PLUGIN_ARCHITECTURE.md", [
    "HACO_PLUGIN_OCI=nerdctl", "HACO_PLUGIN_OCI=docker", "unset HACO_PLUGIN_OCI",
    "Core must not require", "haco plugin oci", "haco base",
])
require_text("docs/OCI_RUNTIME_AND_DOCKER_COMPAT.md", [
    "v0.17", "plugin/adapter", "containerd + nerdctl", "v0.18", "Environment-local",
    "not a Core invariant", "Local OCI Registry", "haco plugin oci docker prepare",
])
require_text("docs/13_v0.13_MANAGED_SANDBOX_NETWORK.md", ["Managed Sandbox Network", "haco-sandbox0", "fail closed"])
require_text("docs/14_v0.14_GIT_FETCH_PLUGIN.md", ["Git Fetch Plugin", "haco plugin git fetch", "gh auth git-credential"])
require_text("docs/15_v0.15_OCI_SEED_RECOMMENDATION.md", ["OCI Seed Recommendation", "haco plugin oci seed recommend", "top **10%**", "auto_promote", "v0.18"])
require_text("docs/16_v0.16_OCI_IMAGE_DELETION.md", ["OCI Image Deletion", "haco plugin oci image delete", "tombstone", "v0.18"])
require_text("docs/17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md", [
    "Docker Compatibility Plugin", "on-demand", "repository implementation complete",
    "haco plugin oci docker status", "haco plugin oci docker prepare", "fail closed",
])
require_text("docs/18_v0.18_OCI_SEED_AND_COW.md", [
    "OCI Seed Builder", "first repository slice", "haco plugin oci seed build",
    "haco plugin oci seed current", "Offline Seed Builder", "/var/lib/containerd",
    "Btrfs/COW", "Local Registry is not required",
])
require_text("docs/OPTIONAL_LOCAL_OCI_REGISTRY.md", ["Optional Local OCI Registry", "deferred optional infrastructure", "not a roadmap milestone", "not required"])
require_text("README.md", ["pre-1.0", "haco base list", "haco plugin oci"])
require_text("README.ja.md", ["読み方: はこーん", "pre-1.0", "haco base list", "haco plugin oci"])

if errors:
    print("DOC CONSISTENCY FAILED")
    print("\n".join(errors))
    sys.exit(1)
print("DOC CONSISTENCY OK")

#!/usr/bin/env python3
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parents[1]
files = [p for p in root.rglob("*.md") if p.name != "Hacocoon_v0.1-v0.7_MASTER.md"]

checks = [
    (r"v0\.1 Local Foundation|v0\.2 Developer Workspace|v0\.3 Security Framework|v0\.4 External Capabilities|v0\.5 Local GUI|v0\.6 Local Web", "old release ordering"),
    (r"01_v0\.1_LOCAL_FOUNDATION|02_v0\.2_DEVELOPER_WORKSPACE|03_v0\.3_SECURITY_FRAMEWORK_AND_GIT|04_v0\.4_EXTERNAL_CAPABILITIES|05_v0\.5_LOCAL_GUI_AND_IDE|06_v0\.6_LOCAL_WEB_AND_INTERACTION|07_v0\.7_REMOTE_AND_EC2", "superseded release filename"),
    (r"11_v0\.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER|#\s*v0\.11\s+VS Code Remote Agent Host Adapter", "stale v0.11 Agent Host assignment"),
    (r"13_v0\.13_LOCAL_OCI_REGISTRY|13A_v0\.13_OCI_SEED_AND_COW|13B_v0\.13_SEED_AUTO_PROMOTION|13C_v0\.13_OCI_IMAGE_DELETION", "superseded v0.13 OCI filename"),
    (r"\bv0\.13A\b|\bv0\.13B\b|\bv0\.13C\b", "superseded v0.13 lettered milestone"),
    (r"v0\.13\s*\|\s*Local OCI Registry", "stale Local Registry milestone assignment"),
    (r"implemented milestones are contiguous through \*\*v0\.12\*\*", "stale implemented-milestone ceiling"),
    (r"v0\.13 Local OCI Registry is planned", "stale next-milestone claim"),
    (r"\bHacocoon IAM\b", "legacy Hacocoon IAM term"),
    (r"Manager/Session trust boundary|Manager / Session trust boundary", "legacy Manager/Session architecture boundary"),
    (r"Runtime/Storage seams|Security and Feature Plugin boundaries", "pre-rebaseline ADR terminology"),
    (r"\bDirectoryWorkspace\b", "redundant workspace-provider name"),
    (r"Status:\s*\*\*current implementation gate\*\*", "stale v0.1-as-current status"),
    (r"active PR #111;\s*not yet on `main`|v0\.10 is the trusted adapter layer currently developed in PR #111", "stale PR #111 status"),
]

errors = []
for p in files:
    text = p.read_text()
    for pat, label in checks:
        for m in re.finditer(pat, text, flags=re.IGNORECASE):
            line = text[: m.start()].count("\n") + 1
            snippet = text.splitlines()[line - 1]
            if label == "legacy Hacocoon IAM term" and any(
                word in snippet.lower() for word in ("historical", "legacy", "do not")
            ):
                continue
            errors.append(f"{p.relative_to(root)}:{line}: {label}: {snippet.strip()}")

superseded_files = [
    "docs/01_v0.1_LOCAL_FOUNDATION.md",
    "docs/02_v0.2_DEVELOPER_WORKSPACE.md",
    "docs/03_v0.3_SECURITY_FRAMEWORK_AND_GIT.md",
    "docs/04_v0.4_EXTERNAL_CAPABILITIES.md",
    "docs/05_v0.5_LOCAL_GUI_AND_IDE.md",
    "docs/06_v0.6_LOCAL_WEB_AND_INTERACTION.md",
    "docs/07_v0.7_REMOTE_AND_EC2.md",
    "docs/09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md",
    "docs/10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.md",
    "docs/10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md",
    "docs/11_v0.11_SANDBOX_RESOURCE_LIMITS.md",
    "docs/11_v0.11_SANDBOX_RESOURCE_LIMITS.ja.md",
    "docs/11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md",
    "docs/11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md",
    "docs/13_v0.13_LOCAL_OCI_REGISTRY.md",
    "docs/13A_v0.13_OCI_SEED_AND_COW.md",
    "docs/13A_v0.13_OCI_SEED_AND_COW.ja.md",
    "docs/13B_v0.13_SEED_AUTO_PROMOTION.md",
    "docs/13B_v0.13_SEED_AUTO_PROMOTION.ja.md",
    "docs/13C_v0.13_OCI_IMAGE_DELETION.md",
    "docs/13C_v0.13_OCI_IMAGE_DELETION.ja.md",
]
for rel in superseded_files:
    if (root / rel).exists():
        errors.append(f"superseded documentation file still exists: {rel}")

required_files = [
    "docs/README.md",
    "docs/README.ja.md",
    "docs/00_REBASELINE_AND_ROADMAP.md",
    "docs/00D_VERSIONING_AND_RELEASE_STATUS.md",
    "docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md",
    "docs/00C_TERMINOLOGY_AND_BOUNDARIES.md",
    "docs/00B_SECURITY_ARCHITECTURE.md",
    "docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md",
    "docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md",
    "docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md",
    "docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md",
    "docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md",
    "docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md",
    "docs/13_v0.13_MANAGED_SANDBOX_NETWORK.md",
    "docs/13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md",
    "docs/14_v0.14_GIT_FETCH_PLUGIN.md",
    "docs/14_v0.14_GIT_FETCH_PLUGIN.ja.md",
    "docs/15_v0.15_OCI_SEED_RECOMMENDATION.md",
    "docs/15_v0.15_OCI_SEED_RECOMMENDATION.ja.md",
    "docs/16_v0.16_OCI_IMAGE_DELETION.md",
    "docs/16_v0.16_OCI_IMAGE_DELETION.ja.md",
    "docs/17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md",
    "docs/17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md",
    "docs/18_v0.18_LOCAL_OCI_REGISTRY.md",
    "docs/18_v0.18_LOCAL_OCI_REGISTRY.ja.md",
    "docs/19_v0.19_OCI_SEED_AND_COW.md",
    "docs/19_v0.19_OCI_SEED_AND_COW.ja.md",
    "docs/OCI_RUNTIME_AND_DOCKER_COMPAT.md",
    "docs/IMPLEMENTATION_STATUS.md",
    "docs/IMPLEMENTATION_STATUS.ja.md",
    "README.md",
    "README.ja.md",
    "CODEX_START_HERE.md",
    "docs/ARCHITECTURE_GUIDE.ja.md",
]
for rel in required_files:
    if not (root / rel).exists():
        errors.append(f"missing required documentation: {rel}")


def require_text(path, required_items, label):
    text = (root / path).read_text()
    lowered = text.lower()
    for required in required_items:
        if required.lower() not in lowered:
            errors.append(f"{path} missing {label}: {required}")
    return text


versioning = require_text(
    "docs/00D_VERSIONING_AND_RELEASE_STATUS.md",
    [
        "One independently useful product feature",
        "v0.13 | Managed Sandbox Network",
        "v0.14 | Git Fetch Plugin",
        "v0.15 | OCI Seed Recommendation",
        "v0.16 | OCI Image Deletion",
        "v0.17 | Docker Compatibility Plugin",
        "v0.18 | Optional Local OCI Registry",
        "v0.19 | OCI Seed Builder & Btrfs/COW",
        "contiguous through **v0.16**",
        "same PR",
    ],
    "current numbering rule",
)

status = require_text(
    "docs/IMPLEMENTATION_STATUS.md",
    [
        "current code reality",
        "contiguous through **v0.16**",
        "Managed sandbox network",
        "haco plugin git fetch",
        "OCI Seed automatic promotion",
        "haco image delete",
        "Docker compatibility",
        "v0.18",
        "v0.19",
        "real AWS acceptance pending",
    ],
    "current reality",
)

roadmap = require_text(
    "docs/00_REBASELINE_AND_ROADMAP.md",
    [
        "Hacocoon is a **Secure Workspace Runtime**",
        "v0.13 | Managed Sandbox Network",
        "v0.14 | Git Fetch Plugin",
        "v0.15 | OCI Seed Recommendation",
        "v0.16 | OCI Image Deletion",
        "v0.17 | Docker Compatibility Plugin",
        "v0.18 | Optional Local OCI Registry",
        "v0.19 | OCI Seed Builder & Btrfs/COW",
        "experimental and disabled by default",
        "Historical note",
    ],
    "current roadmap contract",
)

docmap = require_text(
    "docs/README.md",
    [
        "Source-of-truth order",
        "Numbering rule",
        "13_v0.13_MANAGED_SANDBOX_NETWORK.md",
        "14_v0.14_GIT_FETCH_PLUGIN.md",
        "15_v0.15_OCI_SEED_RECOMMENDATION.md",
        "16_v0.16_OCI_IMAGE_DELETION.md",
        "17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md",
        "18_v0.18_LOCAL_OCI_REGISTRY.md",
        "19_v0.19_OCI_SEED_AND_COW.md",
        "contiguous through **v0.16**",
    ],
    "documentation map",
)

require_text(
    "docs/13_v0.13_MANAGED_SANDBOX_NETWORK.md",
    ["Managed Sandbox Network", "haco-sandbox0", "haco-sandbox", "fail closed"],
    "v0.13 contract",
)
require_text(
    "docs/14_v0.14_GIT_FETCH_PLUGIN.md",
    ["Git Fetch Plugin", "haco plugin git fetch", "gh auth git-credential", "credentials remain"],
    "v0.14 contract",
)
require_text(
    "docs/15_v0.15_OCI_SEED_RECOMMENDATION.md",
    ["OCI Seed Recommendation", "6 hours", "30 days", "top 10%", "auto_promote"],
    "v0.15 contract",
)
require_text(
    "docs/16_v0.16_OCI_IMAGE_DELETION.md",
    ["OCI Image Deletion", "haco image delete", "tombstone", "recovery-required"],
    "v0.16 contract",
)
require_text(
    "docs/17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md",
    ["Docker Compatibility Plugin", "containerd + nerdctl", "on-demand", "not yet a complete"],
    "v0.17 contract",
)
require_text(
    "docs/18_v0.18_LOCAL_OCI_REGISTRY.md",
    ["Optional Local OCI Registry", "planned", "direct", "not required"],
    "v0.18 contract",
)
require_text(
    "docs/19_v0.19_OCI_SEED_AND_COW.md",
    ["OCI Seed Builder", "planned", "Offline Seed Builder", "/var/lib/containerd", "Btrfs/COW"],
    "v0.19 contract",
)

require_text(
    "docs/OCI_RUNTIME_AND_DOCKER_COMPAT.md",
    ["v0.17", "plugin/adapter", "containerd + nerdctl", "v0.19", "Environment-local"],
    "Docker compatibility boundary",
)

reference = (root / "docs/91_IMPLEMENTATION_REFERENCE_NOTES.md").read_text()
if "non-normative" not in reference.lower() or "No current architecture contract commits" not in reference:
    errors.append("reference notes must remain explicitly non-normative")

require_text(
    "README.md",
    [
        "docs/README.md",
        "pre-1.0",
        "haco-vscode open",
        "v0.9",
        "v0.10",
        "haco-agent-host",
        "v0.11",
        "haco image list",
        "v0.12",
    ],
    "top-level README baseline",
)

require_text(
    "README.ja.md",
    ["読み方: はこーん", "pre-1.0", "Breaking Change", "IMPLEMENTATION_STATUS.ja.md"],
    "Japanese README baseline",
)

if errors:
    print("DOC CONSISTENCY FAILED")
    print("\n".join(errors))
    sys.exit(1)

print("DOC CONSISTENCY OK")

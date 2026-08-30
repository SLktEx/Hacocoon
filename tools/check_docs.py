#!/usr/bin/env python3
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parents[1]
markdown_files = [p for p in root.rglob("*.md") if p.name != "Hacocoon_v0.1-v0.7_MASTER.md"]
errors: list[str] = []


def require_file(rel: str) -> Path:
    path = root / rel
    if not path.exists():
        errors.append(f"missing required file: {rel}")
    return path


def require_text(rel: str, required: list[str]) -> str:
    path = require_file(rel)
    if not path.exists():
        return ""
    text = path.read_text()
    lowered = text.lower()
    for item in required:
        if item.lower() not in lowered:
            errors.append(f"{rel} missing required text: {item}")
    return text


# Core documentation set and current milestone contracts.
required_files = [
    "README.md",
    "README.ja.md",
    "docs/README.md",
    "docs/README.ja.md",
    "docs/00A_PLUGIN_ARCHITECTURE.md",
    "docs/00B_SECURITY_ARCHITECTURE.md",
    "docs/00C_TERMINOLOGY_AND_BOUNDARIES.md",
    "docs/00D_VERSIONING_AND_RELEASE_STATUS.md",
    "docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md",
    "docs/00_REBASELINE_AND_ROADMAP.md",
    "docs/IMPLEMENTATION_STATUS.md",
    "docs/IMPLEMENTATION_STATUS.ja.md",
    "docs/ARCHITECTURE_GUIDE.ja.md",
    "docs/GIT_GITHUB_CAPABILITY.md",
    "docs/OCI_RUNTIME_AND_DOCKER_COMPAT.md",
    "docs/OCI_RUNTIME_AND_DOCKER_COMPAT.ja.md",
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
    "docs/17_v0.17_DOCKER_COMPATIBILITY.md",
    "docs/17_v0.17_DOCKER_COMPATIBILITY.ja.md",
    "docs/18_v0.18_OPTIONAL_LOCAL_OCI_REGISTRY.md",
    "docs/18_v0.18_OPTIONAL_LOCAL_OCI_REGISTRY.ja.md",
    "docs/19_v0.19_OCI_SEED_AND_COW.md",
    "docs/19_v0.19_OCI_SEED_AND_COW.ja.md",
    "cmd/haco/plugin_oci.go",
    "modules/plugin/oci/service.go",
    "modules/plugin/oci/packaging/systemd/hacocoon-docker.service",
    "modules/plugin/oci/packaging/systemd/hacocoon-docker.socket",
]
for rel in required_files:
    require_file(rel)

# Superseded v0.13A/B/C numbering must not return as active files.
superseded_files = [
    "docs/13_v0.13_LOCAL_OCI_REGISTRY.md",
    "docs/13A_v0.13_OCI_SEED_AND_COW.md",
    "docs/13A_v0.13_OCI_SEED_AND_COW.ja.md",
    "docs/13B_v0.13_SEED_AUTO_PROMOTION.md",
    "docs/13B_v0.13_SEED_AUTO_PROMOTION.ja.md",
    "docs/13C_v0.13_OCI_IMAGE_DELETION.md",
    "docs/13C_v0.13_OCI_IMAGE_DELETION.ja.md",
    "internal/seedstats/delete.go",
    "internal/seedstats/service.go",
    "internal/seedstats/store.go",
    "packaging/systemd/hacocoon-docker.service",
    "packaging/systemd/hacocoon-docker.socket",
]
for rel in superseded_files:
    if (root / rel).exists():
        errors.append(f"superseded file still exists: {rel}")

versioning = require_text(
    "docs/00D_VERSIONING_AND_RELEASE_STATUS.md",
    [
        "v0.13 | Managed Sandbox Network",
        "v0.14 | Git Fetch Plugin",
        "v0.15 | OCI Seed Usage & Recommendation",
        "v0.16 | OCI Image Deletion",
        "v0.17 | Docker Compatibility",
        "v0.18 | Optional Local OCI Registry",
        "v0.19 | OCI Seed Builder & Btrfs/COW",
        "contiguous through **v0.17**",
        "HACO_PLUGIN_OCI=nerdctl",
        "haco plugin oci seed recommend",
        "haco plugin oci image delete",
    ],
)
if "v0.13 | local oci registry" in versioning.lower():
    errors.append("versioning still assigns Local OCI Registry to v0.13")

status = require_text(
    "docs/IMPLEMENTATION_STATUS.md",
    [
        "current code reality",
        "v0.13",
        "Managed sandbox network",
        "v0.14",
        "haco plugin git fetch",
        "gh auth git-credential",
        "v0.15",
        "haco plugin oci seed sample|recommend",
        "v0.16",
        "haco plugin oci image delete",
        "v0.17",
        "Docker compatibility",
        "v0.18",
        "planned",
        "v0.19",
    ],
)
if "contiguous through **v0.12**" in status.lower():
    errors.append("IMPLEMENTATION_STATUS still claims implementation stops at v0.12")

require_text(
    "docs/00_REBASELINE_AND_ROADMAP.md",
    [
        "Hacocoon is a **Secure Workspace Runtime**",
        "optional feature surfaces",
        "haco plugin git fetch",
        "HACO_PLUGIN_OCI=nerdctl",
        "v0.13 | Managed Sandbox Network",
        "v0.17 | Docker Compatibility",
        "v0.18 | Optional Local OCI Registry",
        "v0.19 | OCI Seed Builder & Btrfs/COW",
        "Browser/Web notification",
    ],
)

require_text(
    "docs/00A_PLUGIN_ARCHITECTURE.md",
    [
        "haco plugin git",
        "haco plugin oci",
        "HACO_PLUGIN_OCI=nerdctl",
        "not require nerdctl",
        "dynamic loading",
    ],
)

require_text(
    "docs/GIT_GITHUB_CAPABILITY.md",
    [
        "haco plugin git fetch",
        "haco plugin git push",
        "gh auth git-credential",
        "fixed refspec",
        "Tags and submodules are not implicitly fetched",
    ],
)

require_text(
    "docs/OCI_RUNTIME_AND_DOCKER_COMPAT.md",
    [
        "optional OCI plugin",
        "HACO_PLUGIN_OCI",
        "Core must not require nerdctl",
        "genuine Docker CLI",
        "modules/plugin/oci/packaging/systemd/",
        "never mount the Host Docker socket",
    ],
)

require_text(
    "docs/README.md",
    [
        "v0.13 | Managed Sandbox Network",
        "v0.14 | Git Fetch Plugin",
        "v0.15 | OCI Seed Usage & Recommendation",
        "v0.16 | OCI Image Deletion",
        "v0.17 | Docker Compatibility",
        "v0.18 | Optional Local OCI Registry",
        "v0.19 | OCI Seed Builder & Btrfs/COW",
        "haco plugin oci",
    ],
)

# Prevent old workload-OCI commands or old "canonical OCI runtime" claims from
# becoming active documentation again. Historical commit messages are not part
# of the Markdown source set, so these can be strict.
stale_patterns = [
    (re.compile(r"\bhaco image seed (?:sample|recommend)\b"), "workload OCI seed command escaped plugin namespace"),
    (re.compile(r"\bhaco image delete\b"), "workload OCI delete command escaped plugin namespace"),
    (re.compile(r"Hacocoon(?:'s|の) (?:canonical|標準) OCI runtime", re.IGNORECASE), "OCI runtime incorrectly described as Core canonical runtime"),
    (re.compile(r"\]\(13_v0\.13_LOCAL_OCI_REGISTRY\.md\)"), "link to superseded v0.13 registry document"),
    (re.compile(r"\]\(13A_v0\.13_OCI_SEED_AND_COW(?:\.ja)?\.md\)"), "link to superseded v0.13A document"),
    (re.compile(r"\]\(13B_v0\.13_SEED_AUTO_PROMOTION(?:\.ja)?\.md\)"), "link to superseded v0.13B document"),
    (re.compile(r"\]\(13C_v0\.13_OCI_IMAGE_DELETION(?:\.ja)?\.md\)"), "link to superseded v0.13C document"),
]
for path in markdown_files:
    text = path.read_text()
    for pattern, label in stale_patterns:
        for match in pattern.finditer(text):
            line = text[: match.start()].count("\n") + 1
            errors.append(f"{path.relative_to(root)}:{line}: {label}")

# Retain older architecture rebaseline guards that should never reappear.
legacy_patterns = [
    (r"v0\.1 Local Foundation|v0\.2 Developer Workspace|v0\.3 Security Framework|v0\.4 External Capabilities|v0\.5 Local GUI|v0\.6 Local Web", "old release ordering"),
    (r"\bHacocoon IAM\b", "legacy Hacocoon IAM term"),
    (r"Manager/Session trust boundary|Manager / Session trust boundary", "legacy Manager/Session boundary"),
]
for path in markdown_files:
    text = path.read_text()
    for pattern, label in legacy_patterns:
        for match in re.finditer(pattern, text, flags=re.IGNORECASE):
            line = text[: match.start()].count("\n") + 1
            snippet = text.splitlines()[line - 1]
            if label == "legacy Hacocoon IAM term" and any(word in snippet.lower() for word in ("historical", "legacy", "do not")):
                continue
            errors.append(f"{path.relative_to(root)}:{line}: {label}: {snippet.strip()}")

if errors:
    print("DOC CONSISTENCY FAILED")
    print("\n".join(errors))
    sys.exit(1)

print("DOC CONSISTENCY OK")

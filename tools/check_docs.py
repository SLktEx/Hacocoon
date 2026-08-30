#!/usr/bin/env python3
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parents[1]
docs_root = root / "docs"
markdown_files = list(root.rglob("*.md"))
errors = []

# Long-lived documentation addresses are semantic. ADR sequence numbers are identity.
for p in docs_root.rglob("*.md"):
    rel = p.relative_to(docs_root)
    if rel.parts and rel.parts[0] == "adr":
        continue
    if re.match(r"^\d{2}[A-Z]?_", p.name, flags=re.IGNORECASE):
        errors.append(f"{p.relative_to(root)}: numbered/order-prefixed documentation filename is prohibited")
    if re.search(r"(?:^|[_-])v\d+\.\d+(?:[_-]|$)", p.name, flags=re.IGNORECASE):
        errors.append(f"{p.relative_to(root)}: version-encoded documentation filename is prohibited")

legacy_versioned_name = re.compile(r"\b\d{2}[A-Z]?_v0\.\d+_[A-Z0-9_]+(?:\.ja)?\.md\b", re.IGNORECASE)
legacy_ordered_name = re.compile(r"\b(?:00[A-Z]?|90|91)_[A-Z0-9_]+(?:\.ja)?\.md\b", re.IGNORECASE)
md_link = re.compile(r"\[[^\]]*\]\(([^)]+)\)")

stale_claims = [
    (r"\|\s*v0\.(?:13|18)\s*\|\s*(?:Optional )?Local OCI Registry\s*\|", "stale Local Registry milestone assignment"),
    (r"\|\s*v0\.19\s*\|\s*OCI Seed Builder", "stale Seed Builder milestone assignment"),
    (r"implemented milestones are contiguous through \*\*v0\.12\*\*", "stale milestone ceiling"),
    (r"EC2 remains experimental and disabled by default", "stale active EC2 claim"),
    (r"\|\s*EC2 Environment provider\s*\|\s*experimental", "stale active EC2 provider claim"),
]

for p in markdown_files:
    text = p.read_text()
    for pattern, label in (
        (legacy_versioned_name, "legacy versioned documentation address"),
        (legacy_ordered_name, "legacy ordered documentation address"),
    ):
        for match in pattern.finditer(text):
            line = text[:match.start()].count("\n") + 1
            errors.append(f"{p.relative_to(root)}:{line}: {label}: {match.group(0)}")

    for pattern, label in stale_claims:
        for match in re.finditer(pattern, text, flags=re.IGNORECASE):
            line = text[:match.start()].count("\n") + 1
            errors.append(f"{p.relative_to(root)}:{line}: {label}")

    # Repository-relative Markdown document links must survive document moves.
    for match in md_link.finditer(text):
        raw = match.group(1).strip()
        if not raw or raw.startswith(("http://", "https://", "mailto:", "#")):
            continue
        target = raw.split("#", 1)[0].split("?", 1)[0]
        if not target.lower().endswith(".md"):
            continue
        resolved = (p.parent / target).resolve()
        try:
            resolved.relative_to(root.resolve())
        except ValueError:
            line = text[:match.start()].count("\n") + 1
            errors.append(f"{p.relative_to(root)}:{line}: Markdown link escapes repository: {raw}")
            continue
        if not resolved.is_file():
            line = text[:match.start()].count("\n") + 1
            errors.append(f"{p.relative_to(root)}:{line}: broken Markdown link: {raw}")

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
    "docs/README.md", "docs/README.ja.md", "docs/DOCUMENTATION_STYLE_GUIDE.md",
    "docs/DESIGN_PRINCIPLES.md", "docs/DESIGN_PRINCIPLES.ja.md",
    "docs/IMPLEMENTATION_STATUS.md", "docs/IMPLEMENTATION_STATUS.ja.md",
    "docs/CLIENT_ADAPTER_CONTRACT.md", "docs/CLIENT_ADAPTER_CONTRACT.ja.md",
    "docs/design/plugin-architecture.md",
    "docs/security/security-architecture.md",
    "docs/reference/terminology-and-boundaries.md",
    "docs/status/architecture-and-roadmap.md",
    "docs/status/versioning-and-release-status.md", "docs/status/versioning-and-release-status.ja.md",
    "docs/design/secure-workspace-runtime.md",
    "docs/design/workspace-abstraction-and-lease.md",
    "docs/design/client-and-interactive-access.md",
    "docs/design/policy-and-capability-foundation.md",
    "docs/design/git-and-github-capability.md",
    "docs/design/agent-and-orchestrator-integration.md",
    "docs/design/remote-and-cloud-runtime.md",
    "docs/design/client-adapters-and-vscode-integration.md",
    "docs/design/per-agent-sandbox-and-agent-host.md", "docs/design/per-agent-sandbox-and-agent-host.ja.md",
    "docs/design/vscode-remote-agent-host-adapter.md", "docs/design/vscode-remote-agent-host-adapter.ja.md",
    "docs/design/base-images-and-custom-environments.md",
    "docs/design/sandbox-resource-limits.md", "docs/design/sandbox-resource-limits.ja.md",
    "docs/design/managed-sandbox-network.md", "docs/design/managed-sandbox-network.ja.md",
    "docs/design/git-fetch-plugin.md", "docs/design/git-fetch-plugin.ja.md",
    "docs/design/oci-seed-recommendation.md", "docs/design/oci-seed-recommendation.ja.md",
    "docs/design/oci-image-deletion.md", "docs/design/oci-image-deletion.ja.md",
    "docs/design/oci-seed-and-cow.md", "docs/design/oci-seed-and-cow.ja.md",
    "docs/design/docker-compatibility-plugin.md", "docs/design/docker-compatibility-plugin.ja.md",
    "docs/OPTIONAL_LOCAL_OCI_REGISTRY.md", "docs/OPTIONAL_LOCAL_OCI_REGISTRY.ja.md",
]
for rel in required:
    if not (root / rel).exists():
        errors.append(f"missing required documentation: {rel}")


def require_text(path, items):
    p = root / path
    if not p.is_file():
        return
    text = p.read_text().lower()
    for item in items:
        if item.lower() not in text:
            errors.append(f"{path} missing required text: {item}")

require_text("AGENTS.md", [
    "docs/DOCUMENTATION_STYLE_GUIDE.md", "docs/status/versioning-and-release-status.md",
    "docs/reference/terminology-and-boundaries.md", "docs/security/security-architecture.md",
    "ADR sequence numbers", "bash tools/ci-local.sh",
])
require_text("docs/DOCUMENTATION_STYLE_GUIDE.md", [
    "stable semantic paths", "ADR sequence numbers", "status/versioning-and-release-status.md",
    "reference/terminology-and-boundaries.md", "security/security-architecture.md",
])
require_text("docs/status/versioning-and-release-status.md", [
    "One independently useful product feature", "v0.17 | OCI Seed Builder & Btrfs/COW",
    "v0.18 | Docker Compatibility Plugin", "contiguous through **v0.16**",
    "Local Registry infrastructure is deferred and unversioned",
    "cloud implementation is currently deferred",
])
require_text("docs/IMPLEMENTATION_STATUS.md", [
    "current code reality", "contiguous through **v0.16**", "pkg/clientadapter", "haco ssh",
    "haco plugin oci seed build", "haco plugin oci seed current",
    "v0.17 partial", "v0.18 implemented", "haco plugin oci docker status",
    "HACO_PLUGIN_OCI=nerdctl|docker", "cloud implementation is currently deferred",
])
require_text("docs/status/architecture-and-roadmap.md", [
    "Hacocoon is a **Secure Workspace Runtime**", "Core", "Standard", "Plugin",
    "v0.17 | OCI Seed Builder & Btrfs/COW", "v0.18 | Docker Compatibility Plugin",
    "contiguous through **v0.16**", "Local OCI Registry is not a roadmap milestone",
])
require_text("docs/README.md", [
    "Documentation layout", "Source-of-truth order", "CLIENT_ADAPTER_CONTRACT.md",
    "pkg/clientadapter", "design/oci-seed-and-cow.md", "design/docker-compatibility-plugin.md",
    "contiguous through **v0.16**", "Core", "Standard", "Plugin",
])
require_text("docs/CLIENT_ADAPTER_CONTRACT.md", [
    "pkg/clientadapter", "public-key", "private key", "loopback-only", "/workspace",
    "haco ssh", "pkg/interaction", "VS Code", "JetBrains", "code-server",
])
require_text("docs/design/plugin-architecture.md", [
    "Core / Standard / Plugin classification", "HACO_PLUGIN_OCI=nerdctl",
    "HACO_PLUGIN_OCI=docker", "unset HACO_PLUGIN_OCI", "haco base",
])
require_text("docs/design/oci-seed-and-cow.md", [
    "OCI Seed Builder", "haco plugin oci seed build", "haco plugin oci seed current",
    "/var/lib/containerd", "Btrfs/COW",
])
require_text("docs/design/docker-compatibility-plugin.md", [
    "Docker Compatibility Plugin", "haco plugin oci docker status",
    "haco plugin oci docker prepare", "fail closed",
])
require_text("README.md", ["pre-1.0", "haco base list", "haco plugin oci", "pkg/clientadapter"])
require_text("README.ja.md", ["読み方: はこーん", "pre-1.0", "haco base list", "haco plugin oci", "pkg/clientadapter"])

if errors:
    print("DOC CONSISTENCY FAILED")
    print("\n".join(errors))
    sys.exit(1)
print("DOC CONSISTENCY OK")

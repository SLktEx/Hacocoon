#!/usr/bin/env python3
from pathlib import Path
import re
import sys

from checkpoint_source import CheckpointSourceError, parse_checkpoint_source

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
    (r"haco-storage-helper", "removed Host-managed storage helper reference"),
    (r"HACO_STORAGE_PRIVILEGE_MODE", "removed Host-managed storage mode reference"),
    (r"HACO_BLOCK_BACKEND", "removed Host-managed block backend reference"),
    (r"Managed Btrfs Host Privilege Broker", "removed Host-managed storage milestone name"),
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
    "docs/INTERACTION_EVENTS.md", "docs/INTERACTION_EVENTS.ja.md",
    "docs/EGRESS_AUTHORIZATION.md", "docs/EGRESS_AUTHORIZATION.ja.md",
    "docs/design/plugin-architecture.md",
    "docs/design/trusted-host.md", "docs/design/trusted-host.ja.md",
    "docs/security/security-architecture.md",
    "docs/reference/terminology-and-boundaries.md",
    "docs/reference/build-release-identity.md", "docs/reference/build-release-identity.ja.md",
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
    "docs/design/btrfs-storage-layout.md", "docs/design/btrfs-storage-layout.ja.md",
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


def extract_checkpoint(path, pattern):
    p = root / path
    if not p.is_file():
        return None
    text = p.read_text()
    matches = re.findall(pattern, text, flags=re.IGNORECASE)
    if len(matches) != 1:
        errors.append(f"{path}: expected exactly one current milestone declaration, found {len(matches)}")
        return None
    return matches[0].lower()


# docs/status/checkpoints.yaml is the machine-readable source of truth for checkpoint
# numbering, the current checkpoint, and gate identity. Markdown documents and the
# generated build input are mirrors with richer human-facing status/acceptance prose.
checkpoint_source = None
checkpoint_source_path = root / "docs/status/checkpoints.yaml"
try:
    checkpoint_source = parse_checkpoint_source(checkpoint_source_path)
except CheckpointSourceError as exc:
    errors.append(f"docs/status/checkpoints.yaml: {exc}")

checkpoint_mirrors = {
    "docs/status/versioning-and-release-status.md": r"current milestone position is \*\*(v0\.\d+)\*\*",
    "docs/status/versioning-and-release-status.ja.md": r"現在のmilestone位置は\s*\*\*(v0\.\d+)\*\*",
    "docs/IMPLEMENTATION_STATUS.md": r"current milestone position is \*\*(v0\.\d+)\*\*",
    "docs/IMPLEMENTATION_STATUS.ja.md": r"現在のmilestone位置は\s*\*\*(v0\.\d+)\*\*",
}
for path, pattern in checkpoint_mirrors.items():
    value = extract_checkpoint(path, pattern)
    if checkpoint_source is not None and value is not None and value != checkpoint_source.current.lower():
        errors.append(
            f"{path}: current milestone mirror {value} disagrees with "
            f"docs/status/checkpoints.yaml current {checkpoint_source.current}"
        )

# English/Japanese status tables must mirror the ordered version/gate mapping from
# YAML exactly. Their third status column remains intentionally human-maintained.
version_table_paths = [
    "docs/status/versioning-and-release-status.md",
    "docs/status/versioning-and-release-status.ja.md",
]
for path in version_table_paths:
    p = root / path
    if not p.is_file():
        continue
    rows = re.findall(
        r"^\|\s*(v0\.\d+)\s*\|\s*([^|]+?)\s*\|",
        p.read_text(),
        flags=re.IGNORECASE | re.MULTILINE,
    )
    if not rows:
        errors.append(f"{path}: no checkpoint rows found in version table")
        continue
    if checkpoint_source is None:
        continue
    expected = [(item.version.lower(), item.gate) for item in checkpoint_source.milestones]
    actual = [(version.lower(), gate.strip()) for version, gate in rows]
    if actual != expected:
        mismatch = None
        for index in range(max(len(actual), len(expected))):
            got = actual[index] if index < len(actual) else None
            want = expected[index] if index < len(expected) else None
            if got != want:
                mismatch = (index + 1, got, want)
                break
        errors.append(
            f"{path}: checkpoint table does not mirror docs/status/checkpoints.yaml; "
            f"first mismatch at row {mismatch[0]}: got {mismatch[1]!r}, expected {mismatch[2]!r}"
        )

# The build-time generated checkpoint must be a mirror of the same YAML source.
generated_checkpoint_path = root / "internal/buildinfo/checkpoint_generated.go"
if not generated_checkpoint_path.is_file():
    errors.append("missing generated checkpoint input: internal/buildinfo/checkpoint_generated.go")
elif checkpoint_source is not None:
    generated_matches = re.findall(
        r'const GeneratedCheckpoint = "(v0\.\d+)"',
        generated_checkpoint_path.read_text(),
    )
    if len(generated_matches) != 1:
        errors.append("internal/buildinfo/checkpoint_generated.go: expected exactly one GeneratedCheckpoint")
    elif generated_matches[0].lower() != checkpoint_source.current.lower():
        errors.append(
            "internal/buildinfo/checkpoint_generated.go: generated checkpoint "
            f"{generated_matches[0]} disagrees with docs/status/checkpoints.yaml current "
            f"{checkpoint_source.current}"
        )

# Intro/index/roadmap documents link to the authorities instead of copying concrete
# checkpoint numbers. This is the main drift-prevention rule for fast v0.N progress.
checkpoint_copy_free = [
    "README.md",
    "README.ja.md",
    "docs/README.md",
    "docs/README.ja.md",
    "docs/status/architecture-and-roadmap.md",
]
concrete_checkpoint = re.compile(r"\bv0\.\d+\b", re.IGNORECASE)
for path in checkpoint_copy_free:
    p = root / path
    if not p.is_file():
        continue
    if concrete_checkpoint.search(p.read_text()):
        errors.append(f"{path}: concrete checkpoint numbers belong in version/status authorities; link instead")

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
    "Minor milestones are lightweight pre-1.0 progress checkpoints",
    "Interaction Notification Clients", "Real Incus E2E Acceptance", "Structured Logging",
    "Incus-owned Btrfs Storage Acceptance", "Trusted `haco-host` & Default WSL Entry",
    "Local Registry infrastructure is deferred and unversioned", "cloud implementation is currently deferred",
    "Tags/releases are separate",
])
require_text("docs/status/versioning-and-release-status.ja.md", [
    "Interaction Notification Clients", "Real Incus E2E Acceptance", "Structured Logging",
    "Incus-owned Btrfs Storage Acceptance", "Trusted `haco-host` & Default WSL Entry",
    "release tagとroadmap milestone番号は別物",
])
require_text("docs/reference/build-release-identity.md", [
    "status/checkpoints.yaml", "tools/bump-milestone", "machine-readable", "generated build input",
])
require_text("docs/reference/build-release-identity.ja.md", [
    "status/checkpoints.yaml", "tools/bump-milestone", "machine-readable", "generated build input",
])
require_text("docs/IMPLEMENTATION_STATUS.md", [
    "current code reality", "pkg/clientadapter", "haco ssh", "haco plugin oci seed build",
    "haco-notify", "Incus-owned Btrfs", "haco setup", "HACO_PLUGIN_OCI=nerdctl|docker",
    "design/btrfs-storage-layout.md", "design/trusted-host.md", "compress=zstd:3",
    "cloud implementation is currently deferred",
])
require_text("docs/IMPLEMENTATION_STATUS.ja.md", [
    "haco-notify", "Incus-owned Btrfs", "haco setup", "design/trusted-host.ja.md",
])
require_text("docs/status/architecture-and-roadmap.md", [
    "Hacocoon is a **Secure Workspace Runtime**", "Core", "Standard", "Plugin",
    "does **not** duplicate the current checkpoint table", "Roadmap model", "Trusted Host direction",
    "Client direction", "Operational confidence direction", "Local OCI Registry is not a required roadmap gate",
])
require_text("docs/README.md", [
    "Documentation layout", "Source-of-truth order", "Current checkpoint", "CLIENT_ADAPTER_CONTRACT.md",
    "pkg/clientadapter", "design/trusted-host.md", "design/oci-seed-and-cow.md",
    "design/docker-compatibility-plugin.md", "EGRESS_AUTHORIZATION.md", "INTERACTION_EVENTS.md",
    "reference/logging.md", "design/btrfs-storage-layout.md", "Core", "Standard", "Plugin", "haco-notify",
])
require_text("docs/README.ja.md", [
    "現在のcheckpoint", "haco-notify", "haco-host", "status/versioning-and-release-status.ja.md",
])
require_text("docs/design/trusted-host.md", [
    "haco-host", "Physical Host", "haco setup", "hacoq host shell",
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
require_text("docs/EGRESS_AUTHORIZATION.md", [
    "Domain-aware egress authorization", "network.egress/connect", "haco egress serve", "SNI",
])
require_text("docs/design/btrfs-storage-layout.md", [
    "Incus-owned loop-backed Btrfs", "Managed Btrfs Transparent Compression",
    "haco-local-default", "Environment rootfs", "compress=zstd:3", "compress-force", "autodefrag",
])
require_text("docs/INTERACTION_EVENTS.md", ["browser", "native", "VS Code", "notification"])
require_text("README.md", [
    "pre-1.0", "haco base list", "haco plugin oci", "pkg/clientadapter", "haco-notify", "haco-host",
    "Versioning and release status",
])
require_text("README.ja.md", [
    "読み方: はこーん", "pre-1.0", "haco base list", "haco plugin oci", "pkg/clientadapter",
    "haco-notify", "haco-host", "Versioning / Release status",
])

if errors:
    print("DOC CONSISTENCY FAILED")
    print("\n".join(errors))
    sys.exit(1)
print("DOC CONSISTENCY OK")

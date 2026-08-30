#!/usr/bin/env python3
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parents[1]
files = [p for p in root.rglob("*.md") if p.name != "Hacocoon_v0.1-v0.7_MASTER.md"]

checks = [
    (r"v0\.1 Local Foundation|v0\.2 Developer Workspace|v0\.3 Security Framework|v0\.4 External Capabilities|v0\.5 Local GUI|v0\.6 Local Web", "old release ordering"),
    (r"01_v0\.1_LOCAL_FOUNDATION|02_v0\.2_DEVELOPER_WORKSPACE|03_v0\.3_SECURITY_FRAMEWORK_AND_GIT|04_v0\.4_EXTERNAL_CAPABILITIES|05_v0\.5_LOCAL_GUI_AND_IDE|06_v0\.6_LOCAL_WEB_AND_INTERACTION|07_v0\.7_REMOTE_AND_EC2", "superseded release filename"),
    (r"11_v0\.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER|#\s*v0\.11\s+VS Code Remote Agent Host Adapter", "stale v0.11 Agent Host adapter assignment"),
    (r"\bHacocoon IAM\b", "legacy Hacocoon IAM term"),
    (r"Manager/Session trust boundary|Manager / Session trust boundary", "legacy Manager/Session architecture boundary"),
    (r"Runtime/Storage seams|Security and Feature Plugin boundaries", "pre-rebaseline ADR terminology"),
    (r"\bDirectoryWorkspace\b", "redundant workspace-provider name"),
    (r"Status:\s*\*\*current implementation gate\*\*", "stale v0.1-as-current status"),
    (r"v0\.10 is the active VS Code Remote Agent Host Adapter integration candidate", "stale v0.10 unmerged status"),
    (r"v0\.11 Base Images & Custom Environments and v0\.12 Sandbox Resource Limits remain design-only", "stale v0.11/v0.12 design-only status"),
    (r"active PR #111;\s*not yet on `main`", "stale PR #111 current-status claim"),
    (r"v0\.10 is the trusted adapter layer currently developed in PR #111", "stale PR #111 current-status claim"),
    (r"This gate is \*\*not implemented on `main` until PR #111 is merged\*\*", "stale v0.10 not-implemented claim"),
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
    "docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md",
    "docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md",
    "docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md",
    "docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md",
    "docs/BASE_IMAGES.md",
    "docs/BASE_IMAGES.ja.md",
    "docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md",
    "docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md",
    "docs/13_v0.13_LOCAL_OCI_REGISTRY.md",
    "docs/13A_v0.13_OCI_SEED_AND_COW.md",
    "docs/13A_v0.13_OCI_SEED_AND_COW.ja.md",
    "docs/13B_v0.13_SEED_AUTO_PROMOTION.md",
    "docs/13B_v0.13_SEED_AUTO_PROMOTION.ja.md",
    "docs/13C_v0.13_OCI_IMAGE_DELETION.md",
    "docs/13C_v0.13_OCI_IMAGE_DELETION.ja.md",
    "docs/IMPLEMENTATION_STATUS.md",
    "docs/IMPLEMENTATION_STATUS.ja.md",
    "README.md",
    "README.ja.md",
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


docmap = require_text(
    "docs/README.md",
    [
        "Source-of-truth order",
        "Specification vs implementation",
        "pre-1.0",
        "Current code reality",
        "IMPLEMENTATION_STATUS.md",
        "00D_VERSIONING_AND_RELEASE_STATUS.md",
        "09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md",
        "10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md",
        "11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md",
        "12_v0.12_SANDBOX_RESOURCE_LIMITS.md",
        "13_v0.13_LOCAL_OCI_REGISTRY.md",
        "13A_v0.13_OCI_SEED_AND_COW.md",
        "haco-agent-host",
        "haco base list",
        "haco plugin oci",
        "v0.13",
    ],
    "current documentation-map content",
)

versioning = require_text(
    "docs/00D_VERSIONING_AND_RELEASE_STATUS.md",
    [
        "v0.8 | Client Adapters & VS Code Integration",
        "v0.9 | Per-Agent Sandbox & Agent Host Integration",
        "v0.10 | VS Code Remote Agent Host Adapter",
        "v0.11 | Base Images & Custom Environments",
        "v0.12 | Sandbox Resource Limits",
        "v0.13 | Local OCI Registry",
        "implemented progression is therefore contiguous through **v0.12**",
        "v0.13 is the next planned milestone",
    ],
    "current numbering rule",
)

roadmap = require_text(
    "docs/00_REBASELINE_AND_ROADMAP.md",
    [
        "Hacocoon is a **Secure Workspace Runtime**",
        "Project status at a glance",
        "Current code reality",
        "IMPLEMENTATION_STATUS.md",
        "v0.9 | Per-Agent Sandbox & Agent Host Integration",
        "v0.10 | VS Code Remote Agent Host Adapter",
        "v0.11 | Base Images & Custom Environments",
        "v0.12 | Sandbox Resource Limits",
        "experimental and disabled by default",
        "Historical note",
        "pre-1.0",
    ],
    "current contract text",
)

status = require_text(
    "docs/IMPLEMENTATION_STATUS.md",
    [
        "current code reality",
        "`haco create --workspace`",
        "v0.9 |",
        "Per-agent sandbox broker",
        "internal/agenthost",
        "agent-bindings.json",
        "v0.10 |",
        "haco-agent-host",
        "v0.11 |",
        "haco base list",
        "haco create --base",
        "HACO_INCUS_BASES_JSON",
        "v0.12 |",
        "Resource budget model",
        "--cpu",
        "Incus resource enforcement",
        "haco plugin oci seed recommend",
        "haco plugin oci image delete",
        "real AWS acceptance pending",
    ],
    "current reality",
)

v01 = require_text(
    "docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md",
    ["haco create --workspace", "haco exec", "haco shell", "haco delete", "Incus"],
    "required runtime gate text",
)

v08 = require_text(
    "docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md",
    ["Client Adapter", "haco-vscode", "Remote-SSH", "loopback-only", "Windows + WSL"],
    "required client-adapter contract text",
)

v09 = require_text(
    "docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md",
    [
        "Per-Agent Sandbox",
        "per-session broker",
        "Agent Host",
        "AHP",
        "WorkspaceLease",
        "persisted binding proof",
        "v0.11 Base Images",
    ],
    "required per-agent contract text",
)

v10 = require_text(
    "docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md",
    [
        "VS Code Remote Agent Host Adapter",
        "haco-agent-host",
        "loopback",
        "opaque session",
        "private key",
        "code --agents",
    ],
    "required Agent Host adapter content",
)

v11 = require_text(
    "docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md",
    [
        "Base Images & Custom Environments",
        "first implementation slice",
        "immutable Base revision",
        "Incus image fingerprint",
        "haco base list",
        "haco create --base",
        "HACO_INCUS_BASES_JSON",
        "referenced Base revision",
    ],
    "required Base-image contract text",
)

v12 = require_text(
    "docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md",
    [
        "Sandbox Resource Limits",
        "ResourceBudget",
        "CPU",
        "memory",
        "process/PID",
        "root-storage",
        "fail closed",
        "v0.11 Bases",
        "v0.9 agent-session binding",
    ],
    "required resource-budget contract text",
)

v13 = require_text(
    "docs/13_v0.13_LOCAL_OCI_REGISTRY.md",
    [
        "Local OCI Registry",
        "containerd",
        "nerdctl",
    ],
    "required v0.13 registry contract text",
)

v13a = require_text(
    "docs/13A_v0.13_OCI_SEED_AND_COW.md",
    [
        "OCI Seed",
        "usage telemetry/recommendation first slice implemented",
        "Seed build/publish remains planned",
        "haco plugin oci seed recommend",
        "Local Registry",
        "Btrfs",
        "/var/lib/containerd",
    ],
    "required v0.13A contract text",
)

v13b = require_text(
    "docs/13B_v0.13_SEED_AUTO_PROMOTION.md",
    ["top **10%**", "auto_promote", "haco plugin oci seed recommend"],
    "required v0.13B auto-promotion contract text",
)

v13c = require_text(
    "docs/13C_v0.13_OCI_IMAGE_DELETION.md",
    ["haco plugin oci image delete", "--all-environments", "deletion tombstone", "nerdctl rmi"],
    "required v0.13C deletion contract text",
)

reference = (root / "docs/91_IMPLEMENTATION_REFERENCE_NOTES.md").read_text()
if "non-normative" not in reference.lower() or "No current architecture contract commits" not in reference:
    errors.append("reference notes must remain explicitly non-normative")

readme = require_text(
    "README.md",
    [
        "docs/README.md",
        "pre-1.0",
        "haco-vscode open",
        "v0.9",
        "09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md",
        "v0.10",
        "10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md",
        "haco-agent-host",
        "v0.11",
        "haco base list",
        "haco create --base",
        "haco plugin oci",
        "11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md",
        "v0.12",
        "12_v0.12_SANDBOX_RESOURCE_LIMITS.md",
    ],
    "current roadmap entry-point content",
)

ja_readme = require_text(
    "README.ja.md",
    [
        "読み方: はこーん",
        "pre-1.0",
        "Breaking Change",
        "IMPLEMENTATION_STATUS.ja.md",
        "haco-vscode open",
        "v0.9",
        "v0.10",
        "haco-agent-host",
        "v0.11",
        "haco base list",
        "haco plugin oci",
        "v0.12",
    ],
    "required Japanese entry-point content",
)

if errors:
    print("DOC CONSISTENCY FAILED")
    print("\n".join(errors))
    sys.exit(1)

print("DOC CONSISTENCY OK")

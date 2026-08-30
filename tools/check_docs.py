#!/usr/bin/env python3
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parents[1]
files = [p for p in root.rglob('*.md') if p.name != 'Hacocoon_v0.1-v0.7_MASTER.md']

checks = [
    (r'v0\.1 Local Foundation|v0\.2 Developer Workspace|v0\.3 Security Framework|v0\.4 External Capabilities|v0\.5 Local GUI|v0\.6 Local Web', 'old release ordering'),
    (r'01_v0\.1_LOCAL_FOUNDATION|02_v0\.2_DEVELOPER_WORKSPACE|03_v0\.3_SECURITY_FRAMEWORK_AND_GIT|04_v0\.4_EXTERNAL_CAPABILITIES|05_v0\.5_LOCAL_GUI_AND_IDE|06_v0\.6_LOCAL_WEB_AND_INTERACTION|07_v0\.7_REMOTE_AND_EC2', 'superseded release filename'),
    (r'11_v0\.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER|#\s*v0\.11\s+VS Code Remote Agent Host Adapter', 'stale v0.11 Agent Host adapter assignment'),
    (r'\bHacocoon IAM\b', 'legacy Hacocoon IAM term'),
    (r'Manager/Session trust boundary|Manager / Session trust boundary', 'legacy Manager/Session architecture boundary'),
    (r'Runtime/Storage seams|Security and Feature Plugin boundaries', 'pre-rebaseline ADR terminology'),
    (r'\bDirectoryWorkspace\b', 'redundant workspace-provider name'),
    (r'Status:\s*\*\*current implementation gate\*\*', 'stale v0.1-as-current status'),
]

errors = []
for p in files:
    text = p.read_text()
    for pat, label in checks:
        for m in re.finditer(pat, text, flags=re.IGNORECASE):
            line = text[:m.start()].count('\n') + 1
            snippet = text.splitlines()[line - 1]
            if label == 'legacy Hacocoon IAM term' and any(word in snippet.lower() for word in ('historical', 'legacy', 'do not')):
                continue
            errors.append(f'{p.relative_to(root)}:{line}: {label}: {snippet.strip()}')

superseded_files = [
    'docs/01_v0.1_LOCAL_FOUNDATION.md',
    'docs/02_v0.2_DEVELOPER_WORKSPACE.md',
    'docs/03_v0.3_SECURITY_FRAMEWORK_AND_GIT.md',
    'docs/04_v0.4_EXTERNAL_CAPABILITIES.md',
    'docs/05_v0.5_LOCAL_GUI_AND_IDE.md',
    'docs/06_v0.6_LOCAL_WEB_AND_INTERACTION.md',
    'docs/07_v0.7_REMOTE_AND_EC2.md',
    'docs/09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md',
    'docs/10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.md',
    'docs/10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md',
    'docs/11_v0.11_SANDBOX_RESOURCE_LIMITS.md',
    'docs/11_v0.11_SANDBOX_RESOURCE_LIMITS.ja.md',
    'docs/11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md',
    'docs/11_v0.11_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md',
]
for rel in superseded_files:
    if (root / rel).exists():
        errors.append(f'superseded documentation file still exists: {rel}')

required_files = [
    'docs/README.md',
    'docs/README.ja.md',
    'docs/00_REBASELINE_AND_ROADMAP.md',
    'docs/00D_VERSIONING_AND_RELEASE_STATUS.md',
    'docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md',
    'docs/00C_TERMINOLOGY_AND_BOUNDARIES.md',
    'docs/00B_SECURITY_ARCHITECTURE.md',
    'docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md',
    'docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md',
    'docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md',
    'docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md',
    'docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md',
    'docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md',
    'docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md',
    'docs/BASE_IMAGES.md',
    'docs/BASE_IMAGES.ja.md',
    'docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md',
    'docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md',
    'docs/IMPLEMENTATION_STATUS.md',
    'docs/IMPLEMENTATION_STATUS.ja.md',
    'README.md',
    'README.ja.md',
    'docs/ARCHITECTURE_GUIDE.ja.md',
]
for rel in required_files:
    if not (root / rel).exists():
        errors.append(f'missing required documentation: {rel}')

# Documentation map must expose the current source-of-truth and current gates.
docmap = (root / 'docs/README.md').read_text()
for required in [
    'Source-of-truth order',
    'Specification vs implementation',
    'pre-1.0',
    'IMPLEMENTATION_STATUS.md',
    '00D_VERSIONING_AND_RELEASE_STATUS.md',
    '09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md',
    '10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md',
    '11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md',
    '12_v0.12_SANDBOX_RESOURCE_LIMITS.md',
    'haco-agent-host',
    'haco image list',
]:
    if required not in docmap:
        errors.append(f'docs/README.md missing current documentation-map content: {required}')

# The authoritative numbering must keep implemented milestones contiguous through v0.12.
versioning = (root / 'docs/00D_VERSIONING_AND_RELEASE_STATUS.md').read_text()
numbering_required = [
    'v0.8 | Client Adapters & VS Code Integration | implemented',
    'v0.9 | Per-Agent Sandbox & Agent Host Integration | broker foundation implemented',
    'v0.10 | VS Code Remote Agent Host Adapter | implemented',
    'v0.11 | Base Images & Custom Environments | implemented first slice',
    'v0.12 | Sandbox Resource Limits | implemented first slice',
    'implemented progression is therefore contiguous through **v0.12**',
    'PR #137',
]
for required in numbering_required:
    if required.lower() not in versioning.lower():
        errors.append(f'versioning status missing current numbering rule: {required}')

roadmap = (root / 'docs/00_REBASELINE_AND_ROADMAP.md').read_text()
for required in [
    'Hacocoon is a **Secure Workspace Runtime**',
    'v0.9 | Per-Agent Sandbox & Agent Host Integration',
    'v0.10 | VS Code Remote Agent Host Adapter',
    'v0.11 | Base Images & Custom Environments',
    'v0.12 | Sandbox Resource Limits',
    'experimental and disabled by default',
    'pre-1.0',
]:
    if required not in roadmap:
        errors.append(f'roadmap missing current contract text: {required}')

status = (root / 'docs/IMPLEMENTATION_STATUS.md').read_text()
for required in [
    'current code reality',
    '`haco create --workspace`',
    'v0.9 |',
    'Per-agent sandbox broker',
    'internal/agenthost',
    'agent-bindings.json',
    'v0.10 |',
    'haco-agent-host',
    'PR #137',
    'v0.11 |',
    'haco image list',
    'haco create --base',
    'HACO_INCUS_BASES_JSON',
    'v0.12 |',
    'Resource budget model',
    '--cpu',
    'Incus resource enforcement',
    'real AWS acceptance pending',
]:
    if required.lower() not in status.lower():
        errors.append(f'IMPLEMENTATION_STATUS missing current reality: {required}')

v01 = (root / 'docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md').read_text()
for required in ['haco create --workspace', 'haco exec', 'haco shell', 'haco delete', 'Incus']:
    if required not in v01:
        errors.append(f'v0.1 missing required runtime gate text: {required}')

v08 = (root / 'docs/08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md').read_text()
for required in ['Client Adapter', 'haco-vscode', 'Remote-SSH', 'loopback-only', 'Windows + WSL']:
    if required not in v08:
        errors.append(f'v0.8 missing required client-adapter contract text: {required}')

v09 = (root / 'docs/09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md').read_text()
for required in ['Per-Agent Sandbox', 'per-session broker', 'Agent Host', 'AHP', 'WorkspaceLease', 'persisted binding proof', 'v0.11 Base Images']:
    if required.lower() not in v09.lower():
        errors.append(f'v0.9 missing required per-agent contract text: {required}')

v10 = (root / 'docs/10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md').read_text()
for required in ['VS Code Remote Agent Host Adapter', 'haco-agent-host', 'loopback', 'opaque session', 'private key', 'code --agents', 'PR #137']:
    if required.lower() not in v10.lower():
        errors.append(f'v0.10 missing required Agent Host adapter content: {required}')

v11 = (root / 'docs/11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md').read_text()
for required in [
    'Base Images & Custom Environments',
    'first implementation slice',
    'immutable Base revision',
    'Incus image fingerprint',
    'haco image list',
    'haco create --base',
    'HACO_INCUS_BASES_JSON',
    'referenced Base revision',
]:
    if required.lower() not in v11.lower():
        errors.append(f'v0.11 missing required Base-image contract text: {required}')

v12 = (root / 'docs/12_v0.12_SANDBOX_RESOURCE_LIMITS.md').read_text()
for required in ['Sandbox Resource Limits', 'ResourceBudget', 'CPU', 'memory', 'process/PID', 'root-storage', 'fail closed', 'v0.11 Bases', 'v0.9 agent-session binding']:
    if required.lower() not in v12.lower():
        errors.append(f'v0.12 missing required resource-budget contract text: {required}')

reference = (root / 'docs/91_IMPLEMENTATION_REFERENCE_NOTES.md').read_text()
if 'non-normative' not in reference.lower() or 'No current architecture contract commits' not in reference:
    errors.append('reference notes must remain explicitly non-normative')

readme = (root / 'README.md').read_text()
for required in [
    'docs/README.md',
    'pre-1.0',
    'haco-vscode open',
    'v0.9',
    '09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md',
    'v0.10',
    '10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md',
    'haco-agent-host',
    'v0.11',
    'haco image list',
    'haco create --base',
    '11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md',
    'v0.12',
    '12_v0.12_SANDBOX_RESOURCE_LIMITS.md',
]:
    if required not in readme:
        errors.append(f'README.md missing current roadmap entry-point content: {required}')

ja_readme = (root / 'README.ja.md').read_text()
for required in [
    '読み方: はこーん',
    'pre-1.0',
    'Breaking Change',
    'IMPLEMENTATION_STATUS.ja.md',
    'haco-vscode open',
    'v0.9',
    'v0.10',
    'haco-agent-host',
    'v0.11',
    'haco image list',
    'v0.12',
]:
    if required not in ja_readme:
        errors.append(f'README.ja.md missing required Japanese entry-point content: {required}')

if errors:
    print('DOC CONSISTENCY FAILED')
    print('\n'.join(errors))
    sys.exit(1)

print('DOC CONSISTENCY OK')

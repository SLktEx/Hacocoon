#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]
order = [
    'README.md',
    'CODEX_START_HERE.md',
    'REFACTOR_NOTES.md',
    'docs/00_REBASELINE_AND_ROADMAP.md',
    'docs/00C_TERMINOLOGY_AND_BOUNDARIES.md',
    'docs/00A_PLUGIN_ARCHITECTURE.md',
    'docs/00B_SECURITY_ARCHITECTURE.md',
    'docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md',
    'docs/02_v0.2_WORKSPACE_ABSTRACTION_AND_LEASE.md',
    'docs/03_v0.3_CLIENT_AND_INTERACTIVE_ACCESS.md',
    'docs/04_v0.4_POLICY_AND_CAPABILITY_FOUNDATION.md',
    'docs/05_v0.5_GIT_AND_GITHUB_CAPABILITY.md',
    'docs/06_v0.6_AGENT_AND_ORCHESTRATOR_INTEGRATION.md',
    'docs/07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md',
    'docs/90_CODEX_IMPLEMENTATION_HANDOFF.md',
    'docs/91_IMPLEMENTATION_REFERENCE_NOTES.md',
    'docs/IMPLEMENTATION_STATUS.md',
    'docs/IMPLEMENTATION_STATUS_TEMPLATE.md',
]

out = []
for rel in order:
    p = root / rel
    out.append(f'<!-- FILE: {rel} -->\n\n' + p.read_text().rstrip() + '\n')

(root / 'Hacocoon_v0.1-v0.7_MASTER.md').write_text('\n\n'.join(out) + '\n')

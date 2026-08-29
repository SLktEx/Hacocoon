#!/usr/bin/env python3
from pathlib import Path
import hashlib
import re
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
files = [p for p in root.rglob('*.md') if p.name != 'Hacocoon_v0.1-v0.7_MASTER.md']

checks = [
    (r'v0\.1 Local Foundation|v0\.2 Developer Workspace|v0\.3 Security Framework|v0\.4 External Capabilities|v0\.5 Local GUI|v0\.6 Local Web', 'old release ordering'),
    (r'\bHacocoon IAM\b', 'legacy Hacocoon IAM term'),
]

errors = []
for p in files:
    text = p.read_text()
    for pat, label in checks:
        for m in re.finditer(pat, text):
            line = text[:m.start()].count('\n') + 1
            snippet = text.splitlines()[line - 1]
            if label == 'legacy Hacocoon IAM term' and (p.name == 'REFACTOR_NOTES.md' or 'historical' in snippet.lower()):
                continue
            errors.append(f'{p.relative_to(root)}:{line}: {label}: {snippet.strip()}')

v01 = (root / 'docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md').read_text()
for required in [
    'haco create --workspace',
    'haco exec',
    'haco shell',
    'haco delete',
    'External',
    'Incus',
]:
    if required not in v01:
        errors.append(f'v0.1 missing required runtime gate text: {required}')

v07 = (root / 'docs/07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md').read_text()
if 'EC2/EBS provisioning design gate' not in v07:
    errors.append('v0.7 missing EC2/EBS provisioning design gate')

roadmap = (root / 'docs/00_REBASELINE_AND_ROADMAP.md').read_text()
if 'Hacocoon is a **Secure Workspace Runtime**' not in roadmap and 'Hacocoon は **AI 開発オーケストレーターではなく、安全な Workspace Runtime**' not in roadmap:
    errors.append('roadmap missing secure workspace runtime baseline')

if errors:
    print('DOC CONSISTENCY FAILED')
    print('\n'.join(errors))
    sys.exit(1)

master = root / 'Hacocoon_v0.1-v0.7_MASTER.md'
before = hashlib.sha256(master.read_bytes()).hexdigest() if master.exists() else None
subprocess.run([sys.executable, str(root / 'tools/build_master.py')], check=True)
after = hashlib.sha256(master.read_bytes()).hexdigest()
if before is not None and before != after:
    print('DOC CONSISTENCY FAILED')
    print('MASTER was stale and was regenerated; commit/regenerate it before packaging')
    sys.exit(1)

print('DOC CONSISTENCY OK')

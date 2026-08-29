#!/usr/bin/env python3
from pathlib import Path
import re, sys
root=Path(__file__).resolve().parents[1]
files=[p for p in root.rglob('*.md') if p.name!='Hacocoon_v0.1-v0.7_MASTER.md']
checks=[
    (r'\bHacocoon IAM\b', 'legacy Hacocoon IAM term'),
    (r'storage\.local-btrfs', 'old storage.local-btrfs id'),
    (r'05_v0\.5_REMOTE|06_v0\.6_NATIVE|v0\.5 Remote|v0\.6 Native', 'old release ordering'),
    (r'Manager / Controller|Controller/Manager|AuthProvider', 'superseded manager/access-layer terminology'),
    (r'plugin\.intellij\b', 'old plugin.intellij id'),
]
errors=[]
for p in files:
    text=p.read_text()
    for pat,label in checks:
        for m in re.finditer(pat,text):
            # canonical glossary/security may intentionally mention the forbidden historical term in a negation.
            line=text[:m.start()].count('\n')+1
            snippet=text.splitlines()[line-1]
            if label=='legacy Hacocoon IAM term' and (p.name in {'REFACTOR_NOTES.md','00C_TERMINOLOGY_AND_BOUNDARIES.md'} or 'Do not' in snippet or 'named' in snippet or 'canonical' in snippet):
                continue
            if label=='old storage.local-btrfs id' and p.name=='REFACTOR_NOTES.md':
                continue
            errors.append(f'{p.relative_to(root)}:{line}: {label}: {snippet.strip()}')
# storage outer-shrink ordering invariant
v01=(root/'docs/01_v0.1_LOCAL_FOUNDATION.md').read_text()
if 'Never shrink/truncate the outer image before the filesystem reports a successful smaller size.' not in v01:
    errors.append('v0.1 missing inner-to-outer shrink invariant')
# v0.7 must contain explicit EC2/EBS design gate
v07=(root/'docs/07_v0.7_REMOTE_AND_EC2.md').read_text()
if 'EC2/EBS provisioning design gate' not in v07:
    errors.append('v0.7 missing EC2/EBS provisioning design gate')
if errors:
    print('DOC CONSISTENCY FAILED')
    print('\n'.join(errors))
    sys.exit(1)
# Ensure generated master is current.
import subprocess, hashlib
master=root/'Hacocoon_v0.1-v0.7_MASTER.md'
before=hashlib.sha256(master.read_bytes()).hexdigest() if master.exists() else None
subprocess.run([sys.executable, str(root/'tools/build_master.py')], check=True)
after=hashlib.sha256(master.read_bytes()).hexdigest()
if before is not None and before != after:
    print('DOC CONSISTENCY FAILED')
    print('MASTER was stale and was regenerated; commit/regenerate it before packaging')
    sys.exit(1)
print('DOC CONSISTENCY OK')

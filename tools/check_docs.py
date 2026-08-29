#!/usr/bin/env python3
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parents[1]
files = [p for p in root.rglob('*.md') if p.name != 'Hacocoon_v0.1-v0.7_MASTER.md']

checks = [
    (r'v0\.1 Local Foundation|v0\.2 Developer Workspace|v0\.3 Security Framework|v0\.4 External Capabilities|v0\.5 Local GUI|v0\.6 Local Web', 'old release ordering'),
    (r'01_v0\.1_LOCAL_FOUNDATION|02_v0\.2_DEVELOPER_WORKSPACE|03_v0\.3_SECURITY_FRAMEWORK_AND_GIT|04_v0\.4_EXTERNAL_CAPABILITIES|05_v0\.5_LOCAL_GUI_AND_IDE|06_v0\.6_LOCAL_WEB_AND_INTERACTION|07_v0\.7_REMOTE_AND_EC2', 'superseded release filename'),
    (r'\bHacocoon IAM\b', 'legacy Hacocoon IAM term'),
    (r'Manager/Session trust boundary|Manager / Session trust boundary', 'legacy Manager/Session architecture boundary'),
    (r'Runtime/Storage seams|Security and Feature Plugin boundaries', 'pre-rebaseline ADR terminology'),
    (r'\bDirectoryWorkspace\b', 'redundant workspace-provider name'),
    (r'Status:\s*\*\*current implementation gate\*\*', 'stale v0.1-as-current status'),
    (r'Finish \*\*v0\.1 Secure Workspace Runtime MVP\*\* before extending later releases', 'stale v0.1-only implementation objective'),
    (r'Do not implement v0\.2-v0\.7 while finishing v0\.1', 'stale v0.1-only implementation rule'),
    (r'Stop at the v0\.1 acceptance gate', 'stale v0.1-only stop rule'),
    (r'current release specification.*currently `01_v0\.1_SECURE_WORKSPACE_RUNTIME\.md`', 'stale documentation precedence'),
]

errors = []
for p in files:
    text = p.read_text()
    for pat, label in checks:
        for m in re.finditer(pat, text, flags=re.IGNORECASE):
            line = text[:m.start()].count('\n') + 1
            snippet = text.splitlines()[line - 1]
            # Historical mention of the old IAM name is allowed only when clearly
            # marked as historical/forbidden rather than presented as architecture.
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
]
for rel in superseded_files:
    if (root / rel).exists():
        errors.append(f'superseded documentation file still exists: {rel}')

required_files = [
    'docs/README.md',
    'docs/00_REBASELINE_AND_ROADMAP.md',
    'docs/00C_TERMINOLOGY_AND_BOUNDARIES.md',
    'docs/00B_SECURITY_ARCHITECTURE.md',
    'docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md',
    'docs/IMPLEMENTATION_STATUS.md',
    'README.ja.md',
    'docs/README.ja.md',
    'docs/ARCHITECTURE_GUIDE.ja.md',
    'docs/IMPLEMENTATION_STATUS.ja.md',
]
for rel in required_files:
    if not (root / rel).exists():
        errors.append(f'missing required documentation: {rel}')

if (root / 'docs/README.md').exists():
    docmap = (root / 'docs/README.md').read_text()
    if 'Source-of-truth order' not in docmap or 'Specification vs implementation' not in docmap:
        errors.append('docs/README.md must define documentation precedence and specification-vs-implementation semantics')
    if 'pre-1.0' not in docmap or 'IMPLEMENTATION_STATUS.md' not in docmap:
        errors.append('docs/README.md must describe pre-1.0 compatibility and point to current implementation reality')
    if 'README.ja.md' not in docmap or 'ARCHITECTURE_GUIDE.ja.md' not in docmap:
        errors.append('docs/README.md must link the maintained Japanese documentation entry points')

v01 = (root / 'docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md').read_text()
for required in ['haco create --workspace', 'haco exec', 'haco shell', 'haco delete', 'Incus']:
    if required not in v01:
        errors.append(f'v0.1 missing required runtime gate text: {required}')
if 'implementation exists on `main`' not in v01:
    errors.append('v0.1 spec must be identified as an implemented roadmap contract, not the current-only gate')

v03 = (root / 'docs/03_v0.3_CLIENT_AND_INTERACTIVE_ACCESS.md').read_text()
if 'v0.4 Policy/Capability boundary' not in v03:
    errors.append('v0.3 must defer privileged/sensitive exposure decisions to the v0.4 Policy/Capability boundary')

v07 = (root / 'docs/07_v0.7_REMOTE_AND_CLOUD_RUNTIME.md').read_text()
if 'EC2/EBS provisioning design gate' not in v07:
    errors.append('v0.7 missing EC2/EBS provisioning design gate')
if 'EC2 release status: experimental and disabled by default' not in v07:
    errors.append('v0.7 must mark EC2 experimental and disabled by default')
if 'no AWS API call' not in v07:
    errors.append('v0.7 must require the disabled EC2 path to avoid AWS API calls')
for required in ['HACO_RUNTIME_PROVIDER=runtime.ec2', 'HACO_EXPERIMENTAL_EC2=1']:
    if required not in v07:
        errors.append(f'v0.7 must document the implemented EC2 opt-in: {required}')

roadmap = (root / 'docs/00_REBASELINE_AND_ROADMAP.md').read_text()
if 'Hacocoon is a **Secure Workspace Runtime**' not in roadmap:
    errors.append('roadmap missing Secure Workspace Runtime baseline')
if 'experimental and disabled by default' not in roadmap:
    errors.append('roadmap must preserve the v0.7 experimental EC2 default-off rule')
if 'pre-1.0' not in roadmap:
    errors.append('roadmap must preserve the pre-1.0 compatibility policy')

status = (root / 'docs/IMPLEMENTATION_STATUS.md').read_text()
if 'current code reality' not in status.lower() or '`haco create --workspace`' not in status:
    errors.append('IMPLEMENTATION_STATUS must distinguish current repository reality from versioned roadmap contracts')
if 'pre-1.0' not in status or 'real AWS acceptance pending' not in status:
    errors.append('IMPLEMENTATION_STATUS must distinguish compatibility and real-provider acceptance from implementation presence')

reference = (root / 'docs/91_IMPLEMENTATION_REFERENCE_NOTES.md').read_text()
if 'non-normative' not in reference.lower() or 'No current architecture contract commits' not in reference:
    errors.append('reference notes must be explicitly non-normative and avoid turning historical storage experiments into commitments')

for rel in ('README.md', 'CODEX_START_HERE.md', 'Hacocoon_v0.1-v0.7_MASTER.md'):
    text = (root / rel).read_text()
    if 'docs/README.md' not in text:
        errors.append(f'{rel} must point to docs/README.md for documentation precedence')
    if 'pre-1.0' not in text:
        errors.append(f'{rel} must make the pre-1.0 compatibility state explicit')

readme = (root / 'README.md').read_text()
if 'README.ja.md' not in readme:
    errors.append('README.md must link the maintained Japanese README')

ja_readme = (root / 'README.ja.md').read_text()
for required in ['読み方: はこーん', 'pre-1.0', 'Breaking Change', 'IMPLEMENTATION_STATUS.ja.md', 'ARCHITECTURE_GUIDE.ja.md']:
    if required not in ja_readme:
        errors.append(f'README.ja.md missing required Japanese entry-point content: {required}')

if errors:
    print('DOC CONSISTENCY FAILED')
    print('\n'.join(errors))
    sys.exit(1)

print('DOC CONSISTENCY OK')
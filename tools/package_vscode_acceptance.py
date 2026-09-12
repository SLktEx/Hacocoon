#!/usr/bin/env python3
"""Package the disposable UI observer using the VSIX container format."""
import argparse
import json
from pathlib import Path
import re
import secrets
import zipfile

HERE = Path(__file__).resolve().parent
def package(environment: str, output: Path, result: Path) -> dict:
    if not re.fullmatch(r'win-ssh-[a-f0-9]{16}', environment):
        raise ValueError('invalid acceptance Environment')
    if not result.is_absolute() or result.exists():
        raise ValueError('result must be a fresh absolute path')
    fixture = {'authority': 'ssh-remote+haco-' + environment,
               'nonce': secrets.token_hex(16), 'result': str(result)}
    manifest = {'name': 'hacocoon-acceptance', 'displayName': 'Hacocoon Acceptance',
                'publisher': 'hacocoon-validation', 'version': '0.0.1',
                'engines': {'vscode': '^1.95.0'}, 'main': './extension.js',
                'extensionKind': ['ui'], 'activationEvents': ['onStartupFinished'],
                'capabilities': {'untrustedWorkspaces': {'supported': False}}}
    vsix = """<?xml version="1.0" encoding="utf-8"?>
<PackageManifest Version="2.0.0" xmlns="http://schemas.microsoft.com/developer/vsx-schema/2011">
<Metadata><Identity Language="en-US" Id="hacocoon-acceptance" Version="0.0.1" Publisher="hacocoon-validation"/>
<DisplayName>Hacocoon Acceptance</DisplayName><Description xml:space="preserve">Disposable desktop acceptance observer</Description>
<Properties><Property Id="Microsoft.VisualStudio.Code.Engine" Value="^1.95.0"/></Properties></Metadata>
<Installation><InstallationTarget Id="Microsoft.VisualStudio.Code"/></Installation><Dependencies/>
<Assets><Asset Type="Microsoft.VisualStudio.Code.Manifest" Path="extension/package.json" Addressable="true"/></Assets>
</PackageManifest>"""
    types = """<?xml version="1.0" encoding="utf-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="json" ContentType="application/json"/><Default Extension="js" ContentType="application/javascript"/>
<Default Extension="vsixmanifest" ContentType="text/xml"/></Types>"""
    with zipfile.ZipFile(output, 'x', zipfile.ZIP_DEFLATED) as z:
        z.writestr('extension.vsixmanifest', vsix)
        z.writestr('[Content_Types].xml', types)
        z.writestr('extension/package.json', json.dumps(manifest))
        z.writestr('extension/fixture.json', json.dumps(fixture))
        z.write(HERE / 'vscode-acceptance/extension.js', 'extension/extension.js')
        z.write(HERE.parent / 'clients/vscode-notify/review.js', 'extension/review.js')
    return fixture

def main():
    p = argparse.ArgumentParser()
    p.add_argument('--environment', required=True)
    p.add_argument('--output', type=Path, required=True)
    p.add_argument('--result', type=Path, required=True)
    p.add_argument('--manifest', type=Path, required=True)
    args = p.parse_args()
    fixture = package(args.environment, args.output, args.result)
    with args.manifest.open('x', encoding='utf-8') as f:
        json.dump(fixture, f)
if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Run Windows-native acceptance while an ordinary trusted Host terminal is open.
Reuses the maintained ConPTY driver, including product login/startup waiting.
No source setup, provider repair or replacement controller is invoked.
"""
import argparse
import importlib.util
import re
import shutil
import subprocess
import sys
from pathlib import Path


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--interop-only', action='store_true')
    parser.add_argument('--require-non-c', action='store_true')
    parser.add_argument('--persistence-manifest')
    args = parser.parse_args()
    here = Path(__file__).resolve().parent
    spec = importlib.util.spec_from_file_location('native_access_driver', here / 'windows-installer-user-path-e2e.py')
    driver = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = driver
    spec.loader.exec_module(driver)
    powershell = shutil.which('pwsh')
    if not powershell: raise RuntimeError('PowerShell 7 is required for the acceptance scripts')
    terminal = driver.TerminalProcess()
    stage, sent_at = 0, 0

    def drive(output, process):
        nonlocal stage, sent_at
        if stage == 0 and driver.cmd_prompt_count(output):
            process.write('wsl -d Hacocoon\r\n')
            stage, sent_at = 1, len(output)
        elif stage == 1 and re.search(r'(?m)^[^\r\n]*@haco-host:[^\r\n]*[#\$]\s*$', output[sent_at:]):
            scripts = [('test_windows_host_interop.ps1', ['-RequireNonC'] if args.require_non_c else [])]
            if args.persistence_manifest: scripts[0][1].extend(['-PersistenceManifest', str(Path(args.persistence_manifest).resolve())])
            if not args.interop_only: scripts.append(('test_windows_environment_ssh.ps1', []))
            for name, options in scripts:
                result = subprocess.run([powershell, '-NoLogo', '-NoProfile', '-NonInteractive',
                    '-ExecutionPolicy', 'Bypass', '-File', str(here / name), *options], timeout=1800, capture_output=True, text=True, encoding='utf-8', errors='replace')
                print(result.stdout, result.stderr, flush=True)
                expected = 'Direct .exe / Windows PATH / stdout / stderr / exit 23 / spaces: PASS' if name == 'test_windows_host_interop.ps1' else 'WINDOWS DIRECT ENVIRONMENT SSH: PASS'
                if expected not in result.stdout: raise RuntimeError(f'{name} did not report its acceptance assertions')
                if result.returncode: raise RuntimeError(f'{name} failed with exit {result.returncode}')
            process.write('exit\r\n')
            stage, sent_at = 2, len(output)
        elif stage == 2 and driver.cmd_prompt_count(output[sent_at:]):
            process.write('exit\r\n')
            stage = 3

    try:
        terminal.run(on_output=drive)
        if stage != 3: raise RuntimeError('Ordinary Host terminal did not complete the native acceptance sequence')
    finally:
        if terminal.proc.isalive(): terminal.proc.terminate(force=True)
    print('WINDOWS NATIVE ACCESS THROUGH ORDINARY HOST ENTRY: PASS')


if __name__ == '__main__':
    main()
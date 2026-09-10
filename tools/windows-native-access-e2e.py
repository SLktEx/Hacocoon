#!/usr/bin/env python3
"""Run Windows-native acceptance while an ordinary trusted Host terminal is open.
Reuses the maintained ConPTY driver, including product login/startup waiting.
No source setup, provider repair or replacement controller is invoked.
"""
import argparse
import os
import importlib.util
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


def run_acceptance(command, timeout=1800):
    # File-backed output cannot hold communicate() open through a surviving
    # WSL descendant after the immediate PowerShell process exits.
    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        process = subprocess.Popen(command, stdout=stdout, stderr=stderr)
        try:
            process.wait(timeout=timeout)
        except subprocess.TimeoutExpired as error:
            if os.name == 'nt':
                subprocess.run(['taskkill', '/PID', str(process.pid), '/T', '/F'],
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                               timeout=15, check=False)
            if process.poll() is None:
                process.kill()
            process.wait(timeout=15)
            raise RuntimeError('Native acceptance child timed out') from error
        finally:
            if process.poll() is None:
                process.kill()
                process.wait(timeout=15)
        stdout.seek(0)
        stderr.seek(0)
        limit = 4 * 1024 * 1024
        out, err = stdout.read(limit + 1), stderr.read(limit + 1)
        if len(out) > limit or len(err) > limit:
            raise RuntimeError('Native acceptance output exceeded its limit')
        return subprocess.CompletedProcess(command, process.returncode,
            out.decode('utf-8', errors='replace'), err.decode('utf-8', errors='replace'))


def verify_acceptance_result(name, result, require_vscode=False):
    # A real child failure takes precedence over missing success markers.
    if result.returncode:
        raise RuntimeError(f'{name} failed with exit {result.returncode}')
    expected = {
        'test_windows_host_interop.ps1': 'Direct .exe / Windows PATH / stdout / stderr / exit 23 / spaces: PASS',
        'test_windows_environment_ssh.ps1': 'WINDOWS DIRECT ENVIRONMENT SSH: PASS',
        'test_host_customization.ps1': 'HOST CUSTOMIZATION SAVE / REPLAY / UPDATE / CLEAR: PASS',
    }[name]
    if expected not in result.stdout:
        raise RuntimeError(f'{name} did not report its acceptance assertions')
    if name == 'test_windows_environment_ssh.ps1' and require_vscode and 'VS CODE REMOTE ENVIRONMENT: PASS' not in result.stdout:
        raise RuntimeError('Real VS Code acceptance did not report its assertions')


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
            print('NATIVE ACCEPTANCE: entering ordinary Host terminal', flush=True)
            process.write('wsl -d Hacocoon\r\n')
            stage, sent_at = 1, len(output)
        elif stage == 1 and re.search(r'(?m)^[^\r\n]*@haco-host:[^\r\n]*[#\$]\s*$', output[sent_at:]):
            scripts = [('test_windows_host_interop.ps1', ['-RequireNonC'] if args.require_non_c else [])]
            if args.persistence_manifest: scripts[0][1].extend(['-PersistenceManifest', str(Path(args.persistence_manifest).resolve())])
            if not args.interop_only:
                # Environment creation/deletion must not break interop in the
                # already-open trusted Host session.
                scripts.extend([('test_windows_environment_ssh.ps1', [])])
                if os.environ.get('GITHUB_ACTIONS') == 'true':
                    scripts.append(('test_host_customization.ps1', []))
                scripts.append(scripts[0])
            for name, options in scripts:
                print(f'NATIVE ACCEPTANCE START: {name}', flush=True)
                result = run_acceptance([powershell, '-NoLogo', '-NoProfile', '-NonInteractive',
                    '-ExecutionPolicy', 'Bypass', '-File', str(here / name), *options], timeout=1800)
                print(result.stdout, result.stderr, flush=True)
                verify_acceptance_result(name, result, os.environ.get('GITHUB_ACTIONS') == 'true')
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
#!/usr/bin/env python3
"""Exercise temporary TTY execution through the ordinary installed Windows entry."""
import importlib.util
import os
from pathlib import Path
import re
import shlex
import sys


def main():
    if os.name != 'nt' or os.environ.get('GITHUB_ACTIONS') != 'true' or os.environ.get('RUNNER_ENVIRONMENT') != 'github-hosted':
        raise RuntimeError('temporary terminal acceptance requires the disposable Windows GHA host')
    spec = importlib.util.spec_from_file_location('run_stream_driver', Path(__file__).with_name('windows-installer-user-path-e2e.py'))
    driver = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = driver
    spec.loader.exec_module(driver)
    terminal = driver.TerminalProcess()
    stage, sent_at, native = 0, 0, None
    guest = (
        'test -t 0; test "$PWD" = /workspace; printf "RUN-TTY-READY\\n"; stty size; '
        'IFS= read -r line; printf "RUN-INPUT:%s\\n" "$line"; '
        'while [ "$(stty size)" != "43 132" ]; do sleep 0.1; done; '
        'printf "RUN-RESIZED:"; stty size; exit 17'
    )
    command = (
        'run_before=$(haco env list --json) && run_terminal=$(stty -g) && '
        'haco run -it -- sh -ec ' + shlex.quote(guest) + '; '
        'run_status=$?; printf "RUN-EXIT:%s\\n" "$run_status"; '
        'test "$run_terminal" = "$(stty -g)" && '
        'test "$run_before" = "$(haco env list --json)" && printf "RUN-RESTORED\\n"'
    )

    def provider_names():
        return set(driver.inspect_root('incus', 'list', '--project', 'hacocoon', '--format', 'csv', '-c', 'n').splitlines())

    def drive(output, process):
        nonlocal stage, sent_at, native
        fresh = output[sent_at:]
        if stage == 0 and driver.cmd_prompt_count(output):
            process.write('wsl -d Hacocoon\r\n')
            stage, sent_at = 1, len(output)
        elif stage == 1 and re.search(r'(?m)^[^\r\n]*@haco-host:[^\r\n]*[#\$]\s*$', fresh):
            native = provider_names()
            process.write(command + '\r\n')
            stage, sent_at = 2, len(output)
        elif stage == 2 and re.search(r'(?m)^RUN-TTY-READY\s*$', fresh) and re.search(r'(?m)^48 160\s*$', fresh):
            # pywinpty's public API takes (rows, columns); resize the actual
            # Windows terminal, without injecting a product resize request.
            process.proc.setwinsize(43, 132)
            process.write('abc\x7fD\r')
            stage = 3
        elif stage == 3 and re.search(r'(?m)^RUN-RESTORED\s*$', fresh):
            for pattern in (r'^RUN-INPUT:abD\s*$', r'^RUN-RESIZED:43 132\s*$', r'^RUN-EXIT:17\s*$'):
                driver.require_output(fresh, pattern, phase='ordinary Windows temporary terminal')
            if provider_names() != native:
                raise RuntimeError('temporary Windows terminal did not preserve native ownership')
            process.write('exit\r\n')
            stage, sent_at = 4, len(output)
        elif stage == 4 and driver.cmd_prompt_count(fresh):
            process.write('exit\r\n')
            stage = 5

    try:
        terminal.run(on_output=drive, timeout=600)
        if stage != 5:
            raise RuntimeError('ordinary Windows temporary terminal did not finish its assertions')
    finally:
        if terminal.proc.isalive():
            terminal.proc.terminate(force=True)
    print('WINDOWS TEMPORARY TTY / ORDINARY HOST ENTRY / EDIT-RESIZE / EXIT 17 / RESTORE / CLEANUP: PASS')


if __name__ == '__main__':
    main()

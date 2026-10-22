#!/usr/bin/env python3
"""Read only bounded, fixture-bound observer signals, never editor logs."""
import argparse
import json
import os
import re
import stat
from pathlib import Path

# Fixed local marker names; order is presentation, not an inferred event timeline.
PHASES = (
    'activated', 'target-missing', 'target-scheme-mismatch',
    'target-authority-mismatch', 'target-path-mismatch', 'target-matched',
    'remote-kind', 'remote-filesystem', 'remote-terminal',
    'local-approval-review', 'cleanup', 'complete', 'deadline',
    'finish-started', 'terminal-disposed', 'review-disposed',
    'receipt-written', 'receipt-published',
)
MAX_SIGNAL_BYTES = 512


def signal_identity(metadata):
    if (not stat.S_ISREG(metadata.st_mode) or metadata.st_nlink != 1
            or getattr(metadata, 'st_file_attributes', 0) & 0x400):
        raise ValueError('not a regular signal')
    return metadata.st_dev, metadata.st_ino


def read_signal(path: Path) -> bytes:
    # Only for the trusted disposable fixture directory and its parent path.
    # Portable os.open has no atomic Windows no-follow/nonblocking guarantee;
    # snapshots reject observable changes, not hostile concurrent path mutation.
    before = signal_identity(path.lstat())
    flags = os.O_RDONLY | getattr(os, 'O_NOFOLLOW', 0) | getattr(os, 'O_NONBLOCK', 0)
    try:
        with os.fdopen(os.open(path, flags), 'rb') as stream:
            if (signal_identity(os.fstat(stream.fileno())) != before
                    or signal_identity(path.lstat()) != before):
                raise ValueError('signal changed')
            raw = stream.read(MAX_SIGNAL_BYTES + 1)
            if (signal_identity(os.fstat(stream.fileno())) != before
                    or signal_identity(path.lstat()) != before):
                raise ValueError('signal changed')
            return raw
    except FileNotFoundError:
        # Initial absence is unobserved; disappearance after lstat is invalid.
        raise ValueError('signal changed') from None


def unique_fields(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('duplicate field')
        result[key] = value
    return result


def summarize(fixture: dict) -> dict:
    if (not isinstance(fixture, dict)
            or not isinstance(fixture.get('authority'), str)
            or not re.fullmatch(r'ssh-remote\+haco-win-ssh-[a-f0-9]{16}', fixture['authority'])
            or not isinstance(fixture.get('nonce'), str)
            or not re.fullmatch(r'[a-f0-9]{32}', fixture['nonce'])
            or not isinstance(fixture.get('result'), str)
            or not Path(fixture['result']).is_absolute()):
        raise ValueError('invalid fixture')
    observed = []
    invalid = False
    for phase in PHASES:
        path = Path(fixture['result'] + '.progress-' + phase)
        try:
            raw = read_signal(path)
            if len(raw) > MAX_SIGNAL_BYTES:
                invalid = True
                continue
            signal = json.loads(raw, object_pairs_hook=unique_fields)
            if signal != {'phase': phase, 'authority': fixture['authority'],
                          'nonce': fixture['nonce']}:
                invalid = True
                continue
            observed.append(phase)
        except FileNotFoundError:
            continue
        except (OSError, ValueError, TypeError):
            invalid = True
    return {'component': 'ci', 'operation': 'vscode_acceptance',
            'observer_progress': observed, 'invalid_progress': invalid}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest', type=Path, required=True)
    args = parser.parse_args()
    try:
        with args.manifest.open('rb') as stream:
            raw = stream.read(4097)
        if len(raw) > 4096:
            raise ValueError('oversized fixture')
        result = summarize(json.loads(raw, object_pairs_hook=unique_fields))
    except (OSError, ValueError, TypeError, KeyError, RecursionError):
        result = {'component': 'ci', 'operation': 'vscode_acceptance',
                  'observer_progress': [], 'invalid_progress': True}
    print(json.dumps(result))


if __name__ == '__main__':
    main()

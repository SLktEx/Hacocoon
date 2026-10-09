#!/usr/bin/env python3
"""Check actual offline VSIX contents against their runtime imports and icon."""
import json
import os
import stat
from pathlib import Path
import tempfile
import subprocess
import sys
import unittest
from unittest import mock
from types import SimpleNamespace
import zipfile

import package_vscode_acceptance
import package_vscode_notifications
import vscode_acceptance_diagnostics as diagnostics


class PackagingTests(unittest.TestCase):
    def test_optional_extension_contains_gui_and_adopted_icon(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'notifications.vsix'
            package_vscode_notifications.package(output)
            with zipfile.ZipFile(output) as archive:
                metadata = json.loads(archive.read('extension/package.json'))
                self.assertEqual(archive.read('extension/' + metadata['icon'])[:8], b'\x89PNG\r\n\x1a\n')
                self.assertIn(b"require('./review_panel')", archive.read('extension/review.js'))
                self.assertIn(b'module.exports = { panelHTML }', archive.read('extension/review_panel.js'))
                self.assertIn(b'ContentType="image/png"', archive.read('[Content_Types].xml'))
            with self.assertRaises(FileExistsError):
                package_vscode_notifications.package(output)

    def test_native_observer_contains_the_actual_renderer(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'observer.vsix'
            package_vscode_acceptance.package('win-ssh-' + 'a' * 16, output, Path(directory) / 'result.json')
            with zipfile.ZipFile(output) as archive:
                source = Path(__file__).resolve().parents[1] / 'clients/vscode-notify/review_panel.js'
                self.assertEqual(archive.read('extension/review_panel.js'), source.read_bytes())


class DiagnosticTests(unittest.TestCase):
    def fixture(self, directory):
        return {'authority': 'ssh-remote+haco-win-ssh-' + 'a' * 16,
                'nonce': 'b' * 32, 'result': str(Path(directory) / 'result.json')}

    def test_only_fresh_exact_allowlisted_signals_are_emitted(self):
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.fixture(directory)
            signal = {key: fixture[key] for key in ('authority', 'nonce')}
            signal['phase'] = 'activated'
            path = Path(fixture['result'] + '.progress-activated')
            path.write_text(json.dumps(signal))
            result = diagnostics.summarize(fixture)
            self.assertEqual(result['observer_progress'], ['activated'])
            self.assertFalse(result['invalid_progress'])
            for key, value in [('nonce', 'c' * 32), ('authority', 'secret-other-target'),
                               ('phase', 'secret-arbitrary-phase'), ('secret-extra-field', 'secret-value')]:
                with self.subTest(key=key):
                    path.write_text(json.dumps({**signal, key: value}))
                    result = diagnostics.summarize(fixture)
                    self.assertEqual(result['observer_progress'], [])
                    self.assertTrue(result['invalid_progress'])
                    self.assertNotIn('secret', json.dumps(result))

    def test_truncated_oversized_duplicate_and_nonobject_signals_are_invalid(self):
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.fixture(directory)
            path = Path(fixture['result'] + '.progress-activated')
            for raw in [b'{"phase":', b'secret' * 100, b'[]', b'null', b'"secret"',
                        b'{"phase":"activated","phase":"activated"}', b'\xff']:
                with self.subTest(raw=raw):
                    path.write_bytes(raw)
                    result = diagnostics.summarize(fixture)
                    self.assertEqual(result['observer_progress'], [])
                    self.assertTrue(result['invalid_progress'])
                    self.assertNotIn('secret', json.dumps(result))

    def test_missing_signals_do_not_claim_nonactivation_or_success(self):
        with tempfile.TemporaryDirectory() as directory:
            result = diagnostics.summarize(self.fixture(directory))
            self.assertEqual(result['observer_progress'], [])
            self.assertFalse(result['invalid_progress'])
            self.assertNotIn('status', result)

    def test_link_and_directory_signals_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.fixture(directory)
            path = Path(fixture['result'] + '.progress-activated')
            path.mkdir()
            self.assertTrue(diagnostics.summarize(fixture)['invalid_progress'])
            path.rmdir()
            try:
                path.symlink_to(Path(directory) / 'absent')
            except OSError:
                self.skipTest('local symlink creation unavailable')
            self.assertTrue(diagnostics.summarize(fixture)['invalid_progress'])

    def valid_signal(self, fixture):
        path = Path(fixture['result'] + '.progress-activated')
        path.write_text(json.dumps({'phase': 'activated', 'authority': fixture['authority'],
                                    'nonce': fixture['nonce']}))
        return path

    def assert_invalid_signal(self, fixture):
        self.assertEqual(diagnostics.summarize(fixture), {
            'component': 'ci', 'operation': 'vscode_acceptance',
            'observer_progress': [], 'invalid_progress': True})

    def test_hardlinked_signal_is_rejected_before_open(self):
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.fixture(directory)
            path = self.valid_signal(fixture)
            os.link(path, Path(directory) / 'other-link')
            with mock.patch.object(diagnostics.os, 'open', wraps=os.open) as opened:
                self.assert_invalid_signal(fixture)
                opened.assert_not_called()

    def test_same_inode_symlink_swap_at_open_is_rejected_without_nofollow(self):
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.fixture(directory)
            path = self.valid_signal(fixture)
            target = Path(directory) / 'moved-signal'
            # Probe local symlink support, not a Windows security privilege change.
            probe = Path(directory) / 'link-probe'
            try:
                probe.symlink_to(path)
            except OSError:
                self.skipTest('local symlink creation unavailable')
            probe.unlink()
            real_open = os.open

            def replace_at_open(name, flags):
                path.rename(target)
                path.symlink_to(target)
                return real_open(name, flags)

            # Exercise the portable fallback; this does not claim native Windows
            # race prevention or atomic no-follow support.
            with mock.patch.object(diagnostics.os, 'O_NOFOLLOW', 0, create=True), \
                    mock.patch.object(diagnostics.os, 'O_NONBLOCK', 0, create=True), \
                    mock.patch.object(diagnostics.os, 'open', side_effect=replace_at_open):
                self.assert_invalid_signal(fixture)

    def test_link_added_at_open_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.fixture(directory)
            path = self.valid_signal(fixture)
            real_open = os.open

            def link_at_open(name, flags):
                fd = real_open(name, flags)
                os.link(path, Path(directory) / 'new-link')
                return fd

            with mock.patch.object(diagnostics.os, 'open', side_effect=link_at_open):
                self.assert_invalid_signal(fixture)

    def test_observable_signal_changes_during_read_are_discarded(self):
        for change in ('replacement', 'symlink', 'hardlink', 'missing'):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as directory:
                fixture = self.fixture(directory)
                path = self.valid_signal(fixture)
                other = Path(directory) / 'other-signal'
                if change == 'symlink':
                    try:
                        other.symlink_to(path)
                    except OSError:
                        self.skipTest('local symlink creation unavailable')
                    other.unlink()
                real_fdopen = os.fdopen
                mutation_completed = False

                def change_after_read(fd, mode):
                    stream = real_fdopen(fd, mode)
                    wrapped = mock.MagicMock(wraps=stream)
                    wrapped.__enter__.return_value = wrapped
                    wrapped.__exit__.side_effect = stream.__exit__

                    def read(size):
                        nonlocal mutation_completed
                        raw = stream.read(size)
                        try:
                            if change == 'hardlink':
                                os.link(path, other)
                            elif change == 'missing':
                                path.unlink()
                            else:
                                path.rename(other)
                                if change == 'symlink':
                                    path.symlink_to(other)
                                else:
                                    path.write_bytes(raw)
                        except PermissionError:
                            self.skipTest('local filesystem denies open-file mutation')
                        mutation_completed = True
                        return raw

                    wrapped.read.side_effect = read
                    return wrapped

                with mock.patch.object(diagnostics.os, 'O_NOFOLLOW', 0, create=True), \
                        mock.patch.object(diagnostics.os, 'O_NONBLOCK', 0, create=True), \
                        mock.patch.object(diagnostics.os, 'fdopen', side_effect=change_after_read):
                    self.assert_invalid_signal(fixture)
                self.assertTrue(mutation_completed)

    def test_single_link_signal_remains_valid_without_optional_open_flags(self):
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.fixture(directory)
            self.valid_signal(fixture)
            with mock.patch.object(diagnostics.os, 'O_NOFOLLOW', 0, create=True), \
                    mock.patch.object(diagnostics.os, 'O_NONBLOCK', 0, create=True):
                self.assertEqual(diagnostics.summarize(fixture), {
                    'component': 'ci', 'operation': 'vscode_acceptance',
                    'observer_progress': ['activated'], 'invalid_progress': False})

    def test_descriptor_checks_reject_nonregular_linked_and_reparse_files(self):
        for sample in (1, 2):  # Opened descriptor before and after the read.
            for changed in ({'st_mode': stat.S_IFIFO}, {'st_nlink': 2},
                            {'st_file_attributes': 0x400}):
                with self.subTest(sample=sample, changed=changed), \
                        tempfile.TemporaryDirectory() as directory:
                    fixture = self.fixture(directory)
                    self.valid_signal(fixture)
                    real_fstat = os.fstat
                    calls = 0

                    def changed_metadata(fd):
                        nonlocal calls
                        calls += 1
                        metadata = real_fstat(fd)
                        if calls != sample:
                            return metadata
                        fields = {name: getattr(metadata, name) for name in
                                  ('st_mode', 'st_nlink', 'st_dev', 'st_ino')}
                        return SimpleNamespace(**{**fields, **changed})

                    with mock.patch.object(diagnostics.os, 'fstat', side_effect=changed_metadata):
                        self.assert_invalid_signal(fixture)

    def test_signal_disappearing_at_open_is_invalid_not_initially_missing(self):
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.fixture(directory)
            path = self.valid_signal(fixture)
            real_open = os.open

            def disappear_at_open(name, flags):
                path.unlink()
                return real_open(name, flags)

            with mock.patch.object(diagnostics.os, 'open', side_effect=disappear_at_open):
                self.assert_invalid_signal(fixture)

    def test_unknown_marker_names_are_not_read(self):
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.fixture(directory)
            Path(fixture['result'] + '.progress-secret-unknown').write_text('secret')
            self.assertEqual(diagnostics.summarize(fixture)['observer_progress'], [])

    def test_cli_failure_is_bounded_and_does_not_echo_manifest(self):
        with tempfile.TemporaryDirectory() as directory:
            manifest = Path(directory) / 'fixture.json'
            for raw in ['secret', '[' * 2000, json.dumps({'authority': 'secret'})]:
                with self.subTest(raw=raw[:20]):
                    manifest.write_text(raw)
                    result = subprocess.run([sys.executable, diagnostics.__file__, '--manifest', str(manifest)],
                                            capture_output=True, text=True, timeout=10, check=True)
                    self.assertEqual(result.stderr, '')
                    self.assertNotIn('secret', result.stdout)
                    self.assertNotIn(directory, result.stdout)
                    self.assertLess(len(result.stdout), 256)
                    self.assertTrue(json.loads(result.stdout)['invalid_progress'])

    def test_writer_phases_match_reader_allowlist(self):
        source = (Path(__file__).resolve().parent / 'vscode-acceptance/extension.js').read_text()
        import re
        phases = set(re.findall(r"(?:progress\(|stage = )'([a-z-]+)'", source))
        self.assertEqual(phases, set(diagnostics.PHASES))


if __name__ == '__main__':
    unittest.main()

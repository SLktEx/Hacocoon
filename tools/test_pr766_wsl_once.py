#!/usr/bin/env python3
"""Pure contracts: no WSL, shell, subprocess, network, or registry execution."""
import argparse
import copy
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import queue
import subprocess
import sys
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import Mock, patch

SOURCE = Path(__file__).parent / 'diagnostics' / 'pr766_wsl_once.py'
spec = importlib.util.spec_from_file_location('pr766_wsl_once', SOURCE)
p = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = p
spec.loader.exec_module(p)
NAME = 'HacoDiag766-123'
GUID = '{12345678-1234-1234-1234-123456789abc}'
ROW = {'guid': GUID, 'name': NAME, 'path': p.canonical(r'C:\fixture\distro')}
RECORD = {'name': NAME, 'import_succeeded': True, 'pre_absent': True,
          'registration': ROW, 'directory_identity': (1, 2), 'pre_guids': []}
EMPTY = {'services': [], 'processes': []}


class MemoryJournal:
    def __init__(self):
        self.data = {'events': [], 'outcome': 'inconclusive'}
    def save(self):
        pass
    def event(self, phase, **fields):
        self.data['events'].append({'phase': phase, **fields})


class Contracts(unittest.TestCase):
    def tearDown(self):
        p.CAPTURE_DEADLINE = None

    def test_owner_positive_and_every_changed_identity_refuses(self):
        p.assert_owned(RECORD, [ROW], r'C:\fixture\distro', (1, 2))
        for key, value in [('import_succeeded', False), ('pre_absent', False),
                           ('pre_guids', [GUID]), ('directory_identity', (1, 3))]:
            record = copy.deepcopy(RECORD)
            record[key] = value
            with self.subTest(key=key), self.assertRaises(p.Fault):
                p.assert_owned(record, [ROW], r'C:\fixture\distro', (1, 2))
        for rows in ([], [ROW, ROW], [{**ROW, 'name': NAME.lower()}],
                     [{**ROW, 'guid': '{87654321-1234-1234-1234-123456789abc}'}],
                     [{**ROW, 'path': p.canonical(r'C:\other')}], [{**ROW, 'guid': 'bad'}]):
            with self.subTest(rows=rows), self.assertRaises(p.Fault):
                p.assert_owned(RECORD, rows, r'C:\fixture\distro', (1, 2))

    def test_path_reparse_refusal_and_stable_directory_identity(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            self.assertEqual(p.path_identity(root), p.path_identity(root))
            link = root / 'link'
            try:
                link.symlink_to(root, target_is_directory=True)
            except OSError:
                return  # Windows non-elevated job may prohibit creating symlinks.
            with self.assertRaisesRegex(p.Fault, 'reparse_path'):
                p.path_identity(link)

    def test_windows_reparse_attribute_refuses_cleanup_without_symlink_privilege(self):
        target = Mock()
        target.parents = ()
        target.is_symlink.return_value = False
        target.lstat.return_value = SimpleNamespace(st_file_attributes=0x400)
        with patch.object(p, 'Path', return_value=target), patch.object(p, 'registrations', return_value=[ROW]), patch.object(p, 'capture') as capture:
            with self.assertRaisesRegex(p.Fault, 'reparse_path'):
                p.cleanup('wsl.exe', NAME, target, RECORD, MemoryJournal())
            capture.assert_not_called()
        with self.assertRaisesRegex(p.Fault, 'file_identity_unavailable'):
            p.file_identity(target)
        target.unlink.assert_not_called()

    def test_cleanup_only_unregisters_exact_proven_owner_once(self):
        journal, target = MemoryJournal(), Mock()
        target.exists.return_value = False
        with patch.object(p, 'assert_owned') as owner, patch.object(p, 'registrations', side_effect=[[ROW], [ROW], []]), patch.object(p, 'listed', side_effect=[[NAME], []]), patch.object(p, 'path_identity', return_value=(1, 2)), patch.object(p, 'capture', return_value=p.Result(0, '', '')) as capture:
            p.cleanup('wsl.exe', NAME, target, RECORD, journal)
            self.assertEqual(owner.call_count, 2)
            capture.assert_called_once_with(['wsl.exe', '--unregister', NAME], 45)
            self.assertEqual(journal.data['cleanup'], 'verified')

    def test_cleanup_ambiguity_never_unregisters(self):
        for listed in ([], [NAME, NAME]):
            journal = MemoryJournal()
            with patch.object(p, 'assert_owned'), patch.object(p, 'registrations', return_value=[ROW]), patch.object(p, 'listed', return_value=listed), patch.object(p, 'path_identity', return_value=(1, 2)), patch.object(p, 'capture') as capture:
                with self.assertRaises(p.Fault):
                    p.cleanup('wsl.exe', NAME, Mock(), RECORD, journal)
                capture.assert_not_called()
                self.assertEqual(journal.data['cleanup'], 'unconfirmed')
        with patch.object(p, 'assert_owned', side_effect=p.Fault('changed')), patch.object(p, 'registrations', return_value=[ROW]), patch.object(p, 'path_identity', return_value=(1, 2)), patch.object(p, 'capture') as capture:
            with self.assertRaises(p.Fault):
                p.cleanup('wsl.exe', NAME, Mock(), RECORD, MemoryJournal())
            capture.assert_not_called()

    def test_cleanup_residual_or_changed_is_not_verified(self):
        for kind in ('residual', 'changed'):
            target, journal = Mock(), MemoryJournal()
            target.exists.return_value = True
            target.iterdir.return_value = iter(['residual'])
            identities = [(1, 2), (1, 2), (1, 3) if kind == 'changed' else (1, 2)]
            with patch.object(p, 'assert_owned'), patch.object(p, 'registrations', side_effect=[[ROW], [ROW], []]), patch.object(p, 'listed', side_effect=[[NAME], []]), patch.object(p, 'path_identity', side_effect=identities), patch.object(p, 'capture', return_value=p.Result(0, '', '')):
                with self.assertRaisesRegex(p.Fault, 'cleanup_partial'):
                    p.cleanup('wsl.exe', NAME, target, RECORD, journal)
                self.assertEqual(journal.data['events'][-1]['directory_state'], kind)
                self.assertEqual(journal.data['cleanup'], 'unconfirmed')

    def test_image_cache_is_verified_not_trusted(self):
        with tempfile.TemporaryDirectory() as folder:
            image = Path(folder) / 'cache.wsl'
            image.write_bytes(b'fixture')
            with patch.object(p.urllib.request, 'urlopen') as request, patch.object(p, 'IMAGE_SHA', hashlib.sha256(b'fixture').hexdigest()):
                self.assertEqual(p.verified_image(image, Path(folder) / 'new.wsl', float('inf')), image)
                request.assert_not_called()
            with self.assertRaisesRegex(p.Fault, 'image_hash_mismatch'):
                p.verified_image(image, Path(folder) / 'new.wsl', float('inf'))

    def test_download_identity_hash_redirect_and_exclusive_creation(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            response = Mock()
            response.url = p.IMAGE_URL
            response.read.side_effect = [b'fixture', b'']
            response.__enter__ = Mock(return_value=response)
            response.__exit__ = Mock(return_value=False)
            with patch.object(p.urllib.request, 'urlopen', return_value=response), patch.object(p, 'IMAGE_SHA', hashlib.sha256(b'fixture').hexdigest()):
                image = p.verified_image(root / 'missing', root / 'new.wsl', float('inf'))
                self.assertEqual(image.read_bytes(), b'fixture')
            response.url = 'https://unapproved.example/image'
            with patch.object(p.urllib.request, 'urlopen', return_value=response):
                with self.assertRaisesRegex(p.Fault, 'image_redirect'):
                    p.verified_image(root / 'missing', root / 'redirect.wsl', float('inf'))

    def test_exact_owned_file_cleanup_never_touches_cache(self):
        for downloaded in (True, False):
            with tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                home = root / 'owned'
                home.mkdir()
                target = home / 'distro'
                target.mkdir()
                image = (home if downloaded else root) / 'ubuntu.wsl'
                image.write_bytes(b'fixture')
                record = {'directory_identity': p.path_identity(target)}
                p.cleanup_files(home, p.path_identity(home), target, record, image, p.file_identity(image), MemoryJournal())
                self.assertFalse(home.exists())
                self.assertEqual(image.exists(), not downloaded)

    def test_changed_file_and_residual_directory_survive(self):
        with tempfile.TemporaryDirectory() as folder:
            home = Path(folder) / 'owned'
            home.mkdir()
            target = home / 'distro'
            target.mkdir()
            image = home / 'ubuntu.wsl'
            image.write_bytes(b'fixture')
            identity = p.file_identity(image)
            image.write_bytes(b'replacement-data')
            with self.assertRaisesRegex(p.Fault, 'image_identity_changed'):
                p.cleanup_files(home, p.path_identity(home), target, {'directory_identity': p.path_identity(target)}, image, identity, MemoryJournal())
            self.assertEqual(image.read_bytes(), b'replacement-data')
            target.mkdir()
            (target / 'retained').write_text('keep')
            with self.assertRaises(OSError):
                p.cleanup_files(home, p.path_identity(home), target, {'directory_identity': p.path_identity(target)}, image, p.file_identity(image), MemoryJournal())
            self.assertTrue((target / 'retained').exists())

    def test_observation_redaction_and_schema(self):
        data = {'services': [{'name': 'WslService', 'state': 'Running', 'pid': 1, 'secret': 'TOKEN'}],
                'processes': [{'name': 'wsl.exe', 'pid': 2, 'parent': 1, 'created': '2026-10-11T00:00:00.000Z', 'CommandLine': 'SECRET'}], 'environment': 'PRIVATE'}
        result = json.dumps(p.selected_observations(data))
        for secret in ('TOKEN', 'SECRET', 'PRIVATE', 'CommandLine'):
            self.assertNotIn(secret, result)
        for bad in ({'services': [{}], 'processes': []}, {'services': [], 'processes': [{'name': 'wsl.exe', 'pid': True}]}, {'services': [], 'processes': [{}] * 65}, {'services': [{'name': 'WslService', 'state': 'sensitive'}], 'processes': []}):
            with self.assertRaises(p.Fault):
                p.selected_observations(bad)

    def test_version_receipts_and_error_redaction(self):
        self.assertEqual(p.version_fields('WSL version: 2.6.3.0\nSECRET: 1.2.3\nKernel version: 6.6.87.2\n'), {'WSL version': '2.6.3.0', 'Kernel version': '6.6.87.2'})
        with self.assertRaises(p.Fault):
            p.version_fields('SECRET: 1.2.3')
        self.assertEqual(p.version_fields('WSL version: 2.6.3.0\nKernel version: 6.6.87.2-1')['Kernel version'], '6.6.87.2-1')
        receipt = p.Result(-1, 'secret Wsl/Service/E_UNEXPECTED', 'credential').receipt()
        self.assertTrue(receipt['wsl_service_unexpected'])
        self.assertNotIn('credential', json.dumps(receipt))
        self.assertEqual(p.decode('name'.encode('utf-16-le')), 'name')

    def test_journal_io_failure_is_nonthrowing_and_recorded(self):
        with tempfile.TemporaryDirectory() as folder:
            journal = p.Journal(Path(folder) / 'record.json', {})
            journal.handle.close()
            with patch('builtins.print', side_effect=BrokenPipeError('sensitive')):
                journal.event('cleanup_started')
            journal.close()
            self.assertTrue(journal.data['sink_failed'])

    def test_optional_inventory_failure_is_explicit_redacted_and_non_gating(self):
        for error, category in ((p.Fault('observation_timeout'), 'observation_timeout'),
                                (OSError('PRIVATE'), 'OSError'),
                                (p.Fault('PRIVATE'), 'unrecognized_observation_error')):
            journal = MemoryJournal()
            journal.data.update(primary_failure='original', cleanup='verified')
            with patch.object(p, 'capture', side_effect=error) as capture:
                p.observe_outside_lifecycle(journal, 'inventory_after_cleanup')
            self.assertEqual(capture.call_args.args[1], 5)
            self.assertEqual(journal.data['events'][-1]['observations'], {'status': 'unavailable', 'category': category})
            self.assertTrue(journal.data['observations_incomplete'])
            self.assertEqual(journal.data['primary_failure'], 'original')
            self.assertEqual(journal.data['cleanup'], 'verified')
            self.assertNotIn('PRIVATE', json.dumps(journal.data))
        journal = MemoryJournal()
        with patch.object(p, 'capture', return_value=p.Result(0, json.dumps(EMPTY), '')):
            p.observe_outside_lifecycle(journal, 'inventory_before_import')
        self.assertEqual(journal.data['events'][-1]['observations'], {'status': 'available', **EMPTY})

    def test_inventory_process_uncertainty_is_sticky_without_masking_results(self):
        for category in ('observation_termination_unconfirmed', 'observation_pipe_unconfirmed', 'observation_timeout'):
            journal = MemoryJournal()
            journal.data.update(primary_failure='original', cleanup='verified', outcome='reproduced')
            fault = p.Fault(category, process_cleanup_unconfirmed=True)
            with patch.object(p, 'capture', side_effect=[fault, p.Result(0, json.dumps(EMPTY), '')]):
                p.observe_outside_lifecycle(journal, 'inventory_before_import')
                p.observe_outside_lifecycle(journal, 'inventory_after_cleanup')
            self.assertTrue(journal.data['inventory_process_cleanup_unconfirmed'])
            self.assertEqual(journal.data['events'][0]['observations']['process_cleanup'], 'unconfirmed')
            self.assertEqual(journal.data['primary_failure'], 'original')
            self.assertEqual(journal.data['cleanup'], 'verified')
            self.assertEqual(journal.data['outcome'], 'reproduced')

    def test_capture_success_output_limit_and_timeout_without_native_process(self):
        for content, limit, expected in ((b'hello', 10, None), (b'oversized', 3, 'observation_size_limit')):
            child = Mock(stdout=io.BytesIO(content), stderr=io.BytesIO(), returncode=0)
            child.poll.return_value = 0
            with patch.object(p.subprocess, 'Popen', return_value=child) as popen:
                if expected:
                    with self.assertRaisesRegex(p.Fault, expected):
                        p.capture(['fixed-observation'], 1, limit)
                else:
                    self.assertEqual(p.capture(['fixed-observation'], 1, limit).out, 'hello')
                self.assertEqual(popen.call_args.kwargs['bufsize'], 0)
                child.kill.assert_not_called()
        for failed_kill in (False, True):
            child = Mock(stdout=io.BytesIO(), stderr=io.BytesIO(), returncode=-1)
            child.poll.return_value = None
            if failed_kill:
                child.wait.side_effect = subprocess.TimeoutExpired('owned', 5)
            with patch.object(p.subprocess, 'Popen', return_value=child):
                with self.assertRaisesRegex(p.Fault, 'observation_termination_unconfirmed' if failed_kill else 'observation_timeout'):
                    p.capture(['fixed-observation'], 0)
                child.kill.assert_called_once()
                child.wait.assert_called_once_with(timeout=5)

    def test_expired_budget_starts_no_process(self):
        p.CAPTURE_DEADLINE = -1
        with patch.object(p.subprocess, 'Popen') as popen, self.assertRaisesRegex(p.Fault, 'capture_budget_expired'):
            p.capture(['fixed-observation'], 1)
        popen.assert_not_called()

    def test_reader_error_or_unconfirmed_pipe_never_yields_partial_success(self):
        stream = Mock()
        stream.read.side_effect = OSError('PRIVATE')
        child = Mock(stdout=stream, stderr=io.BytesIO(), returncode=0)
        child.poll.return_value = 0
        with patch.object(p.subprocess, 'Popen', return_value=child), self.assertRaisesRegex(p.Fault, 'observation_reader_failed'):
            p.capture(['fixed-observation'], 1)
        self.assertTrue(stream.close.called)
        stream.read.side_effect = None
        stream.read.return_value = b''
        stream.close.side_effect = OSError('PRIVATE CLOSE ERROR')
        child.stderr = io.BytesIO()
        with patch.object(p.subprocess, 'Popen', return_value=child), self.assertRaisesRegex(p.Fault, 'observation_reader_failed'):
            p.capture(['fixed-observation'], 1)
        thread = Mock()
        thread.is_alive.return_value = True
        with patch.object(p.subprocess, 'Popen', return_value=child), patch.object(p.threading, 'Thread', return_value=thread), self.assertRaisesRegex(p.Fault, 'observation_pipe_unconfirmed'):
            p.capture(['fixed-observation'], 1)
        self.assertTrue(thread.join.called)
        child.poll.return_value = None
        with patch.object(p.subprocess, 'Popen', return_value=child), patch.object(p.threading, 'Thread', return_value=thread), self.assertRaisesRegex(p.Fault, 'observation_timeout') as caught:
            p.capture(['fixed-observation'], 0)
        self.assertTrue(caught.exception.process_cleanup_unconfirmed)

    def test_console_ctrl_c_normal_exit_and_failure_cleanup(self):
        class FakeConsole:
            pid, exitstatus = 42, 0
            def __init__(self):
                self.queue, self.alive, self.writes = queue.Queue(), True, []
                self.queue.put('C:\\fixture>')
            def read(self, size):
                item = self.queue.get(timeout=1)
                if item is None:
                    raise EOFError()
                return item
            def write(self, value):
                self.writes.append(value)
                if self.fail_write:
                    raise p.Fault('console_output_limit')
                if value == '\x03':
                    self.queue.put('\r\nDIAG_INT\r\nC:\\fixture>')
                elif value == 'exit\r\n':
                    self.alive = False
                    self.queue.put(None)
                else:
                    self.queue.put('\r\nDIAG_READY\r\n')
            def isalive(self):
                return self.alive
            def terminate(self, force):
                self.alive = False
                self.queue.put(None)
                return True
        for failed in (False, True):
            console = FakeConsole()
            console.fail_write = failed
            spawn = Mock(return_value=console)
            output = io.StringIO()
            module = SimpleNamespace(PtyProcess=SimpleNamespace(spawn=spawn))
            with patch.dict(sys.modules, {'winpty': module}), patch.object(p, 'capture', side_effect=AssertionError('no inventory inside console')), patch('sys.stdout', output):
                if failed:
                    with self.assertRaisesRegex(p.Fault, 'console_output_limit'):
                        p.console_child(NAME, r'C:\\Windows\\System32\\wsl.exe')
                else:
                    p.console_child(NAME, r'C:\\Windows\\System32\\wsl.exe')
            self.assertEqual(spawn.call_args.kwargs['backend'], '0')
            rows = [json.loads(row) for row in output.getvalue().splitlines()]
            if failed:
                self.assertEqual(rows, [{'phase': 'failure_cleanup', 'state': 'terminated'}])
                self.assertNotIn('\x03', console.writes)
            else:
                self.assertEqual([row['phase'] for row in rows], ['ready', 'ctrl_c_sent', 'wsl_returned', 'cmd_exited'])
                self.assertEqual(console.writes[1:], ['\x03', 'exit\r\n'])
                self.assertFalse(console.alive)

    def test_child_receipts_required_and_hostile_error_text_redacted(self):
        for error, expected in (('console_timeout', 'console_timeout'), ('PRIVATE SECRET', 'unrecognized_child_error')):
            journal = MemoryJournal()
            result = p.Result(1, json.dumps({'phase': 'failure_cleanup', 'state': 'unconfirmed'}), json.dumps({'error': error}))
            with patch.object(p, 'capture', return_value=result), self.assertRaisesRegex(p.Fault, 'console_sequence_failed'):
                p.run_console(NAME, 'wsl.exe', journal)
            self.assertEqual(journal.data['events'][-1]['category'], expected)
            self.assertNotIn('PRIVATE', json.dumps(journal.data))
        rows = [{'phase': phase, 'cmd_pid': 12, 'observations': dict(p.CRITICAL_OBSERVATIONS), 'monotonic_ns': index + 1, 'utc': '2026-10-11T00:00:00+00:00'} for index, phase in enumerate(('ready', 'ctrl_c_sent', 'wsl_returned', 'cmd_exited'))]
        rows[-1].update(exit_code=0, reader='eof')
        journal = MemoryJournal()
        with patch.object(p, 'capture', return_value=p.Result(0, '\n'.join(map(json.dumps, rows)), '')):
            p.run_console(NAME, 'wsl.exe', journal)
        self.assertEqual(journal.data['process_cleanup'], 'normal_exit')
        with patch.object(p, 'capture', return_value=p.Result(0, json.dumps(rows[1]), '')), self.assertRaisesRegex(p.Fault, 'console_receipt_invalid'):
            p.run_console(NAME, 'wsl.exe', MemoryJournal())
        for key, value in (('exit_code', None), ('exit_code', 1), ('exit_code', False), ('reader', None), ('reader', 'failed'), ('cmd_pid', 13), ('cmd_pid', True), ('cmd_pid', 0), ('monotonic_ns', 1), ('utc', 'private'), ('observations', EMPTY), ('observations', {'status': 'available'})):
            bad = copy.deepcopy(rows)
            bad[-1][key] = value
            with self.subTest(key=key, value=value), patch.object(p, 'capture', return_value=p.Result(0, '\n'.join(map(json.dumps, bad)), '')), self.assertRaises(p.Fault):
                p.run_console(NAME, 'wsl.exe', MemoryJournal())

    def test_experiment_stops_at_first_failure_and_classifies_final(self):
        with patch.object(p, 'observe_outside_lifecycle', side_effect=AssertionError('no inventory inside lifecycle')), patch.object(p, 'capture', return_value=p.Result(1, '', '')) as capture, patch.object(p, 'run_console') as console:
            with self.assertRaisesRegex(p.Fault, 'baseline_failed'):
                p.experiment('wsl.exe', NAME, MemoryJournal())
            self.assertEqual(capture.call_count, 1)
            console.assert_not_called()
        for final, expected in ((p.Result(0, '', ''), 'not_reproduced'), (p.Result(-1, '', 'Wsl/Service/E_UNEXPECTED'), 'reproduced'), (p.Result(1, '', 'another failure'), 'inconclusive')):
            journal = MemoryJournal()
            with patch.object(p, 'observe_outside_lifecycle', side_effect=AssertionError('no inventory inside lifecycle')), patch.object(p, 'capture', side_effect=[p.Result(0, '', ''), final]) as capture, patch.object(p, 'run_console') as console:
                p.experiment('wsl.exe', NAME, journal)
                self.assertEqual(capture.call_count, 2)
                console.assert_called_once()
                self.assertEqual(journal.data['outcome'], expected)
        with patch.object(p, 'observe_outside_lifecycle', side_effect=AssertionError('no inventory inside lifecycle')), patch.object(p, 'capture', return_value=p.Result(0, '', '')) as capture, patch.object(p, 'run_console', side_effect=p.Fault('console_failed')):
            with self.assertRaises(p.Fault):
                p.experiment('wsl.exe', NAME, MemoryJournal())
            self.assertEqual(capture.call_count, 1)

    def test_workflow_is_one_create_attempt_and_read_only(self):
        workflow = (SOURCE.parents[2] / '.github/workflows/pr766-wsl-once.yml').read_text()
        for text in ('on: create', 'github.run_attempt == 1', p.BRANCH, 'timeout-minutes: 15', 'contents: read', 'persist-credentials: false', 'if: always()'):
            self.assertIn(text, workflow)
        for text in ('workflow_dispatch', 'pull_request:', 'push:', 'contents: write', 'secrets.', 'actions/cache/save'):
            self.assertNotIn(text, workflow)
        source = SOURCE.read_text()
        self.assertIn('backend="0"', source)
        for forbidden in ('--shutdown', '--terminate', '--update', 'shutil.rmtree', 'Set-ExecutionPolicy'):
            self.assertNotIn(forbidden, source)

    def test_full_orchestration_cleanup_preserves_primary_failure(self):
        for mode in ('pass', 'experiment_failed', 'cleanup_failed', 'import_failed'):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                image = root / 'cached.wsl'
                image.write_bytes(b'fixture')
                output = root / 'result.json'
                target = root / 'pr766-once-123' / 'distro'
                row = {**ROW, 'path': p.canonical(target)}
                env = {'RUNNER_TEMP': str(root), 'SystemRoot': str(root), 'GITHUB_SHA': 'a' * 40, 'ImageVersion': '20260925.250.1'}
                def experiment(wsl, name, journal):
                    if mode in ('experiment_failed', 'cleanup_failed'):
                        raise p.Fault('original_failure')
                    journal.data['outcome'] = 'not_reproduced'
                def cleanup(wsl, name, target, record, journal):
                    if mode == 'cleanup_failed':
                        raise p.Fault('secondary_failure')
                    journal.data['cleanup'] = 'verified'
                results = [p.Result(0, 'WSL version: 2.6.3.0', ''), p.Result(1 if mode == 'import_failed' else 0, '', '')]
                with patch.dict(os.environ, env), patch.object(p, 'require_context', return_value='123'), patch.object(p, 'observe_outside_lifecycle'), patch.object(p, 'verified_image', return_value=image), patch.object(p, 'registrations', side_effect=[[], [row], [row]]), patch.object(p, 'listed', return_value=[]), patch.object(p, 'capture', side_effect=results), patch.object(p, 'experiment', side_effect=experiment), patch.object(p, 'cleanup', side_effect=cleanup) as cleanup_mock, patch('builtins.print'):
                    status = p.run(argparse.Namespace(output=output, image=image))
                data = json.loads(output.read_text())
                self.assertEqual(status, 0 if mode == 'pass' else 1)
                if mode == 'import_failed':
                    cleanup_mock.assert_not_called()
                    self.assertEqual(data['cleanup'], 'not_imported_or_ownership_unconfirmed')
                else:
                    cleanup_mock.assert_called_once()
                if mode in ('experiment_failed', 'cleanup_failed'):
                    self.assertEqual(data['primary_failure'], 'original_failure')
                if mode == 'cleanup_failed':
                    self.assertEqual(data['cleanup_failure'], 'secondary_failure')
                self.assertTrue(image.exists())  # Shared cache is never removed.

    def test_inventory_timeout_cannot_prevent_actual_lifecycle_or_cleanup(self):
        for uncertain in (False, True):
            with self.subTest(uncertain=uncertain), tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                image = root / 'cached.wsl'
                image.write_bytes(b'fixture')
                output = root / 'result.json'
                row = {**ROW, 'path': p.canonical(root / 'pr766-once-123' / 'distro')}
                env = {'RUNNER_TEMP': str(root), 'SystemRoot': str(root), 'GITHUB_SHA': 'a' * 40, 'ImageVersion': '20260925.250.1'}
                sequence = []
                def capture(argv, timeout):
                    if argv[0] == 'powershell.exe':
                        sequence.append('inventory')
                        self.assertEqual(timeout, 5)
                        raise p.Fault('observation_timeout', process_cleanup_unconfirmed=uncertain)
                    if argv[1] == '--version':
                        return p.Result(0, 'WSL version: 2.7.14.0', '')
                    self.assertEqual(argv[1], '--import')
                    sequence.append('import')
                    return p.Result(0, '', '')
                def experiment(wsl, name, journal):
                    sequence.append('lifecycle')
                    journal.data['outcome'] = 'not_reproduced'
                def cleanup(wsl, name, target, record, journal):
                    sequence.append('cleanup')
                    journal.data['cleanup'] = 'verified'
                with patch.dict(os.environ, env), patch.object(p, 'require_context', return_value='123'), patch.object(p, 'verified_image', return_value=image), patch.object(p, 'registrations', side_effect=[[], [row], [row]]), patch.object(p, 'listed', return_value=[]), patch.object(p, 'capture', side_effect=capture), patch.object(p, 'experiment', side_effect=experiment), patch.object(p, 'cleanup', side_effect=cleanup), patch('builtins.print'):
                    self.assertEqual(p.run(argparse.Namespace(output=output, image=image)), 1 if uncertain else 0)
                self.assertEqual(sequence, ['inventory', 'import', 'lifecycle', 'cleanup', 'inventory'])
                data = json.loads(output.read_text())
                self.assertEqual(data['outcome'], 'not_reproduced')
                self.assertEqual(data['cleanup'], 'verified')
                self.assertTrue(data['observations_incomplete'])
                self.assertNotIn('primary_failure', data)

if __name__ == '__main__':
    unittest.main(verbosity=2)

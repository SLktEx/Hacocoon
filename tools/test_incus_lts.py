#!/usr/bin/env python3
"""Run the shipped package helper with command-boundary fakes, never host writes."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
HELPER = ROOT / 'scripts/incus-lts.sh'
FPR = '4EFC590696CB15B87C73A3AD82CC8797C838DCFD'
KEY = 'pub:::::::::\nfpr:::::::::' + FPR + ':\n'
SOURCE = 'https://pkgs.zabbly.com/incus/lts-7.0'


class LTS(unittest.TestCase):
    def test_every_native_ci_setup_uses_the_same_install_and_version_gate(self):
        # CI routing is part of the supported-substrate contract. A distro
        # package install here previously let Core/Btrfs silently stay on 6.x.
        for name in ('ci-incus.sh', 'ci-incus-core.sh'):
            with self.subTest(helper=name):
                source = (ROOT / 'tools' / name).read_text()
                self.assertIn('/incus-lts.sh"', source)
                self.assertIn('sh "$INCUS_LTS_HELPER" install', source)
                self.assertIn('sh "$INCUS_LTS_HELPER" verify-version "$server_version"', source)
                self.assertNotIn('ge 6.0', source)
                commands = source.replace('\\\n', ' ').splitlines()
                for command in commands:
                    if 'apt-get install' in command:
                        self.assertNotRegex(command, r'\bincus(?:-base|-client)?\b')

    def install(self, *, keys=KEY, packages=None, installed='', fail=''):
        if packages is None:
            packages = '\n'.join(f'incus-base | {v} | {uri} resolute/main amd64 Packages' for v, uri in [
                ('1:7.0.1-ubuntu26.04-1', SOURCE), ('1:7.0.9-ubuntu26.04-2', SOURCE),
                ('1:7.1.0-ubuntu26.04-1', SOURCE), ('1:7.0.99-ubuntu26.04-1', SOURCE + '/hostile')])
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'config.json').write_text(json.dumps(dict(keys=keys, packages=packages, installed=installed, fail=fail)))
            fake = root / 'command.py'
            fake.write_text('''#!/usr/bin/python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['FIXTURE'])
config = json.loads((root / 'config.json').read_text())
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
with (root / 'trace').open('a') as f: f.write(json.dumps([name, args]) + '\\n')
if name == config['fail']: sys.exit(42)
if name == 'id': print('0')
elif name == 'curl': pathlib.Path(args[args.index('-o')+1]).write_text('fixture key')
elif name == 'gpg': print(config['keys'])
elif name == 'dpkg-query': print(config['installed'])
elif name == 'apt-cache': print(config['packages'])
elif name == 'install' and args[0] != '-d':
    (root / pathlib.Path(args[-1]).name).write_bytes(pathlib.Path(args[-2]).read_bytes())
elif name not in ('apt-get', 'install'): sys.exit(99)
''')
            fake.chmod(0o755)
            for command in ('id', 'curl', 'gpg', 'dpkg-query', 'apt-cache', 'apt-get', 'install'):
                (root / command).symlink_to(fake)
            env = dict(os.environ, FIXTURE=str(root), PATH=str(root) + ':' + os.environ['PATH'], TMPDIR=str(root))
            result = subprocess.run(['sh', str(HELPER), 'install'], env=env, text=True, capture_output=True)
            trace = [json.loads(l) for l in (root / 'trace').read_text().splitlines()]
            outputs = {p.name: p.read_text() for p in root.iterdir() if p.name in ('hacocoon-incus-lts', 'zabbly-incus-lts-7.0.sources')}
            self.assertFalse(any(p.is_dir() for p in root.iterdir()), 'temporary key/source directory leaked')
            return result, trace, outputs

    def test_greatest_patch_from_exact_source_without_persisted_patch_pin(self):
        result, trace, outputs = self.install()
        self.assertEqual(result.returncode, 0, result.stderr)
        install = [args for name, args in trace if name == 'apt-get' and args[0] == 'install']
        self.assertEqual(len(install), 1)
        self.assertIn('incus-base=1:7.0.9-ubuntu26.04-2', install[0])
        self.assertIn('incus-client=1:7.0.9-ubuntu26.04-2', install[0])
        self.assertIn('Pin: version 1:7.0.*', outputs['hacocoon-incus-lts'])
        self.assertIn('Pin-Priority: -1', outputs['hacocoon-incus-lts'])
        self.assertNotIn('7.0.9', outputs['hacocoon-incus-lts'])
        self.assertIn('Signed-By: /etc/apt/keyrings/zabbly.asc', outputs['zabbly-incus-lts-7.0.sources'])

    def test_untrusted_extra_missing_duplicate_keys_fail_before_host_writes(self):
        for keys in ('', KEY.replace(FPR, '0'*40), KEY + KEY.replace(FPR, '0'*40), KEY + KEY):
            with self.subTest(keys=keys):
                result, trace, _ = self.install(keys=keys)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(any(name in ('install', 'apt-get') for name, _ in trace))

    def test_backend_failures_never_install_a_package(self):
        for failure in ('curl', 'gpg', 'apt-cache', 'apt-get'):
            with self.subTest(failure=failure):
                result, trace, _ = self.install(fail=failure)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(any(name == 'apt-get' and args[0] == 'install' for name, args in trace))

    def test_wrong_series_or_source_never_installs(self):
        for packages in ('', f'incus-base | 1:7.1.0-1 | {SOURCE} resolute/main amd64 Packages',
                         f'incus-base | 1:7.0.0-1 | {SOURCE} resolute/main amd64 Packages',
                         f'incus-base | --force-yes | {SOURCE} resolute/main amd64 Packages',
                         f'incus-base | 1:7.0.9-1 | {SOURCE}.invalid resolute/main amd64 Packages'):
            result, trace, _ = self.install(packages=packages)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(any(name == 'apt-get' and args[0] == 'install' for name, args in trace))

    def test_newer_installed_series_does_not_get_downgraded(self):
        for version in ('1:7.2.0-1', '7.1.0-1', '7.2.0-1', '0:8.0.1-1'):
            with self.subTest(version=version):
                result, trace, _ = self.install(installed=version)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(any(name in ('curl', 'install', 'apt-get') for name, _ in trace))

    def test_server_version_is_bounded_and_fails_closed(self):
        for version, accepted in [('7.0.1', True), ('7.0.99', True), ('6.0.5', False), ('7.0.0', False),
                                  ('7.1.0', False), ('7.0.1-rc1', False), ('', False), ('--help', False),
                                  ('7.0.1\n7.0.2', False), ('\x1b[31m7.0.1', False)]:
            result = subprocess.run(['sh', str(HELPER), 'verify-version', version], capture_output=True, text=True)
            self.assertEqual(result.returncode == 0, accepted, result.stderr)
            self.assertNotIn('\x1b', result.stderr)


if __name__ == '__main__':
    unittest.main()

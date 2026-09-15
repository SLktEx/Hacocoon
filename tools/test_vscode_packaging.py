#!/usr/bin/env python3
"""Check actual offline VSIX contents against their runtime imports and icon."""
import json
from pathlib import Path
import tempfile
import unittest
import zipfile

import package_vscode_acceptance
import package_vscode_notifications


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


if __name__ == '__main__':
    unittest.main()

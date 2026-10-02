#!/usr/bin/env python3
"""Security regressions for release inventories; binary verification runs in CI."""
import importlib.util
import io
import tarfile
import tempfile
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('release_packages', Path(__file__).with_name('check-release-packages.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class InventoryTests(unittest.TestCase):
    def test_runtime_secrets_and_databases_are_refused(self):
        for path in ['release/.env', 'release/static/.env', 'release/static/.env.production', 'release/static/book.db', 'release/data/cache.js', 'release/backups/snapshot', 'release/.git/config']:
            with self.subTest(path=path), self.assertRaises(ValueError):
                module.safe_asset(path, 'release')

    def test_traversal_and_unexpected_files_are_refused(self):
        for path in ['/release/static/a.js', 'release/../a', 'other/static/a.js', 'release/static\\a.js', 'release/config.yaml']:
            with self.subTest(path=path), self.assertRaises(ValueError):
                module.safe_asset(path, 'release')

    def test_template_assets_and_environment_example_are_allowed(self):
        for path in ['release/.env.example', 'release/static/fonts/notosansarabic.ttf', 'release/templates/base.html', 'release/BUILD-INFO.txt']:
            module.safe_asset(path, 'release')


class ArchiveTests(unittest.TestCase):
    def test_links_and_duplicate_members_are_refused(self):
        root = 'defta-librairie-1.7.2-linux-amd64'
        for kind in ('link', 'duplicate'):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as directory:
                archive = Path(directory) / (root + '.tar.gz')
                with tarfile.open(archive, 'w:gz') as container:
                    entry = tarfile.TarInfo(root + '/static/a.js')
                    if kind == 'link':
                        entry.type = tarfile.SYMTYPE
                        entry.linkname = '/etc/passwd'
                        container.addfile(entry)
                    else:
                        entry.size = 1
                        container.addfile(entry, io.BytesIO(b'a'))
                        container.addfile(entry, io.BytesIO(b'b'))
                with self.assertRaisesRegex(ValueError, 'Links|Duplicate'):
                    module.verify(archive, '1.7.2', 'a' * 40)


if __name__ == '__main__':
    unittest.main()

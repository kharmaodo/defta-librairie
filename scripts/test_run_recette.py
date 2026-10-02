import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('launcher', Path(__file__).with_name('run-recette.py'))
launcher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(launcher)


class RecipeTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.source = self.root / 'source'
        self.source.mkdir()
        (self.source / 'cmd').mkdir()
        (self.source / 'go.mod').write_text('module example.test/recipe\n')
        (self.root / 'defta-restored.db').write_bytes(b'copy')
        self.services = self.root / 'services.env'
        self.services.write_text('MINIO_ACCESS_KEY=test\nMINIO_SECRET_KEY=test\nNATS_USER=test\nNATS_PASSWORD=test\nMINIO_BUCKET_COVERS=recipe\nMINIO_API_BIND=127.0.0.1:19000\nNATS_CLIENT_BIND=127.0.0.1:14222\n')
        self.services.chmod(0o600)

    def test_modes_and_active_environment_isolation(self):
        with patch.dict(os.environ, {'JWT_SECRET': 'active', 'DB_PATH': '/active.db', 'NATS_URL': 'nats://active:4222'}):
            _, local = launcher.prepare(self.root, self.source, '')
            _, tunnel = launcher.prepare(self.root, self.source, 'https://example.ngrok-free.dev')
        self.assertEqual(local['AUTH_COOKIE_SECURE'], 'false')
        self.assertEqual(tunnel['AUTH_COOKIE_SECURE'], 'true')
        self.assertEqual(local['DB_PATH'], str(self.root / 'defta-restored.db'))
        self.assertEqual(local['NATS_URL'], 'nats://127.0.0.1:14222')
        self.assertNotEqual(local['JWT_SECRET'], 'active')
        self.assertNotEqual(local['JWT_SECRET'], tunnel['JWT_SECRET'])
        self.assertEqual((self.root / 'defta-restored.db').read_bytes(), b'copy')

    def test_invalid_origins(self):
        for value in ['http://example.test', 'https://user:pass@example.test', 'https://example.test/', 'https://example.test?x=y', 'https://example.test:bad', 'https://example.test#x']:
            with self.subTest(value=value), self.assertRaises(ValueError):
                launcher.prepare(self.root, self.source, value)

    def test_env_is_preserved_and_refused(self):
        existing = self.source / '.env'
        existing.write_text('private sentinel')
        with self.assertRaises(ValueError):
            launcher.prepare(self.root, self.source, '')
        self.assertEqual(existing.read_text(), 'private sentinel')

    def test_private_file_and_database_links(self):
        self.services.chmod(0o644)
        with self.assertRaises(ValueError):
            launcher.prepare(self.root, self.source, '')
        self.services.chmod(0o600)
        db = self.root / 'defta-restored.db'
        db.rename(self.root / 'active.db')
        db.symlink_to(self.root / 'active.db')
        with self.assertRaises(ValueError):
            launcher.prepare(self.root, self.source, '')

    def test_config_injection_is_refused(self):
        with self.services.open('a') as stream:
            stream.write('DB_PATH=/active.db\n')
        with self.assertRaises(ValueError):
            launcher.prepare(self.root, self.source, '')


if __name__ == '__main__':
    unittest.main()

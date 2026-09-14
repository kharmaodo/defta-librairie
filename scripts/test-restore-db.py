#!/usr/bin/env python3
"""Offline restoration acceptance tests using temporary databases only."""
import hashlib
import importlib.util
from pathlib import Path
import sqlite3
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("restore_db", ROOT / "scripts/restore-db.py")
restore_db = importlib.util.module_from_spec(spec)
spec.loader.exec_module(restore_db)


class RestoreTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.source = self.root / "backup.db"
        self.output = self.root / "restored.db"
        db = sqlite3.connect(self.source)
        try:
            db.execute("CREATE TABLE schema_migrations(version TEXT PRIMARY KEY)")
            for migration in sorted((ROOT / "internal/migrations/sql").glob("*.sql")):
                db.executescript(migration.read_text())
                db.execute("INSERT INTO schema_migrations VALUES(?)", (migration.name,))
                db.commit()
        finally:
            db.close()

    def test_restores_committed_wal_and_checks_digest(self):
        live = sqlite3.connect(self.source)
        try:
            live.execute("PRAGMA journal_mode=WAL")
            live.execute("PRAGMA wal_autocheckpoint=0")
            live.execute("INSERT INTO defta(title,price) VALUES(?,?)", ("كتاب restauration", 1500))
            live.commit()
            self.assertGreater(Path(str(self.source) + "-wal").stat().st_size, 0)
            result = restore_db.restore(self.source, self.output)
            self.assertEqual(result["sha256"], hashlib.sha256(self.output.read_bytes()).hexdigest())
            restored = sqlite3.connect(self.output)
            try:
                self.assertEqual(restored.execute("SELECT title,price FROM defta").fetchall(), [("كتاب restauration", 1500)])
                self.assertEqual(restored.execute("PRAGMA journal_mode").fetchone()[0], "delete")
                restore_db.validate(restored)
            finally:
                restored.close()
            self.assertEqual(self.output.stat().st_mode & 0o777, 0o600)
        finally:
            live.close()

    def test_existing_destination_is_unchanged(self):
        self.output.write_bytes(b"do not overwrite")
        with self.assertRaises(ValueError):
            restore_db.restore(self.source, self.output)
        self.assertEqual(self.output.read_bytes(), b"do not overwrite")

    def test_leftover_sidecar_and_symlink_are_rejected(self):
        sidecar = Path(str(self.output) + "-wal")
        sidecar.write_bytes(b"wal")
        with self.assertRaises(ValueError):
            restore_db.restore(self.source, self.output)
        sidecar.unlink()
        self.output.symlink_to(self.root / "missing")
        with self.assertRaises(ValueError):
            restore_db.restore(self.source, self.output)

    def test_corrupt_source_does_not_publish(self):
        self.source.write_bytes(b"not SQLite")
        with self.assertRaises(sqlite3.Error):
            restore_db.restore(self.source, self.output)
        self.assertFalse(self.output.exists())
        self.assertEqual(list(self.root.glob(".defta-restore-*")), [])

    def test_foreign_key_violation_does_not_publish(self):
        db = sqlite3.connect(self.source)
        try:
            db.execute("INSERT INTO library_settings(library_id,updated_at) VALUES('missing','now')")
            db.commit()
        finally:
            db.close()
        with self.assertRaisesRegex(ValueError, "clé étrangère"):
            restore_db.restore(self.source, self.output)
        self.assertFalse(self.output.exists())

    def test_unrelated_database_is_rejected(self):
        unrelated = self.root / "unrelated.db"
        db = sqlite3.connect(unrelated)
        db.execute("CREATE TABLE unrelated(id)")
        db.close()
        with self.assertRaisesRegex(ValueError, "schéma"):
            restore_db.restore(unrelated, self.output)
        self.assertFalse(self.output.exists())


if __name__ == "__main__":
    unittest.main(verbosity=2)

#!/usr/bin/env python3
"""Restore a SQLite backup into a new file, never over the running database."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import tempfile
import time

REQUIRED_TABLES = {"defta", "users", "libraries", "schema_migrations"}


def validate(db):
    integrity = db.execute("PRAGMA integrity_check").fetchall()
    if integrity != [("ok",)]:
        raise ValueError("Le contrôle d’intégrité SQLite a échoué.")
    if db.execute("PRAGMA foreign_key_check").fetchone() is not None:
        raise ValueError("Une violation de clé étrangère a été détectée.")
    tables = {row[0] for row in db.execute("SELECT name FROM sqlite_master WHERE type='table'")}
    if not REQUIRED_TABLES <= tables:
        raise ValueError("La sauvegarde ne contient pas le schéma Defta attendu.")
    if db.execute("SELECT COUNT(*) FROM schema_migrations").fetchone()[0] == 0:
        raise ValueError("La sauvegarde ne contient aucune migration enregistrée.")


def vacant(output):
    for name in (output, Path(str(output) + "-wal"), Path(str(output) + "-shm"), Path(str(output) + "-journal")):
        if os.path.lexists(name):
            raise ValueError("La destination ou un fichier SQLite associé existe déjà ; choisir un nouveau nom.")


def restore(source, output):
    source = Path(source).resolve(strict=True)
    # Resolve the parent only, so a dangling symlink at the output is rejected.
    raw = Path(output).absolute()
    output = raw.parent.resolve(strict=True) / raw.name
    if not source.is_file():
        raise ValueError("La source doit être un fichier SQLite.")
    vacant(output)
    fd, stage_name = tempfile.mkstemp(prefix=".defta-restore-", suffix=".db", dir=output.parent)
    os.close(fd)
    stage = Path(stage_name)
    deadline = time.monotonic() + 120

    def progress(status, remaining, total):
        if time.monotonic() > deadline:
            raise TimeoutError("Restauration interrompue après deux minutes ; vérifier la disponibilité de la source.")

    try:
        # SQLite's backup API includes committed WAL pages; raw file copying does not.
        src = sqlite3.connect(source.as_uri() + "?mode=ro", uri=True, timeout=5)
        try:
            dest = sqlite3.connect(stage, timeout=5)
            try:
                src.backup(dest, pages=256, progress=progress, sleep=0.05)
                dest.execute("PRAGMA journal_mode=DELETE")
                validate(dest)
                migrations = dest.execute("SELECT COUNT(*) FROM schema_migrations").fetchone()[0]
            finally:
                dest.close()
        finally:
            src.close()
        digest = hashlib.sha256()
        with stage.open("rb") as stream:
            for chunk in iter(lambda: stream.read(1024 * 1024), b""):
                digest.update(chunk)
            os.fsync(stream.fileno())
        vacant(output)
        # Atomic publication without overwriting an existing destination.
        os.link(stage, output)
        stage.unlink()
        directory_fd = os.open(output.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory_fd)
        finally:
            os.close(directory_fd)
        return {"status": "restored", "output": str(output), "sha256": digest.hexdigest(), "migrations": migrations}
    finally:
        for suffix in ("", "-wal", "-shm", "-journal"):
            Path(str(stage) + suffix).unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", help="Sauvegarde SQLite à restaurer")
    parser.add_argument("--output", required=True, help="Nouveau fichier ; répertoire parent existant")
    args = parser.parse_args()
    try:
        result = restore(args.source, args.output)
    except (OSError, sqlite3.Error, ValueError, TimeoutError) as exc:
        parser.exit(1, f"Restauration non confirmée : {exc}\nLa destination éventuellement publiée doit être vérifiée avant utilisation.\n")
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()

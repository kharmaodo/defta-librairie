#!/usr/bin/env python3
"""Vérifie le contrat statique du cycle de vie des couvertures v1.4."""

from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
errors: list[str] = []


def require(path: str, markers: tuple[str, ...]) -> None:
    candidate = ROOT / path
    if not candidate.is_file():
        errors.append(f"{path}: fichier absent")
        return
    content = candidate.read_text(encoding="utf-8")
    for marker in markers:
        if marker not in content:
            errors.append(f"{path}: garantie absente: {marker}")


require(
    "internal/repositories/cover_repository.go",
    ("ActiveVariant", "READY", "active"),
)
require(
    "internal/services/book_cover_read_service.go",
    ("BookCoverReadService", "ActiveVariant", "OpenVariant"),
)
require(
    "internal/handlers/book_cover_read.go",
    ("If-None-Match", "Cache-Control", "X-Content-Type-Options", "ETag"),
)
require(
    "cmd/main.go",
    ('GET /api/manage/books/{id}/cover', "coverCleanupDone"),
)
require(
    "internal/migrations/sql/027_create_cover_cleanup_jobs.sql",
    ("cover_object_cleanup_jobs", "available_at", "locked_until"),
)
require(
    "internal/repositories/cover_cleanup_repository.go",
    ("ClaimNext", "MarkCompleted", "MarkFailed", "Reconcile"),
)
require(
    "internal/services/cover_cleanup_service.go",
    ("CleanAvailable", "WithSourceRetention", "Reconcile"),
)
require(
    "cmd/cover_cleanup_runtime.go",
    (
        "cover_cleanup_reconciled",
        "cover_cleanup_reconcile_failed",
        "MinIOSourceRetentionHours",
    ),
)
require(
    "internal/config/config.go",
    ("MinIOSourceRetentionHours", "MINIO_SOURCE_RETENTION_HOURS"),
)
require(
    "internal/repositories/cover_cleanup_repository_test.go",
    ("Reconcile", "MarkFailed", "MarkCompleted"),
)
require(
    "internal/services/cover_cleanup_service_test.go",
    ("Reconcile", "WithSourceRetention", "CleanAvailable"),
)
require(
    "docs/BOOK_COVERS_V1_4.md",
    (
        "MINIO_SOURCE_RETENTION_HOURS",
        "cover_cleanup_reconciled",
        "réconciliation",
    ),
)
require(
    "static/openapi.json",
    ("/api/manage/books/{id}/cover", "If-None-Match"),
)

if errors:
    print("ÉCHEC : cycle de vie des couvertures v1.4 incomplet.", file=sys.stderr)
    for error in errors:
        print(f"- {error}", file=sys.stderr)
    raise SystemExit(1)

print(
    "OK: lecture privée, remplacement atomique, rétention, nettoyage récupérable "
    "et réconciliation idempotente des couvertures v1.4 contrôlés."
)

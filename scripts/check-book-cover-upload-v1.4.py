#!/usr/bin/env python3
"""Vérifie le contrat statique de l'upload sécurisé des couvertures v1.4."""

from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
errors: list[str] = []


def require(path: str, markers: tuple[str, ...]) -> None:
    content = (ROOT / path).read_text(encoding="utf-8")
    for marker in markers:
        if marker not in content:
            errors.append(f"{path}: garantie absente: {marker}")


require(
    "internal/covers/validation.go",
    (
        "image/jpeg",
        "image/png",
        "ErrImageTooLarge",
        "ErrUnsupportedImage",
        "ErrContentTypeMismatch",
        "ErrTooManyPixels",
    ),
)
require(
    "internal/covers/storage.go",
    (
        "sources/",
        "Put(",
        "Delete(",
    ),
)
require(
    "internal/migrations/sql/024_create_book_covers.sql",
    (
        "CREATE TABLE book_covers",
        "CREATE TABLE cover_processing_outbox",
        "PENDING",
        "FAILED",
    ),
)
require(
    "internal/services/book_cover_service.go",
    (
        "CreatePending",
        "Discard",
        "UPLOAD_BOOK_COVER",
    ),
)
require(
    "internal/handlers/book_covers.go",
    (
        "multipart",
        "StatusAccepted",
        "StatusRequestEntityTooLarge",
        "StatusUnprocessableEntity",
        "CoversDisabled",
    ),
)
require(
    "cmd/main.go",
    (
        'POST /api/manage/books/{id}/cover',
        "NewMinIOStore",
        "NewBookCoverService",
    ),
)
require(
    "static/openapi.json",
    (
        '"/api/manage/books/{id}/cover"',
        '"multipart/form-data"',
        '"PendingBookCover"',
        '"413"',
        '"422"',
        '"503"',
    ),
)
require(
    "internal/services/book_cover_service_test.go",
    (
        "TestBookCover",
        "cover_processing_outbox",
        "UPLOAD_BOOK_COVER",
        "deleteCalls",
    ),
)
require(
    ".env.example",
    (
        "COVERS_ENABLED=false",
        "MINIO_BUCKET_COVERS=",
        "COVER_MAX_BYTES=",
        "COVER_MAX_PIXELS=",
    ),
)

if errors:
    print("ÉCHEC : contrat de l’upload des couvertures v1.4 incomplet.", file=sys.stderr)
    for error in errors:
        print(f"- {error}", file=sys.stderr)
    raise SystemExit(1)

print(
    "OK: upload v1.4, validation JPEG/PNG, limites, stockage MinIO, "
    "outbox/audit, isolation et compensation contrôlés."
)

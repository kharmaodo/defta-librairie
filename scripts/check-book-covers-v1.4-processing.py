#!/usr/bin/env python3
"""Vérifie le contrat statique du traitement des couvertures v1.4."""

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
    "internal/covers/processing_storage.go",
    ("SourceReader", "VariantStore", "ProcessingObjectKeys"),
)
require(
    "internal/covers/processing_keys.go",
    ("master.jpg", "large.webp", "thumb.webp", "image/webp"),
)
require(
    "internal/covers/minio_processing_store.go",
    ("OpenSource", "PutVariant", "DeleteVariant", "book-cover-master"),
)
require(
    "internal/covers/image_processor.go",
    (
        "MasterCoverWidth",
        "LargeCoverWidth",
        "ThumbCoverWidth",
        "centeredTwoByThreeCrop",
        "CatmullRom",
        "nativewebp.Encode",
    ),
)
require(
    "internal/services/cover_variant_processor.go",
    ("StoredCoverVariantProcessor", "DeleteVariant", "errors.Join"),
)
require(
    "cmd/cover_worker_runtime.go",
    ("runBookCoverWorker", "NewJetStreamConsumer", "NewImageProcessor"),
)
require(
    "cmd/main.go",
    ("coverWorkerDone", "runBookCoverWorker", "cover_worker_shutdown_timeout"),
)
require(
    "internal/services/cover_pipeline_integration_test.go",
    ("COVER_PIPELINE_INTEGRATION", '"READY"', "StatObject", "generatedKeys"),
)
require(
    "go.mod",
    ("github.com/HugoSmits86/nativewebp", "golang.org/x/image"),
)

if errors:
    print("ÉCHEC : traitement des couvertures v1.4 incomplet.", file=sys.stderr)
    for error in errors:
        print(f"- {error}", file=sys.stderr)
    raise SystemExit(1)

print(
    "OK: lecture privée, recadrage 2:3, variantes JPEG/WebP, compensation MinIO, "
    "worker runtime et intégration complète contrôlés."
)

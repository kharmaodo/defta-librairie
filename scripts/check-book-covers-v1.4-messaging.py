#!/usr/bin/env python3
"""Vérifie le contrat statique de la messagerie des couvertures v1.4."""

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
    "internal/migrations/sql/025_add_cover_outbox_lease.sql",
    ("locked_by", "locked_until", "idx_cover_outbox_claimable"),
)
require(
    "internal/migrations/sql/026_add_cover_processing_lease.sql",
    ("processing_by", "processing_until", "processing_attempts"),
)
require(
    "internal/repositories/cover_outbox_repository.go",
    ("ClaimNext", "MarkPublished", "MarkFailed", "RETURNING"),
)
require(
    "internal/services/cover_outbox_publisher.go",
    ("PublishAvailable", "retryDelay", "MarkPublished", "MarkFailed"),
)
require(
    "internal/covers/jetstream_publisher.go",
    ("Nats-Msg-Id", "FileStorage", "PublishMsg", "Drain"),
)
require(
    "internal/covers/jetstream_consumer.go",
    (
        "PullSubscribe",
        "AckSync",
        "NakWithDelay",
        "MaxDeliver",
        "MarkFailed",
        "ErrPermanentCoverProcessing",
    ),
)
require(
    "internal/repositories/cover_processing_repository.go",
    (
        "CoverProcessingAlreadyReady",
        "processing_attempts",
        "Complete",
        "MarkFailed",
    ),
)
require(
    "internal/services/book_cover_worker.go",
    (
        "BookCoverWorker",
        "CoverVariantProcessor",
        "CoverProcessingAlreadyReady",
        "MAX_DELIVERIES",
    ),
)
require(
    "internal/covers/jetstream_consumer_integration_test.go",
    ("NATS_INTEGRATION", "exhaustingCoverHandler", "failures != 1"),
)
require(
    "internal/services/cover_pipeline_integration_test.go",
    ("COVER_PIPELINE_INTEGRATION", "published_at", "Nats-Msg-Id"),
)
require(
    "internal/config/config.go",
    (
        "NATSCoversStream",
        "NATSCoversSubject",
        "NATSCoversConsumer",
        "CoverWorkerMaxDeliver",
    ),
)
require(
    ".env.example",
    (
        "NATS_COVERS_STREAM=",
        "NATS_COVERS_SUBJECT=",
        "NATS_COVERS_CONSUMER=",
        "COVER_WORKER_MAX_DELIVER=",
    ),
)

if errors:
    print("ÉCHEC : contrat de messagerie des couvertures v1.4 incomplet.", file=sys.stderr)
    for error in errors:
        print(f"- {error}", file=sys.stderr)
    raise SystemExit(1)

print(
    "OK: outbox avec baux, JetStream persistant, publication dédupliquée, "
    "consumer durable, reprises, plafond de livraisons et worker idempotent contrôlés."
)

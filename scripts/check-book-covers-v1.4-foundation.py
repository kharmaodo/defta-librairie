#!/usr/bin/env python3
"""Contrôle statique de la fondation asynchrone des couvertures v1.4."""

from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
errors: list[str] = []


def read(relative: str) -> str:
    path = ROOT / relative
    if not path.is_file():
        errors.append(f"fichier absent : {relative}")
        return ""
    return path.read_text(encoding="utf-8")


architecture = read("docs/BOOK_COVERS_V1_4.md")
roadmap = read("docs/ROADMAP_V1_3_V1_4.md")
backlog = read("BACKLOG.md")
compose = read("deploy/docker-compose.covers.yml")
nats = read("deploy/nats/nats-server.conf")
minio_init = read("deploy/minio/init-bucket.sh")
env_example = read(".env.example")

required_architecture = (
    "transactional outbox",
    "PENDING",
    "PROCESSING",
    "READY",
    "FAILED",
    "book.covers.process.v1",
    "sources/{library_id}/{book_id}/{cover_id}",
    "masters/{library_id}/{book_id}/{cover_id}",
    "linux/amd64",
    "linux/arm64",
    "linux/arm/v7",
)
for token in required_architecture:
    if token not in architecture:
        errors.append(f"contrat incomplet, valeur absente : {token}")

for service in ("minio:", "minio-init:", "nats:"):
    if service not in compose:
        errors.append(f"service Docker absent : {service[:-1]}")

for volume in ("minio-covers-data", "nats-jetstream-data"):
    if volume not in compose:
        errors.append(f"volume Docker absent : {volume}")

if ":latest" in compose or "image: latest" in compose:
    errors.append("tag Docker latest interdit")

for image in (
    "quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z",
    "quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z",
):
    if image not in compose:
        errors.append(f"image MinIO officielle épinglée absente : {image}")

if "healthcheck:" not in compose:
    errors.append("health checks Docker absents")

for token in ("jetstream", "store_dir", "max_file_store"):
    if token not in nats:
        errors.append(f"configuration JetStream incomplète : {token}")

for token in ("mc mb --ignore-existing", "mc anonymous set none"):
    if token not in minio_init:
        errors.append(f"initialisation MinIO incomplète : {token}")

required_env = (
    "MINIO_ENDPOINT=",
    "MINIO_ACCESS_KEY=",
    "MINIO_SECRET_KEY=",
    "MINIO_BUCKET_COVERS=book-covers",
    "NATS_URL=",
    "NATS_USER=",
    "NATS_PASSWORD=",
    "NATS_COVERS_STREAM=BOOK_COVERS",
    "NATS_COVERS_SUBJECT=book.covers.process.v1",
    "COVER_MAX_BYTES=5242880",
    "COVER_MAX_PIXELS=24000000",
)
for token in required_env:
    if token not in env_example:
        errors.append(f"variable de configuration absente : {token}")

if "| 1 | Fondation asynchrone" not in backlog or "| À valider |" not in backlog:
    errors.append("incrément de fondation v1.4 absent du backlog")

if "worker Go asynchrone" not in roadmap:
    errors.append("roadmap non alignée sur le worker asynchrone")

if errors:
    print("ÉCHEC : fondation des couvertures v1.4 incomplète.", file=sys.stderr)
    for error in errors:
        print(f"- {error}", file=sys.stderr)
    raise SystemExit(1)

print(
    "OK : contrat v1.4, outbox SQLite, MinIO privé, NATS JetStream, "
    "worker asynchrone et infrastructure Docker contrôlés."
)

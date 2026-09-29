#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

echo '[v1.7.0] Gate de delivery existant (Go, race, frontend, Playwright)'
sh scripts/check-delivery.sh
echo '[v1.7.0] Analyse statique Go'
go vet -tags fts5 ./...
echo '[v1.7.0] Build React/Vite reproductible'
npm ci --prefix frontend/cover-imports
npm run build --prefix frontend/cover-imports
git diff --exit-code -- static/cover-imports
echo '[v1.7.0] Configuration Docker et worker CPU'
command -v docker >/dev/null 2>&1 || { echo 'Docker requis pour le gate v1.7.0' >&2; exit 1; }
MINIO_ACCESS_KEY=release-gate MINIO_SECRET_KEY=release-gate-secret \
NATS_USER=release-gate NATS_PASSWORD=release-gate-secret \
    docker compose -f deploy/docker-compose.covers.yml --profile worker config --quiet
echo 'OK : gate v1.7.0 terminé.'

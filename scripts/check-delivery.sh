#!/usr/bin/env sh
set -eu

root_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root_dir"
export PYTHONDONTWRITEBYTECODE=1

for command_name in go node npm python3; do
  command -v "$command_name" >/dev/null 2>&1 || {
    echo "Commande requise absente : $command_name" >&2
    exit 1
  }
done

echo "[1/10] Contrôle du patch"
git diff --check

echo "[2/10] Contrat OpenAPI"
python3 scripts/check-openapi.py

echo "[3/10] Accessibilité structurelle"
python3 scripts/check-admin-accessibility.py

echo "[4/10] Contrat du dashboard admin"
python3 scripts/check-admin-dashboard.py

echo "[5/10] Fondation du dashboard v1.2"
python3 scripts/check-admin-v1.2-foundation.py

echo "[6/10] Restauration SQLite"
python3 scripts/test-restore-db.py

echo "[7/10] Tests Go"
go test -tags fts5 ./...

echo "[8/10] Détection de courses Go"
go test -race -tags fts5 ./...

echo "[9/10] Tests frontend"
npm ci
npm run test:frontend

echo "[10/10] Parcours Chromium"
npm run test:browser

echo "OK : contrôle final du delivery terminé."

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

echo "[1/9] Contrôle du patch"
git diff --check

echo "[2/9] Contrat OpenAPI"
python3 scripts/check-openapi.py

echo "[3/9] Accessibilité structurelle"
python3 scripts/check-admin-accessibility.py

echo "[4/9] Contrat du dashboard admin"
python3 scripts/check-admin-dashboard.py

echo "[5/9] Restauration SQLite"
python3 scripts/test-restore-db.py

echo "[6/9] Tests Go"
go test -tags fts5 ./...

echo "[7/9] Détection de courses Go"
go test -race -tags fts5 ./...

echo "[8/9] Tests frontend"
npm ci
npm run test:frontend

echo "[9/9] Parcours Chromium"
npm run test:browser

echo "OK : contrôle final du delivery terminé."

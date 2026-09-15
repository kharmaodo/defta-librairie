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

echo "[1/14] Contrôle du patch"
git diff --check

echo "[2/14] Contrat OpenAPI"
python3 scripts/check-openapi.py

echo "[3/14] Accessibilité structurelle"
python3 scripts/check-admin-accessibility.py

echo "[4/14] Contrat du dashboard admin"
python3 scripts/check-admin-dashboard.py

echo "[5/14] Fondation du dashboard v1.2"
python3 scripts/check-admin-v1.2-foundation.py

echo "[6/14] Couverture navigateur v1.2"
python3 scripts/check-browser-coverage.py

echo "[7/14] Contrat visuel des champs"
python3 scripts/check-admin-form-controls.py

echo "[8/14] Contrat des suppressions"
python3 scripts/check-admin-delete-confirmation.py

echo "[9/14] Fondation des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-foundation.py

echo "[10/14] Restauration SQLite"
python3 scripts/test-restore-db.py

echo "[11/14] Tests Go"
go test -tags fts5 ./...

echo "[12/14] Détection de courses Go"
go test -race -tags fts5 ./...

echo "[13/14] Tests frontend"
npm ci
npm run test:frontend

echo "[14/14] Parcours Chromium"
npm run test:browser

echo "OK : contrôle final du delivery terminé."

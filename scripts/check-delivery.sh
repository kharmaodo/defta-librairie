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

echo "[1/17] Contrôle du patch"
git diff --check

echo "[2/17] Contrat OpenAPI"
python3 scripts/check-openapi.py

echo "[3/17] Accessibilité structurelle"
python3 scripts/check-admin-accessibility.py

echo "[4/17] Contrat du dashboard admin"
python3 scripts/check-admin-dashboard.py

echo "[5/17] Fondation du dashboard v1.2"
python3 scripts/check-admin-v1.2-foundation.py

echo "[6/17] Couverture navigateur v1.2"
python3 scripts/check-browser-coverage.py

echo "[7/17] Contrat visuel des champs"
python3 scripts/check-admin-form-controls.py

echo "[8/17] Contrat des suppressions"
python3 scripts/check-admin-delete-confirmation.py

echo "[9/17] Fondation des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-foundation.py

echo "[10/17] Contrat de l’upload des couvertures v1.4"
python3 scripts/check-book-cover-upload-v1.4.py

echo "[11/17] Messagerie des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-messaging.py

echo "[12/17] Traitement des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-processing.py

echo "[13/17] Restauration SQLite"
python3 scripts/test-restore-db.py

echo "[14/17] Tests Go"
go test -tags fts5 ./...

echo "[15/17] Détection de courses Go"
go test -race -tags fts5 ./...

echo "[16/17] Tests frontend"
npm ci
npm run test:frontend

echo "[17/17] Parcours Chromium"
npm run test:browser

echo "OK : contrôle final du delivery terminé."

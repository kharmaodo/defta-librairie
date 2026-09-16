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

echo "[1/18] Contrôle du patch"
git diff --check

echo "[2/18] Contrat OpenAPI"
python3 scripts/check-openapi.py

echo "[3/18] Accessibilité structurelle"
python3 scripts/check-admin-accessibility.py

echo "[4/18] Contrat du dashboard admin"
python3 scripts/check-admin-dashboard.py

echo "[5/18] Fondation du dashboard v1.2"
python3 scripts/check-admin-v1.2-foundation.py

echo "[6/18] Couverture navigateur v1.2"
python3 scripts/check-browser-coverage.py

echo "[7/18] Contrat visuel des champs"
python3 scripts/check-admin-form-controls.py

echo "[8/18] Contrat des suppressions"
python3 scripts/check-admin-delete-confirmation.py

echo "[9/18] Fondation des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-foundation.py

echo "[10/18] Contrat de l’upload des couvertures v1.4"
python3 scripts/check-book-cover-upload-v1.4.py

echo "[11/18] Messagerie des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-messaging.py

echo "[12/18] Traitement des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-processing.py

echo "[13/18] Cycle de vie des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-lifecycle.py

echo "[14/18] Restauration SQLite"
python3 scripts/test-restore-db.py

echo "[15/18] Tests Go"
go test -tags fts5 ./...

echo "[16/18] Détection de courses Go"
go test -race -tags fts5 ./...

echo "[17/18] Tests frontend"
npm ci
npm run test:frontend

echo "[18/18] Parcours Chromium"
npm run test:browser

echo "OK : contrôle final du delivery terminé."

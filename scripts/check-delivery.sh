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

echo "[1/16] Contrôle du patch"
git diff --check

echo "[2/16] Contrat OpenAPI"
python3 scripts/check-openapi.py

echo "[3/16] Accessibilité structurelle"
python3 scripts/check-admin-accessibility.py

echo "[4/16] Contrat du dashboard admin"
python3 scripts/check-admin-dashboard.py

echo "[5/16] Fondation du dashboard v1.2"
python3 scripts/check-admin-v1.2-foundation.py

echo "[6/16] Couverture navigateur v1.2"
python3 scripts/check-browser-coverage.py

echo "[7/16] Contrat visuel des champs"
python3 scripts/check-admin-form-controls.py

echo "[8/16] Contrat des suppressions"
python3 scripts/check-admin-delete-confirmation.py

echo "[9/16] Fondation des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-foundation.py

echo "[10/16] Contrat de l’upload des couvertures v1.4"
python3 scripts/check-book-cover-upload-v1.4.py

echo "[11/16] Messagerie des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-messaging.py

echo "[12/16] Restauration SQLite"
python3 scripts/test-restore-db.py

echo "[13/16] Tests Go"
go test -tags fts5 ./...

echo "[14/16] Détection de courses Go"
go test -race -tags fts5 ./...

echo "[15/16] Tests frontend"
npm ci
npm run test:frontend

echo "[16/16] Parcours Chromium"
npm run test:browser

echo "OK : contrôle final du delivery terminé."

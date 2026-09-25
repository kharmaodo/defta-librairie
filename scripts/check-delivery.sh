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

echo "[1/19] Contrôle du patch"
git diff --check

echo "[2/19] Contrat OpenAPI"
python3 scripts/check-openapi.py

echo "[3/19] Accessibilité structurelle"
python3 scripts/check-admin-accessibility.py

echo "[4/19] Contrat du dashboard admin"
python3 scripts/check-admin-dashboard.py

echo "[5/19] Fondation du dashboard v1.2"
python3 scripts/check-admin-v1.2-foundation.py

echo "[6/19] Couverture navigateur v1.2"
python3 scripts/check-browser-coverage.py

echo "[7/19] Contrat visuel des champs"
python3 scripts/check-admin-form-controls.py

echo "[8/19] Contrat des suppressions"
python3 scripts/check-admin-delete-confirmation.py

echo "[9/19] Fondation des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-foundation.py

echo "[10/19] Contrat de l’upload des couvertures v1.4"
python3 scripts/check-book-cover-upload-v1.4.py

echo "[11/19] Messagerie des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-messaging.py

echo "[12/19] Traitement des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-processing.py

echo "[13/19] Cycle de vie des couvertures v1.4"
python3 scripts/check-book-covers-v1.4-lifecycle.py

echo "[14/19] Restauration SQLite"
python3 scripts/test-restore-db.py

echo "[15/19] Gate OWASP v1.6"
python3 scripts/check-owasp-v1.6.py

echo "[16/19] Tests Go"
go test -tags fts5 ./...

echo "[17/19] Détection de courses Go"
go test -race -tags fts5 ./...

echo "[18/19] Tests frontend"
npm ci
npm run test:frontend

echo "[19/19] Parcours Chromium"
npm run test:browser

echo "OK : contrôle final du delivery terminé."

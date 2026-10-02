#!/usr/bin/env sh
set -eu
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"
for analyzer in govulncheck gosec staticcheck; do
  command -v "$analyzer" >/dev/null 2>&1 || { echo "Outil requis absent : $analyzer" >&2; exit 1; }
done
printf '%s\n' '[v1.7.2] Analyseurs de sécurité (versions épinglées dans le workflow)'
govulncheck -tags fts5 ./...
gosec -tags fts5 ./...
staticcheck -tags fts5 ./...
python3 scripts/test-release-packages.py
python3 -m unittest discover -s scripts -p test_run_recette.py
# Réutilise tous les parcours métier/OCR et le build frontend existants.
sh scripts/check-release-v1.7.1.sh
printf '%s\n' 'OK : gate technique v1.7.2 ; recette locale et TLS réel à consigner séparément.'

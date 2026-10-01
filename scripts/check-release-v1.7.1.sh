#!/usr/bin/env sh
set -eu
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"
sh scripts/check-release-v1.7.0.sh
printf '%s\n' '[v1.7.1] OCR expérimental Python et conteneur isolé'
(cd services/ocr-experimental && python3 -m ruff check . && python3 -m mypy --strict . && python3 -m unittest discover -s tests -v)
acceptance_tmp=$(mktemp -d)
trap 'rm -rf "$acceptance_tmp"' EXIT HUP INT TERM
CGO_ENABLED=0 go test -c -o "$acceptance_tmp/ocr-adapter.test" ./internal/ocr
OCR_ADAPTER_TEST_BINARY="$acceptance_tmp/ocr-adapter.test" sh services/ocr-experimental/check-container.sh
rm -f "$acceptance_tmp/ocr-adapter.test"
printf '%s\n' '[v1.7.1] Comparateur réel, corpus synthétique et gate bloqué'
sh scripts/check-ocr-acceptance.sh
printf '%s\n' 'OK : contrôles techniques v1.7.1. Activation réelle non validée par ce script.'

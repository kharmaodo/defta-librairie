#!/usr/bin/env bash
# Build candidate or tagged archives without publishing them.
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"
set -euo pipefail
version="${1:?Version requise}"
python3 -c 'import re,sys; sys.exit(0 if re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", sys.argv[1]) else 1)' "$version"
build_date="${2:-$(date -u +%Y-%m-%d)}"
staging="$(mktemp -d)"
trap 'rm -rf "$staging"' EXIT
mkdir -p dist
output_dir="$(cd dist && pwd)"
printf 'VERSION=%s\nBUILD_DATE=%s\nSOURCE_COMMIT=%s\nGO_VERSION=%s\n' \
  "$version" "$build_date" "$(git rev-parse HEAD)" "$(go env GOVERSION)" \
  > "$staging/BUILD-INFO.txt"

package_windows() {
  local name="defta-librairie-${version}-windows-amd64"
  mkdir -p "$staging/$name"
  CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
    CC=x86_64-w64-mingw32-gcc \
    go build -trimpath -tags fts5 -ldflags="-s -w" \
      -o "$staging/$name/defta-librairie.exe" ./cmd
  cp -R templates static .env.example README.md "$staging/BUILD-INFO.txt" "$staging/$name/"
  (cd "$staging" && zip -qr "$output_dir/$name.zip" "$name")
}

package_linux_amd64() {
  local name="defta-librairie-${version}-linux-amd64"
  mkdir -p "$staging/$name"
  CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -trimpath -tags fts5 -ldflags="-s -w" \
      -o "$staging/$name/defta-librairie" ./cmd
  cp -R templates static .env.example README.md "$staging/BUILD-INFO.txt" "$staging/$name/"
  tar -C "$staging" -czf "dist/$name.tar.gz" "$name"
}

package_windows
package_linux_amd64

cp "$staging/BUILD-INFO.txt" dist/BUILD-INFO.txt
(cd dist && sha256sum *.zip *.tar.gz > SHA256SUMS)
file "$staging"/*/defta-librairie*

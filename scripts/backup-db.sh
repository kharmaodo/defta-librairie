#!/usr/bin/env bash
set -euo pipefail

db_path="${DB_PATH:-./data/defta.db}"
backup_root="${DEFTA_BACKUP_DIR:-./data/backups}"

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 est requis pour créer une sauvegarde cohérente" >&2
  exit 1
fi
if ! test -f "$db_path"; then
  echo "Base SQLite introuvable : $db_path" >&2
  exit 1
fi

mkdir -p "$backup_root"
timestamp=$(date -u '+%Y%m%dT%H%M%SZ')
backup_path="$backup_root/defta-$timestamp.db"

sqlite3 "$db_path" ".timeout 10000" ".backup '$backup_path'"
integrity=$(sqlite3 "$backup_path" 'PRAGMA integrity_check;')
if test "$integrity" != "ok"; then
  echo "Échec du contrôle d’intégrité : $integrity" >&2
  exit 1
fi

sha256sum "$backup_path"
printf 'Sauvegarde SQLite créée : %s\n' "$backup_path"

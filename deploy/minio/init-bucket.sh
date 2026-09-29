#!/bin/sh
set -eu

: "${MINIO_ENDPOINT:?MINIO_ENDPOINT is required}"
: "${MINIO_ACCESS_KEY:?MINIO_ACCESS_KEY is required}"
: "${MINIO_SECRET_KEY:?MINIO_SECRET_KEY is required}"
: "${MINIO_BUCKET_COVERS:?MINIO_BUCKET_COVERS is required}"

attempt=1
while [ "$attempt" -le 30 ]; do
  if mc alias set defta "$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" >/dev/null 2>&1 &&
    mc mb --ignore-existing "defta/$MINIO_BUCKET_COVERS" >/dev/null 2>&1 &&
    mc anonymous set none "defta/$MINIO_BUCKET_COVERS" >/dev/null 2>&1; then
    echo "Bucket privé prêt : $MINIO_BUCKET_COVERS"
    exit 0
  fi
  echo "MinIO indisponible, tentative $attempt/30" >&2
  attempt=$((attempt + 1))
  sleep 2
done

echo "Initialisation du bucket impossible après 30 tentatives" >&2
exit 1

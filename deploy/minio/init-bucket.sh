#!/bin/sh
set -eu

: "${MINIO_ENDPOINT:?MINIO_ENDPOINT is required}"
: "${MINIO_ACCESS_KEY:?MINIO_ACCESS_KEY is required}"
: "${MINIO_SECRET_KEY:?MINIO_SECRET_KEY is required}"
: "${MINIO_BUCKET_COVERS:?MINIO_BUCKET_COVERS is required}"

mc alias set defta "$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY"
mc mb --ignore-existing "defta/$MINIO_BUCKET_COVERS"
mc anonymous set none "defta/$MINIO_BUCKET_COVERS"

echo "Bucket privé prêt : $MINIO_BUCKET_COVERS"

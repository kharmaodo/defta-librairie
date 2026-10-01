#!/usr/bin/env sh
set -eu
service_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
container_name="defta-ocr-check-$$"
cleanup() { docker rm -f "$container_name" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
docker compose -f "$service_dir/compose.ocr-experimental.yaml" -f "$service_dir/compose.ocr-diagnostic.yaml" config --quiet
docker build -t defta-ocr-experimental:check "$service_dir"
set --
if [ -n "${OCR_ADAPTER_TEST_BINARY:-}" ]; then
    set -- --mount "type=bind,source=$OCR_ADAPTER_TEST_BINARY,target=/opt/ocr-adapter.test,readonly"
fi
docker run "$@" -d --name "$container_name" --network none --read-only \
    --tmpfs /tmp:rw,noexec,nosuid,size=256m --cap-drop ALL \
    --security-opt no-new-privileges:true --memory 1g --cpus 2 --pids-limit 64 \
    defta-ocr-experimental:check
if ! docker exec -i "$container_name" python - < "$service_dir/tests/container_smoke.py"; then
    docker logs "$container_name"
    exit 1
fi

if [ -n "${OCR_ADAPTER_TEST_BINARY:-}" ]; then
    docker exec -e OCR_INTEGRATION_ENDPOINT=http://127.0.0.1:8091 "$container_name" \
        /opt/ocr-adapter.test -test.run '^TestHTTPContainerIntegration$' -test.v
fi

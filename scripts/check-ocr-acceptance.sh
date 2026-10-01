#!/usr/bin/env sh
# Synthetic integration proves the tool works; it never authorizes activation.
set -eu
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"
umask 077
acceptance_log=$(mktemp)
acceptance_pid=
cleanup() {
    if [ -n "$acceptance_pid" ]; then
        kill "$acceptance_pid" 2>/dev/null || true
        wait "$acceptance_pid" 2>/dev/null || true
    fi
    rm -f "$acceptance_log"
}
trap cleanup EXIT HUP INT TERM
(
    cd services/ocr-experimental
    exec "${OCR_ACCEPTANCE_PYTHON:-python3}" -m uvicorn api:app --host 127.0.0.1 --port 8092
) >"$acceptance_log" 2>&1 &
acceptance_pid=$!
"${OCR_ACCEPTANCE_PYTHON:-python3}" - <<'PY'
import time
import urllib.request
for _ in range(40):
    try:
        with urllib.request.urlopen('http://127.0.0.1:8092/health/ready', timeout=1) as response:
            if response.status == 200:
                break
    except OSError:
        time.sleep(0.25)
else:
    raise SystemExit('OCR_ACCEPTANCE_RUNTIME_UNAVAILABLE')
PY
kill -0 "$acceptance_pid" 2>/dev/null || { echo OCR_ACCEPTANCE_RUNTIME_UNAVAILABLE >&2; exit 1; }
OCR_ACCEPTANCE_INTEGRATION_ENDPOINT=http://127.0.0.1:8092 \
    go test -tags fts5 ./cmd/ocr-acceptance -run TestCommandRealRunnersSyntheticGate -count=1 -v

kill -0 "$acceptance_pid" 2>/dev/null || { echo OCR_ACCEPTANCE_RUNTIME_UNAVAILABLE >&2; exit 1; }

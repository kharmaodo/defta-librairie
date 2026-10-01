"""Run inside the isolated container, using only synthetic in-memory images."""

from __future__ import annotations

import io
import json
import os
import time
import urllib.error
import urllib.request
from pathlib import Path

from PIL import Image


def main() -> None:
    assert os.getuid() == 10001
    base = "http://127.0.0.1:8091"
    for attempt in range(30):
        try:
            with urllib.request.urlopen(base + "/health/ready", timeout=3) as reply:
                assert json.load(reply)["status"] == "ready"
            break
        except (urllib.error.URLError, TimeoutError):
            if attempt == 29:
                raise
            time.sleep(1)
    with urllib.request.urlopen(base + "/health/live", timeout=3) as reply:
        assert json.load(reply)["status"] == "live"
    data = io.BytesIO()
    Image.new("RGB", (80, 100), "white").save(data, "PNG")
    boundary = "defta-1713"
    body = (
        (
            f'--{boundary}\r\nContent-Disposition: form-data; name="image"; filename="synthetic.png"\r\n'
            "Content-Type: image/png\r\n\r\n"
        ).encode()
        + data.getvalue()
        + f"\r\n--{boundary}--\r\n".encode()
    )
    request = urllib.request.Request(
        base + "/v1/ocr",
        data=body,
        headers={
            "Content-Type": f"multipart/form-data; boundary={boundary}",
            "X-Request-Id": "container-1713",
        },
    )
    with urllib.request.urlopen(request, timeout=35) as reply:
        output = json.load(reply)
        assert output["schemaVersion"] == 1
        assert output["engineVersion"].startswith("tesseract 5.")
        assert output["confidence"] is None
        assert reply.headers["X-Request-Id"] == "container-1713"
    assert not list(Path("/tmp").glob("defta-ocr-*"))
    assert not list(Path("/app").glob("*.db"))
    print(
        "OK: non-root offline container, probes, real multipart OCR and temporary cleanup"
    )


if __name__ == "__main__":
    main()

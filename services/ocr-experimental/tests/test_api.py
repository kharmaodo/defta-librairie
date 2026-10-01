from __future__ import annotations

import asyncio
import io
import threading
import unittest
from collections.abc import AsyncIterator
from unittest.mock import patch
from typing import cast

import httpx
from fastapi.testclient import TestClient
from PIL import Image

from api import ServiceSettings, create_app
from ocr_core.pipeline import Extraction, OCRError, PassResult, Settings


def png() -> bytes:
    output = io.BytesIO()
    Image.new("RGB", (20, 30), "white").save(output, "PNG")
    return output.getvalue()


def result(data: bytes, settings: Settings) -> Extraction:
    selected = PassResult("كِتاب ـ العلم", 0.82, 11, "original")
    return Extraction(
        "tesseract-experimental",
        "tesseract 5.test",
        "ara",
        "ocr-local-v1",
        selected,
        selected,
        (selected,),
        1,
    )


def client(settings: ServiceSettings | None = None) -> TestClient:
    return TestClient(create_app(settings, result, lambda timeout: "tesseract 5.test"))


def send(http: TestClient) -> httpx.Response:
    return cast(
        httpx.Response,
        http.post("/v1/ocr", files={"image": ("a.png", png(), "image/png")}),
    )


class APITests(unittest.TestCase):
    def test_contract_and_correlation(self) -> None:
        with client() as http:
            reply = http.post(
                "/v1/ocr",
                files={"image": ("a.png", png(), "image/png")},
                headers={"X-Request-Id": "test-1713"},
            )
            self.assertEqual(reply.status_code, 200)
            body = reply.json()
            self.assertEqual(body["schemaVersion"], 1)
            self.assertEqual(body["textRaw"], "كِتاب ـ العلم")
            self.assertEqual(body["textNormalized"], "كتاب العلم")
            self.assertEqual(body["confidence"], 0.82)
            self.assertEqual(body["psm"], 11)
            self.assertEqual(reply.headers["x-request-id"], "test-1713")
            self.assertEqual(reply.headers["cache-control"], "no-store")
            self.assertNotIn("candidates", body)

    def test_probes_runtime_failure_and_no_docs(self) -> None:
        with client() as http:
            self.assertEqual(http.get("/health/live").status_code, 200)
            self.assertEqual(http.get("/health/ready").status_code, 200)
            self.assertEqual(http.get("/docs").status_code, 404)

        def unavailable(timeout: float) -> str:
            raise OCRError("OCR_UNAVAILABLE", "secret binary path")

        with TestClient(create_app(readiness=unavailable)) as http:
            self.assertEqual(http.get("/health/live").status_code, 200)
            reply = http.get("/health/ready")
            self.assertEqual(reply.status_code, 503)
            self.assertNotIn("secret", reply.text)

    def test_invalid_inputs_and_slot_released(self) -> None:
        with client() as http:
            for name, mime, data in [
                ("image", "image/png", b"bad"),
                ("image", "image/jpeg", png()),
                ("image", "application/octet-stream", png()),
                ("wrong", "image/png", png()),
            ]:
                reply = http.post("/v1/ocr", files={name: ("a.png", data, mime)})
                self.assertEqual(reply.status_code, 422, reply.text)
                self.assertEqual(reply.json()["error"]["code"], "INVALID_IMAGE")
            self.assertEqual(
                http.post(
                    "/v1/ocr", json={"url": "https://example.test/image"}
                ).status_code,
                422,
            )
            self.assertEqual(
                http.post(
                    "/v1/ocr",
                    content=b"broken",
                    headers={"content-type": "multipart/form-data"},
                ).status_code,
                422,
            )
            self.assertEqual(send(http).status_code, 200)

    def test_extra_files_and_fields_rejected(self) -> None:
        with client() as http:
            reply = http.post(
                "/v1/ocr",
                files=[
                    ("image", ("a.png", png(), "image/png")),
                    ("image", ("b.png", png(), "image/png")),
                ],
            )
            self.assertEqual(reply.status_code, 422)
            reply = http.post(
                "/v1/ocr",
                files={"image": ("a.png", png(), "image/png")},
                data={"path": "/secret"},
            )
            self.assertEqual(reply.status_code, 422)

    def test_file_and_total_body_limits(self) -> None:
        with client(ServiceSettings(ocr=Settings(max_bytes=50))) as http:
            self.assertEqual(send(http).status_code, 413)
            for headers in [{"content-length": "70000"}, {}]:
                reply = http.post(
                    "/v1/ocr",
                    content=iter([b"x" * 35000, b"x" * 35000]),
                    headers={
                        "content-type": "multipart/form-data; boundary=x",
                        **headers,
                    },
                )
                self.assertEqual(reply.status_code, 413)

    def test_dimensions_before_inference(self) -> None:
        for config in [
            ServiceSettings(max_dimension=10),
            ServiceSettings(ocr=Settings(max_pixels=10)),
        ]:
            with client(config) as http:
                self.assertEqual(send(http).status_code, 413)

    def test_failure_codes_redaction_and_retry(self) -> None:
        for code, status in [
            ("OCR_TIMEOUT", 504),
            ("OCR_UNAVAILABLE", 503),
            ("OCR_INVALID_OUTPUT", 503),
            ("OCR_OUTPUT_TOO_LARGE", 503),
        ]:

            def failed(data: bytes, settings: Settings) -> Extraction:
                raise OCRError(code, "private OCR text /secret")

            with TestClient(create_app(extractor=failed)) as http:
                reply = send(http)
                self.assertEqual(reply.status_code, status)
                self.assertNotIn("private", reply.text)
                self.assertTrue(reply.json()["error"]["requestId"])

    def test_real_http_tesseract(self) -> None:
        with TestClient(create_app()) as http:
            self.assertEqual(http.get("/health/ready").status_code, 200)
            reply = send(http)
            self.assertEqual(reply.status_code, 200, reply.text)
            self.assertTrue(reply.json()["engineVersion"].startswith("tesseract 5."))
            self.assertIsNone(reply.json()["confidence"])

    def test_invalid_settings_fail_before_serving(self) -> None:
        for config in [
            (0, 15, 10000),
            (5, 15, 10000),
            (1, float("nan"), 10000),
            (1, 15, 10001),
        ]:
            with self.assertRaises(ValueError):
                ServiceSettings(
                    concurrency=config[0],
                    upload_timeout_seconds=config[1],
                    max_dimension=config[2],
                )
        for limits in [Settings(max_bytes=10485761), Settings(color_diagnostics=True)]:
            with self.assertRaises(ValueError):
                ServiceSettings(ocr=limits)
        with patch.dict(
            "os.environ",
            {"OCR_SERVICE_CONCURRENCY": "2", "OCR_SERVICE_TIMEOUT_SECONDS": "12"},
        ):
            parsed = ServiceSettings.from_env()
            self.assertEqual(parsed.concurrency, 2)
            self.assertEqual(parsed.ocr.timeout_seconds, 12)

    def test_busy_does_not_queue_and_health_stays_live(self) -> None:
        started, release = threading.Event(), threading.Event()

        def blocked(data: bytes, settings: Settings) -> Extraction:
            started.set()
            if not release.wait(5):
                raise OCRError("OCR_TIMEOUT", "test hung")
            return result(data, settings)

        with TestClient(create_app(extractor=blocked)) as http:
            replies: list[httpx.Response] = []
            thread = threading.Thread(target=lambda: replies.append(send(http)))
            thread.start()
            try:
                self.assertTrue(started.wait(5))
                reply = send(http)
                self.assertEqual(reply.status_code, 503)
                self.assertEqual(reply.json()["error"]["code"], "OCR_BUSY")
                self.assertEqual(reply.headers["retry-after"], "1")
                self.assertEqual(http.get("/health/live").status_code, 200)
            finally:
                release.set()
                thread.join(5)
            self.assertEqual(replies[0].status_code, 200)
            self.assertEqual(send(http).status_code, 200)


class AsyncAPITests(unittest.IsolatedAsyncioTestCase):
    async def test_slow_upload_timeout_and_recovery(self) -> None:
        app = create_app(ServiceSettings(upload_timeout_seconds=0.02), result)

        async def slow() -> AsyncIterator[bytes]:
            yield b"partial"
            await asyncio.Event().wait()

        async with app.router.lifespan_context(app):
            async with httpx.AsyncClient(
                transport=httpx.ASGITransport(app=app), base_url="http://test"
            ) as http:
                reply = await http.post(
                    "/v1/ocr",
                    content=slow(),
                    headers={"content-type": "multipart/form-data; boundary=x"},
                )
                self.assertEqual(reply.status_code, 504)
                reply = await http.post(
                    "/v1/ocr", files={"image": ("a.png", png(), "image/png")}
                )
                self.assertEqual(reply.status_code, 200)

    async def test_disconnect_does_not_free_running_slot(self) -> None:
        started, release = threading.Event(), threading.Event()

        def blocked(data: bytes, settings: Settings) -> Extraction:
            started.set()
            release.wait(5)
            return result(data, settings)

        app = create_app(extractor=blocked)
        async with app.router.lifespan_context(app):
            async with httpx.AsyncClient(
                transport=httpx.ASGITransport(app=app), base_url="http://test"
            ) as http:
                request = asyncio.create_task(
                    http.post("/v1/ocr", files={"image": ("a.png", png(), "image/png")})
                )
                try:
                    self.assertTrue(await asyncio.to_thread(started.wait, 5))
                    request.cancel()
                    with self.assertRaises(asyncio.CancelledError):
                        await request
                    reply = await http.post(
                        "/v1/ocr", files={"image": ("a.png", png(), "image/png")}
                    )
                    self.assertEqual(reply.json()["error"]["code"], "OCR_BUSY")
                finally:
                    release.set()


if __name__ == "__main__":
    unittest.main()

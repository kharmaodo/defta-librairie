"""Private bounded OCR extraction service; no catalogue or business writes."""

from __future__ import annotations

import asyncio
import io
import os
import re
import threading
import uuid
from collections.abc import AsyncGenerator, AsyncIterator
from concurrent.futures import ThreadPoolExecutor
from contextlib import asynccontextmanager
from dataclasses import dataclass
from typing import Callable

from fastapi import FastAPI, Request
from PIL import Image, UnidentifiedImageError
from starlette.datastructures import Headers, UploadFile
from starlette.formparsers import MultiPartException, MultiPartParser
from starlette.responses import JSONResponse

from ocr_core.catalogue import normalize
from ocr_core.pipeline import Extraction, OCRError, Settings, TesseractRunner, extract


@dataclass(frozen=True)
class ServiceSettings:
    ocr: Settings = Settings()
    concurrency: int = 1
    upload_timeout_seconds: float = 15
    max_dimension: int = 10000

    def __post_init__(self) -> None:
        if not 1 <= self.concurrency <= 4:
            raise ValueError("OCR concurrency must be between 1 and 4")
        if not 0 < self.upload_timeout_seconds <= 30:
            raise ValueError("Upload timeout must be between 0 and 30 seconds")
        if not 1 <= self.max_dimension <= 10000:
            raise ValueError("Image dimension limit must be between 1 and 10000")
        if (
            self.ocr.max_bytes > 10 * 1024 * 1024
            or self.ocr.max_pixels > 24000000
            or self.ocr.max_output_bytes > 1024 * 1024
            or self.ocr.color_diagnostics
        ):
            raise ValueError("Service limits exceed the default OCR budget")

    @classmethod
    def from_env(cls) -> ServiceSettings:
        return cls(
            ocr=Settings(
                timeout_seconds=float(os.getenv("OCR_SERVICE_TIMEOUT_SECONDS", "30")),
                max_bytes=int(os.getenv("OCR_SERVICE_MAX_IMAGE_BYTES", "10485760")),
                max_pixels=int(os.getenv("OCR_SERVICE_MAX_PIXELS", "24000000")),
                max_output_bytes=int(
                    os.getenv("OCR_SERVICE_MAX_OUTPUT_BYTES", "1048576")
                ),
            ),
            concurrency=int(os.getenv("OCR_SERVICE_CONCURRENCY", "1")),
            upload_timeout_seconds=float(
                os.getenv("OCR_SERVICE_UPLOAD_TIMEOUT_SECONDS", "15")
            ),
            max_dimension=int(os.getenv("OCR_SERVICE_MAX_DIMENSION", "10000")),
        )


ERRORS = {
    "INVALID_IMAGE": (422, "Une image JPEG ou PNG valide est requise."),
    "IMAGE_TOO_LARGE": (413, "L’image dépasse les limites autorisées."),
    "OCR_TIMEOUT": (504, "Le délai du traitement OCR est dépassé."),
    "OCR_UNAVAILABLE": (503, "Le moteur OCR est indisponible."),
    "OCR_BUSY": (503, "La capacité OCR est momentanément occupée."),
}


def request_id(request: Request) -> str:
    value = request.headers.get("x-request-id", "")
    return (
        value if re.fullmatch(r"[A-Za-z0-9._:-]{1,128}", value) else str(uuid.uuid4())
    )


def failure(code: str, correlation: str) -> JSONResponse:
    code = code if code in ERRORS else "OCR_UNAVAILABLE"
    status, message = ERRORS[code]
    headers = {"X-Request-Id": correlation, "Cache-Control": "no-store"}
    if code == "OCR_BUSY":
        headers["Retry-After"] = "1"
    return JSONResponse(
        {"error": {"code": code, "message": message, "requestId": correlation}},
        status_code=status,
        headers=headers,
    )


async def upload(request: Request, settings: ServiceSettings) -> tuple[bytes, str]:
    if (
        not request.headers.get("content-type", "")
        .lower()
        .startswith("multipart/form-data;")
    ):
        raise OCRError("INVALID_IMAGE", "Multipart image required")
    # Cap the entire body before parsing, including uploads without Content-Length.
    limit = settings.ocr.max_bytes + 65536
    declared = request.headers.get("content-length")
    if declared:
        try:
            if int(declared) > limit:
                raise OCRError("IMAGE_TOO_LARGE", "Request too large")
            if int(declared) < 0:
                raise ValueError("Negative content length")
        except ValueError as exc:
            raise OCRError("INVALID_IMAGE", "Invalid content length") from exc
    body = bytearray()
    async for chunk in request.stream():
        if len(body) + len(chunk) > limit:
            raise OCRError("IMAGE_TOO_LARGE", "Request too large")
        body.extend(chunk)

    async def chunks() -> AsyncGenerator[bytes, None]:
        yield bytes(body)

    try:
        parser = MultiPartParser(
            Headers(request.headers),
            chunks(),
            max_files=1,
            max_fields=0,
            max_part_size=limit,
        )
        form = await parser.parse()
        try:
            if len(form.multi_items()) != 1:
                raise OCRError("INVALID_IMAGE", "One image required")
            image = form.get("image")
            if not isinstance(image, UploadFile) or image.content_type not in {
                "image/jpeg",
                "image/png",
            }:
                raise OCRError("INVALID_IMAGE", "Image MIME required")
            data = await image.read(settings.ocr.max_bytes + 1)
            if len(data) > settings.ocr.max_bytes:
                raise OCRError("IMAGE_TOO_LARGE", "Image too large")
            return data, image.content_type
        finally:
            await form.close()
    except (MultiPartException, ValueError) as exc:
        raise OCRError("INVALID_IMAGE", "Invalid multipart") from exc


def image_limits(data: bytes, mime: str, settings: ServiceSettings) -> None:
    signature = b"\x89PNG\r\n\x1a\n" if mime == "image/png" else b"\xff\xd8\xff"
    if not data.startswith(signature):
        raise OCRError("INVALID_IMAGE", "MIME and signature mismatch")
    try:
        with Image.open(io.BytesIO(data)) as image:
            if image.format != ("PNG" if mime == "image/png" else "JPEG"):
                raise OCRError("INVALID_IMAGE", "MIME and format mismatch")
            if (
                max(image.size) > settings.max_dimension
                or image.width * image.height > settings.ocr.max_pixels
            ):
                raise OCRError("IMAGE_TOO_LARGE", "Image dimensions too large")
    except (
        UnidentifiedImageError,
        OSError,
        ValueError,
        Image.DecompressionBombError,
    ) as exc:
        raise OCRError("INVALID_IMAGE", "Invalid image header") from exc


def response(result: Extraction, settings: ServiceSettings) -> dict[str, object]:
    selected = result.selected
    if len(selected.text_raw.encode("utf-8")) > settings.ocr.max_output_bytes:
        raise OCRError("OCR_UNAVAILABLE", "OCR result too large")
    return {
        "schemaVersion": 1,
        "engine": result.engine,
        "engineVersion": result.engine_version,
        "policyVersion": result.policy_version,
        "language": result.language,
        "textRaw": selected.text_raw,
        "textNormalized": normalize(selected.text_raw),
        "confidence": selected.confidence,
        "psm": selected.psm,
        "preprocessing": selected.preprocessing,
    }


def create_app(
    settings: ServiceSettings | None = None,
    extractor: Callable[[bytes, Settings], Extraction] | None = None,
    readiness: Callable[[float], str] | None = None,
) -> FastAPI:
    settings = settings or ServiceSettings.from_env()
    extractor = extractor or (lambda data, limits: extract(data, settings=limits))
    readiness = readiness or TesseractRunner().ready
    slots = threading.BoundedSemaphore(settings.concurrency)
    probe = threading.Lock()
    executor = ThreadPoolExecutor(
        max_workers=settings.concurrency, thread_name_prefix="ocr"
    )

    @asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        yield
        executor.shutdown(wait=True)

    app = FastAPI(lifespan=lifespan, docs_url=None, redoc_url=None, openapi_url=None)

    @app.get("/health/live")
    async def live() -> dict[str, str]:
        return {"status": "live"}

    @app.get("/health/ready")
    async def ready(request: Request) -> JSONResponse:
        correlation = request_id(request)
        if not probe.acquire(blocking=False):
            return failure("OCR_BUSY", correlation)
        try:
            version = await asyncio.to_thread(readiness, 2.0)
            return JSONResponse(
                {"status": "ready", "engineVersion": version},
                headers={"Cache-Control": "no-store"},
            )
        except Exception:
            return failure("OCR_UNAVAILABLE", correlation)
        finally:
            probe.release()

    @app.post("/v1/ocr")
    async def ocr(request: Request) -> JSONResponse:
        correlation = request_id(request)
        if not slots.acquire(blocking=False):
            return failure("OCR_BUSY", correlation)
        transferred = False
        try:
            data, mime = await asyncio.wait_for(
                upload(request, settings), settings.upload_timeout_seconds
            )

            def work() -> dict[str, object]:
                try:
                    image_limits(data, mime, settings)
                    return response(extractor(data, settings.ocr), settings)
                finally:
                    slots.release()

            future = asyncio.get_running_loop().run_in_executor(executor, work)
            transferred = True
            # A disconnected/cancelled caller must not release a still-running OCR slot.
            try:
                result = await asyncio.shield(future)
            except asyncio.CancelledError:
                # Consume a late failure without logging any extraction details.
                future.add_done_callback(
                    lambda done: None if done.cancelled() else done.exception()
                )
                raise
            return JSONResponse(
                result,
                headers={"X-Request-Id": correlation, "Cache-Control": "no-store"},
            )
        except TimeoutError:
            return failure("OCR_TIMEOUT", correlation)
        except OCRError as exc:
            return failure(exc.code, correlation)
        except Exception:
            return failure("OCR_UNAVAILABLE", correlation)
        finally:
            if not transferred:
                slots.release()

    return app


app = create_app()

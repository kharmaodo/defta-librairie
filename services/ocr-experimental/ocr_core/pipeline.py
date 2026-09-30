"""Bounded local Tesseract passes; confidence is OCR evidence, not certainty."""

from __future__ import annotations

import csv
import io
import math
import subprocess
import tempfile
import time
import warnings
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Protocol

from PIL import Image, ImageOps, UnidentifiedImageError


class OCRError(Exception):
    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code


@dataclass(frozen=True)
class Settings:
    timeout_seconds: float = 30
    max_bytes: int = 10 * 1024 * 1024
    max_pixels: int = 24_000_000
    max_output_bytes: int = 1024 * 1024

    def __post_init__(self) -> None:
        if (
            not math.isfinite(self.timeout_seconds)
            or min(
                self.timeout_seconds,
                self.max_bytes,
                self.max_pixels,
                self.max_output_bytes,
            )
            <= 0
        ):
            raise ValueError("OCR limits must be positive")
        if self.timeout_seconds > 120:
            raise ValueError("OCR timeout must not exceed 120 seconds")


@dataclass(frozen=True)
class PassResult:
    text_raw: str
    confidence: float | None
    psm: int
    preprocessing: str


@dataclass(frozen=True)
class Extraction:
    engine: str
    engine_version: str
    language: str
    policy_version: str
    baseline: PassResult
    selected: PassResult
    passes: tuple[PassResult, ...]
    duration_ms: float

    def to_dict(self) -> dict[str, object]:
        return asdict(self)


class Runner(Protocol):
    def ready(self, timeout: float) -> str: ...
    def read(self, image: Path, psm: int, timeout: float, max_output: int) -> str: ...


class TesseractRunner:
    def __init__(self, binary: str = "tesseract"):
        self.binary = binary

    def _run(self, args: list[str], timeout: float) -> subprocess.CompletedProcess[str]:
        try:
            result = subprocess.run(
                [self.binary, *args],
                capture_output=True,
                text=True,
                timeout=timeout,
                check=False,
            )
        except subprocess.TimeoutExpired as exc:
            raise OCRError("OCR_TIMEOUT", "Local OCR timed out") from exc
        except OSError as exc:
            raise OCRError(
                "OCR_UNAVAILABLE", "Tesseract executable is unavailable"
            ) from exc
        if result.returncode != 0:
            raise OCRError("OCR_UNAVAILABLE", "Tesseract execution failed")
        return result

    def ready(self, timeout: float) -> str:
        started = time.monotonic()
        version = self._run(["--version"], timeout).stdout.splitlines()
        remaining = timeout - (time.monotonic() - started)
        if remaining <= 0:
            raise OCRError("OCR_TIMEOUT", "Local OCR timed out")
        languages = self._run(["--list-langs"], remaining).stdout.splitlines()
        if not version or not version[0].startswith("tesseract 5."):
            raise OCRError("OCR_UNAVAILABLE", "Tesseract 5 is required")
        if "ara" not in {line.strip() for line in languages}:
            raise OCRError("OCR_UNAVAILABLE", "Tesseract Arabic data is unavailable")
        return version[0]

    def read(self, image: Path, psm: int, timeout: float, max_output: int) -> str:
        output = image.parent / "result"
        self._run(
            [
                str(image),
                str(output),
                "-l",
                "ara",
                "--oem",
                "1",
                "--psm",
                str(psm),
                "-c",
                "tessedit_create_tsv=1",
            ],
            timeout,
        )
        result = output.with_suffix(".tsv")
        try:
            if result.stat().st_size > max_output:
                raise OCRError("OCR_OUTPUT_TOO_LARGE", "OCR output exceeds its limit")
            return result.read_text(encoding="utf-8")
        except OSError as exc:
            raise OCRError(
                "OCR_UNAVAILABLE", "Tesseract output is unavailable"
            ) from exc


def parse_tsv(text: str, psm: int, preprocessing: str) -> PassResult:
    reader = csv.DictReader(io.StringIO(text), delimiter="\t")
    expected = {"level", "block_num", "par_num", "line_num", "conf", "text"}
    if not expected.issubset(set(reader.fieldnames or [])):
        raise OCRError("OCR_INVALID_OUTPUT", "Tesseract TSV header is invalid")
    lines: dict[tuple[str, str, str], list[str]] = {}
    weighted = 0.0
    weight = 0
    for row in reader:
        if row["level"] != "5":
            continue
        word = (row["text"] or "").strip()
        if not word:
            continue
        try:
            conf = float(row["conf"])
        except (ValueError, TypeError) as exc:
            raise OCRError(
                "OCR_INVALID_OUTPUT", "Tesseract confidence is invalid"
            ) from exc
        if not -1 <= conf <= 100:
            raise OCRError("OCR_INVALID_OUTPUT", "Tesseract confidence is out of range")
        key = (row["block_num"], row["par_num"], row["line_num"])
        lines.setdefault(key, []).append(word)
        if conf >= 0:
            weighted += conf * len(word)
            weight += len(word)
    raw = "\n".join(" ".join(words) for words in lines.values())
    return PassResult(
        raw, weighted / (100 * weight) if weight else None, psm, preprocessing
    )


def extract(
    data: bytes, runner: Runner | None = None, settings: Settings | None = None
) -> Extraction:
    settings = settings or Settings()
    runner = runner or TesseractRunner()
    started = time.monotonic()

    def remaining() -> float:
        value = settings.timeout_seconds - (time.monotonic() - started)
        if value <= 0:
            raise OCRError("OCR_TIMEOUT", "Local OCR timed out")
        return value

    if not data:
        raise OCRError("INVALID_IMAGE", "Image is empty")
    if len(data) > settings.max_bytes:
        raise OCRError("IMAGE_TOO_LARGE", "Image exceeds byte limit")
    try:
        with warnings.catch_warnings():
            warnings.simplefilter("error", Image.DecompressionBombWarning)
            with Image.open(io.BytesIO(data)) as source:
                if source.format not in {"JPEG", "PNG"}:
                    raise OCRError("INVALID_IMAGE", "Only JPEG and PNG are supported")
                if source.width * source.height > settings.max_pixels:
                    raise OCRError("IMAGE_TOO_LARGE", "Image exceeds pixel limit")
                source.load()
                oriented = ImageOps.exif_transpose(source)
                rgba = oriented.convert("RGBA")
                image = Image.new("RGB", rgba.size, "white")
                image.paste(rgba, mask=rgba.getchannel("A"))
    except (
        UnidentifiedImageError,
        OSError,
        ValueError,
        Image.DecompressionBombError,
        Image.DecompressionBombWarning,
    ) as exc:
        raise OCRError("INVALID_IMAGE", "Image cannot be decoded safely") from exc

    version = runner.ready(remaining())
    passes: list[PassResult] = []
    with tempfile.TemporaryDirectory(prefix="defta-ocr-") as folder:
        variants = [
            ("original", image),
            (
                "grayscale-autocontrast",
                ImageOps.autocontrast(ImageOps.grayscale(image)),
            ),
        ]
        for name, variant in variants:
            path = Path(folder) / "input.png"
            variant.save(path, format="PNG")
            for psm in (6, 11):
                tsv = runner.read(path, psm, remaining(), settings.max_output_bytes)
                remaining()
                if len(tsv.encode("utf-8")) > settings.max_output_bytes:
                    raise OCRError(
                        "OCR_OUTPUT_TOO_LARGE", "OCR output exceeds its limit"
                    )
                passes.append(parse_tsv(tsv, psm, name))
    # A diagnostic heuristic only. All passes remain available for comparison.
    selected = max(
        passes,
        key=lambda item: (
            bool(item.text_raw),
            item.confidence if item.confidence is not None else -1,
            sum(char.isalpha() for char in item.text_raw),
        ),
    )
    return Extraction(
        "tesseract-experimental",
        version,
        "ara",
        "ocr-local-v1",
        passes[0],
        selected,
        tuple(passes),
        (time.monotonic() - started) * 1000,
    )

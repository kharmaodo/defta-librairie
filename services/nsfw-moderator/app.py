import hashlib
import json
import os
from io import BytesIO
from pathlib import Path
from typing import Any

import numpy as np
import onnxruntime as ort
from fastapi import FastAPI, HTTPException, Request
from PIL import Image, UnidentifiedImageError

MAX_IMAGE_BYTES = int(os.getenv("NSFW_MAX_IMAGE_BYTES", "5242880"))
MODEL_PATH = Path(os.getenv("NSFW_MODEL_PATH", "/models/model.onnx"))
MANIFEST_PATH = Path(os.getenv("NSFW_MODEL_MANIFEST_PATH", "/models/model-manifest.json"))

app = FastAPI(docs_url=None, redoc_url=None)
session: ort.InferenceSession | None = None
manifest: dict[str, Any] | None = None


def fail(detail: str) -> RuntimeError:
    return RuntimeError(f"invalid moderation model configuration: {detail}")


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_model() -> tuple[ort.InferenceSession, dict[str, Any]]:
    if not MODEL_PATH.is_file() or not MANIFEST_PATH.is_file():
        raise fail("model or manifest is missing")
    try:
        loaded = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
        input_spec = loaded["input"]
        output_spec = loaded["output"]
        if loaded["sha256"].lower() != sha256(MODEL_PATH):
            raise fail("model sha256 mismatch")
        if input_spec["layout"] not in {"NCHW", "NHWC"}:
            raise fail("unsupported input layout")
        if input_spec.get("pixelRange", "0_1") not in {"0_1", "0_255"}:
            raise fail("unsupported input pixel range")
        if not isinstance(input_spec["width"], int) or not isinstance(input_spec["height"], int):
            raise fail("invalid input dimensions")
        if len(input_spec["mean"]) != 3 or len(input_spec["std"]) != 3:
            raise fail("mean and std must have three values")
        runtime = ort.InferenceSession(str(MODEL_PATH), providers=["CPUExecutionProvider"])
        if input_spec["name"] not in {item.name for item in runtime.get_inputs()}:
            raise fail("declared input is absent")
        if output_spec.get("name") and output_spec["name"] not in {item.name for item in runtime.get_outputs()}:
            raise fail("declared output is absent")
        return runtime, loaded
    except (KeyError, TypeError, ValueError, json.JSONDecodeError) as error:
        raise fail(str(error)) from error


@app.on_event("startup")
def startup() -> None:
    global session, manifest
    session, manifest = load_model()


@app.get("/health/live")
def live() -> dict[str, str]:
    return {"status": "alive"}


@app.get("/health/ready")
def ready() -> dict[str, str]:
    if session is None or manifest is None:
        raise HTTPException(status_code=503, detail="model_unavailable")
    return {"status": "ready", "modelVersion": manifest["modelVersion"]}


def input_tensor(image: Image.Image, spec: dict[str, Any]) -> np.ndarray:
    resized = image.convert("RGB").resize((spec["width"], spec["height"]), Image.Resampling.BILINEAR)
    pixels = np.asarray(resized, dtype=np.float32)
    if spec.get("pixelRange", "0_1") == "0_1":
        pixels /= 255.0
    mean = np.asarray(spec["mean"], dtype=np.float32)
    std = np.asarray(spec["std"], dtype=np.float32)
    if np.any(std == 0):
        raise fail("std cannot contain zero")
    pixels = (pixels - mean) / std
    if spec["layout"] == "NCHW":
        pixels = np.transpose(pixels, (2, 0, 1))
    return np.expand_dims(pixels, axis=0)


@app.post("/v1/moderate")
async def moderate(request: Request) -> dict[str, Any]:
    if session is None or manifest is None:
        raise HTTPException(status_code=503, detail="model_unavailable")
    if request.headers.get("content-type", "").split(";", 1)[0] not in {"image/jpeg", "image/png"}:
        raise HTTPException(status_code=415, detail="jpeg_or_png_required")
    body = await request.body()
    if not body or len(body) > MAX_IMAGE_BYTES:
        raise HTTPException(status_code=413, detail="invalid_image_size")
    try:
        with Image.open(BytesIO(body)) as image:
            image.load()
            tensor = input_tensor(image, manifest["input"])
    except (UnidentifiedImageError, OSError, ValueError) as error:
        raise HTTPException(status_code=400, detail="invalid_image") from error
    output_name = manifest["output"].get("name")
    values = session.run([output_name] if output_name else None, {manifest["input"]["name"]: tensor})[0]
    probabilities = np.asarray(values, dtype=np.float32).reshape(-1)
    labels = manifest["output"].get("labels")
    if labels:
        if len(labels) != probabilities.size or not all(isinstance(label, str) for label in labels):
            raise HTTPException(status_code=500, detail="invalid_model_output")
        scores = dict(zip(labels, probabilities.tolist()))
        unsafe_score = max(float(scores.get("NSFW", 0)), float(scores.get("NSFL", 0)))
        confidence = max(float(value) for value in probabilities)
        decision = "UNSAFE" if unsafe_score >= 0.75 else "SAFE" if scores.get("SFW", 0) >= 0.75 else "REVIEW"
        return {"class": decision, "score": unsafe_score, "modelVersion": manifest["modelVersion"]}
    index = manifest["output"]["nsfwIndex"]
    if not isinstance(index, int) or index < 0 or index >= probabilities.size:
        raise HTTPException(status_code=500, detail="invalid_model_output")
    score = float(probabilities[index])
    if not 0.0 <= score <= 1.0:
        raise HTTPException(status_code=500, detail="invalid_model_score")
    decision = "SAFE" if score < 0.20 else "REVIEW" if score < 0.75 else "UNSAFE"
    return {"class": decision, "score": score, "modelVersion": manifest["modelVersion"]}

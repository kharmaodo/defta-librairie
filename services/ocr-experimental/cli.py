"""Diagnostic extraction, baseline comparison and library-scoped candidates."""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import os
import sqlite3
import sys
from pathlib import Path

from ocr_core.catalogue import Catalogue, normalize
from ocr_core.pipeline import OCRError, Settings, TesseractRunner, extract


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Local Arabic OCR comparison; catalogue is read-only"
    )
    parser.add_argument(
        "target", type=Path, help="JPEG/PNG file or directory (maximum 100 images)"
    )
    parser.add_argument(
        "--db", required=True, type=Path, help="Consistent copy of existing defta.db"
    )
    parser.add_argument("--library-id", required=True)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--format", choices=("json", "csv"), default="json")
    parser.add_argument("--timeout-seconds", type=float, default=30)
    parser.add_argument("--tesseract", default="tesseract")
    args = parser.parse_args(argv)
    try:
        settings = Settings(timeout_seconds=args.timeout_seconds)
        if args.target.is_dir():
            paths = sorted(
                path
                for path in args.target.iterdir()
                if path.is_file() and path.suffix.lower() in {".jpg", ".jpeg", ".png"}
            )
        else:
            paths = [args.target]
        if not 1 <= len(paths) <= 100:
            raise ValueError("Provide 1 to 100 images")
        output = args.output.resolve()
        if output in {args.db.resolve(), *(path.resolve() for path in paths)}:
            raise ValueError("Output must not replace the database or an input image")
        records: list[dict[str, object]] = []
        failed = False
        with Catalogue(args.db, args.library_id) as catalogue:
            runner = TesseractRunner(args.tesseract)
            for path in paths:
                record: dict[str, object] = {
                    "file": path.name,
                    "library_id": args.library_id,
                }
                try:
                    # Read at most the limit + 1, including files changing during read.
                    with path.open("rb") as source:
                        data = source.read(settings.max_bytes + 1)
                    result = extract(data, runner, settings)
                    record.update(result.to_dict())
                    record["sha256"] = hashlib.sha256(data).hexdigest()
                    record["text_normalized"] = normalize(result.selected.text_raw)
                    record["baseline_candidates"] = catalogue.search(
                        result.baseline.text_raw
                    )
                    record["candidates"] = catalogue.search(result.selected.text_raw)
                except (OCRError, OSError) as exc:
                    failed = True
                    record["error"] = (
                        exc.code if isinstance(exc, OCRError) else "INPUT_UNAVAILABLE"
                    )
                records.append(record)
        # Private diagnostic output contains OCR text; never write it to logs.
        descriptor = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "w", encoding="utf-8", newline="") as destination:
            if args.format == "json":
                json.dump(
                    {"schema_version": 1, "results": records},
                    destination,
                    ensure_ascii=False,
                    indent=2,
                    allow_nan=False,
                )
                destination.write("\n")
            else:
                writer = csv.DictWriter(
                    destination,
                    fieldnames=[
                        "file",
                        "library_id",
                        "sha256",
                        "text_normalized",
                        "baseline",
                        "selected",
                        "baseline_candidates",
                        "candidates",
                        "duration_ms",
                        "error",
                    ],
                    extrasaction="ignore",
                )
                writer.writeheader()
                for record in records:
                    writer.writerow(
                        {
                            key: json.dumps(value, ensure_ascii=False)
                            if isinstance(value, (dict, list))
                            else value
                            for key, value in record.items()
                        }
                    )
        print(
            f"Processed {len(records)} image(s); failures: {sum('error' in r for r in records)}"
        )
        return 1 if failed else 0
    except (ValueError, OSError, sqlite3.Error) as exc:
        # Do not echo database contents, image paths or backend exception details.
        print(
            f"Diagnostic configuration failed ({type(exc).__name__})", file=sys.stderr
        )
        return 2


if __name__ == "__main__":
    raise SystemExit(main())

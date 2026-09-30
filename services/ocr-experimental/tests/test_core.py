from __future__ import annotations

import contextlib
import hashlib
import io
import json
import sqlite3
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from PIL import Image, ImageDraw, ImageFont

from cli import main
from ocr_core.catalogue import Catalogue, normalize, query_tokens
from ocr_core.pipeline import OCRError, Settings, TesseractRunner, extract, parse_tsv


HEADER = "level\tblock_num\tpar_num\tline_num\tconf\ttext\n"


def tsv(confidence: int = 80) -> str:
    return HEADER + f"5\t1\t1\t1\t{confidence}\tكتاب\n5\t1\t1\t1\t60\tعربي\n"


def image_bytes() -> bytes:
    data = io.BytesIO()
    Image.new("RGB", (40, 30), "white").save(data, "PNG")
    return data.getvalue()


class FakeRunner:
    def __init__(self) -> None:
        self.calls: list[tuple[Path, int]] = []
        self.error: OCRError | None = None

    def ready(self, timeout: float) -> str:
        return "tesseract 5.test"

    def read(self, image: Path, psm: int, timeout: float, max_output: int) -> str:
        self.calls.append((image, psm))
        if self.error:
            raise self.error
        return tsv(90 if psm == 11 else 60)


class PipelineTests(unittest.TestCase):
    def test_measured_confidence_and_unicode_order(self) -> None:
        result = parse_tsv(tsv(), 6, "original")
        self.assertEqual(result.text_raw, "كتاب عربي")
        self.assertAlmostEqual(result.confidence or 0, 0.7)
        self.assertIsNone(parse_tsv(HEADER, 6, "original").confidence)

    def test_invalid_tsv_and_confidence(self) -> None:
        for value in (
            "broken",
            HEADER + "5\t1\t1\t1\tnan\tكتاب\n",
            HEADER + "5\t1\t1\t1\t101\tكتاب\n",
        ):
            with self.subTest(value=value), self.assertRaises(OCRError):
                parse_tsv(value, 6, "original")

    def test_pass_comparison_and_cleanup(self) -> None:
        runner = FakeRunner()
        result = extract(image_bytes(), runner)
        self.assertEqual(len(result.passes), 4)
        self.assertEqual(result.baseline.psm, 6)
        self.assertEqual(result.baseline.preprocessing, "original")
        self.assertEqual(result.selected.psm, 11)
        self.assertLess(result.selected.confidence or 0, 0.95)
        self.assertTrue(all(not path.parent.exists() for path, _ in runner.calls))

    def test_invalid_and_oversized_images_before_runner(self) -> None:
        runner = FakeRunner()
        cases = [
            (b"", Settings()),
            (b"invalid", Settings()),
            (image_bytes(), Settings(max_bytes=1)),
            (image_bytes(), Settings(max_pixels=1)),
        ]
        for data, settings in cases:
            with self.subTest(settings=settings), self.assertRaises(OCRError):
                extract(data, runner, settings)
        self.assertEqual(runner.calls, [])

    def test_wrong_format(self) -> None:
        data = io.BytesIO()
        Image.new("RGB", (4, 4)).save(data, "GIF")
        with self.assertRaises(OCRError) as failure:
            extract(data.getvalue(), FakeRunner())
        self.assertEqual(failure.exception.code, "INVALID_IMAGE")

    def test_failure_cleans_temporary_images(self) -> None:
        runner = FakeRunner()
        runner.error = OCRError("OCR_UNAVAILABLE", "offline")
        with self.assertRaises(OCRError):
            extract(image_bytes(), runner)
        self.assertFalse(runner.calls[0][0].parent.exists())

    def test_global_timeout_is_not_reset_per_pass(self) -> None:
        clock = [0.0]
        runner = FakeRunner()
        original = runner.read

        def delayed(image: Path, psm: int, timeout: float, max_output: int) -> str:
            clock[0] += 2
            return original(image, psm, timeout, max_output)

        with (
            patch("ocr_core.pipeline.time.monotonic", side_effect=lambda: clock[0]),
            patch.object(runner, "read", side_effect=delayed),
        ):
            with self.assertRaises(OCRError) as failure:
                extract(image_bytes(), runner, Settings(timeout_seconds=1))
        self.assertEqual(failure.exception.code, "OCR_TIMEOUT")
        self.assertEqual(len(runner.calls), 1)
        self.assertFalse(runner.calls[0][0].parent.exists())

    def test_output_limit(self) -> None:
        with self.assertRaises(OCRError) as failure:
            extract(image_bytes(), FakeRunner(), Settings(max_output_bytes=10))
        self.assertEqual(failure.exception.code, "OCR_OUTPUT_TOO_LARGE")

    def test_unavailable_binary_and_timeout(self) -> None:
        with patch("subprocess.run", side_effect=FileNotFoundError):
            with self.assertRaises(OCRError) as failure:
                TesseractRunner().ready(1)
        self.assertEqual(failure.exception.code, "OCR_UNAVAILABLE")
        with patch(
            "subprocess.run", side_effect=subprocess.TimeoutExpired("tesseract", 1)
        ):
            with self.assertRaises(OCRError) as failure:
                TesseractRunner().ready(1)
        self.assertEqual(failure.exception.code, "OCR_TIMEOUT")

    def test_missing_arabic_data(self) -> None:
        values = [
            subprocess.CompletedProcess([], 0, "tesseract 5.0\n", ""),
            subprocess.CompletedProcess([], 0, "eng\nosd\n", ""),
        ]
        with patch("subprocess.run", side_effect=values), self.assertRaises(OCRError):
            TesseractRunner().ready(1)

    def test_real_tesseract_arabic(self) -> None:
        # Synthetic fixture only: verifies executable/ara/TSV, not cover quality.
        data = io.BytesIO()
        image = Image.new("RGB", (700, 150), "white")
        font = ImageFont.truetype("DejaVuSans.ttf", 64)
        ImageDraw.Draw(image).text((40, 30), "كتاب العربية", font=font, fill="black")
        image.save(data, "PNG")
        result = extract(data.getvalue())
        self.assertTrue(result.engine_version.startswith("tesseract 5."))
        self.assertTrue(
            any("\u0600" <= char <= "\u06ff" for char in result.selected.text_raw)
        )
        self.assertEqual(len(result.passes), 4)


class CatalogueTests(unittest.TestCase):
    def setUp(self) -> None:
        self.folder = tempfile.TemporaryDirectory()
        self.addCleanup(self.folder.cleanup)
        self.path = Path(self.folder.name) / "defta.db"
        with sqlite3.connect(self.path) as db:
            db.executescript("""
                CREATE TABLE libraries(id TEXT PRIMARY KEY,status TEXT);
                CREATE TABLE defta(id INTEGER PRIMARY KEY,library_id TEXT,title TEXT,
                                   editeur TEXT,auteur TEXT,tags TEXT,categorie TEXT,deleted_at TEXT);
                CREATE VIRTUAL TABLE defta_fts USING fts5(title,editeur,auteur,tags,categorie);
                INSERT INTO libraries VALUES('a','ACTIVE'),('b','ACTIVE'),('disabled','DISABLED');
                INSERT INTO defta VALUES(1,'a','كتاب عربي','','','','',NULL);
                INSERT INTO defta VALUES(2,'b','كتاب عربي','','','','',NULL);
                INSERT INTO defta VALUES(3,'a','كتاب عربي','','','','','deleted');
                INSERT INTO defta_fts(rowid,title) VALUES(1,'كتاب عربي'),(2,'كتاب عربي'),(3,'كتاب عربي');
            """)

    def test_scope_deleted_and_read_only(self) -> None:
        before = hashlib.sha256(self.path.read_bytes()).hexdigest()
        with Catalogue(self.path, "a") as catalogue:
            result = catalogue.search("كتاب عربي")
            self.assertEqual([row["book_id"] for row in result], [1])
            with self.assertRaises(sqlite3.OperationalError):
                catalogue.connection.execute("DELETE FROM defta")
        self.assertEqual(hashlib.sha256(self.path.read_bytes()).hexdigest(), before)

    def test_missing_database_not_created(self) -> None:
        missing = self.path.parent / "missing.db"
        with self.assertRaises(sqlite3.OperationalError):
            Catalogue(missing, "a")
        self.assertFalse(missing.exists())

    def test_missing_or_disabled_scope(self) -> None:
        for scope in ("", "foreign", "disabled"):
            with self.subTest(scope=scope), self.assertRaises(ValueError):
                Catalogue(self.path, scope)

    def test_tokens_after_noise_and_operator_safety(self) -> None:
        text = " ".join(str(i) for i in range(30)) + ' " OR * : كتاب عربي'
        with Catalogue(self.path, "a") as catalogue:
            self.assertEqual([r["book_id"] for r in catalogue.search(text)], [1])
            self.assertEqual(catalogue.search("1 2 3"), [])
        self.assertEqual(normalize("الْكِتَابُــ"), "الكتاب")
        self.assertLessEqual(
            len(query_tokens(" ".join(f"word{i}" for i in range(100)))), 64
        )

    def test_candidate_limit_and_order(self) -> None:
        with sqlite3.connect(self.path) as db:
            for book_id in range(4, 12):
                db.execute(
                    "INSERT INTO defta(id,library_id,title) VALUES(?,'a','كتاب عربي')",
                    (book_id,),
                )
                db.execute(
                    "INSERT INTO defta_fts(rowid,title) VALUES(?,'كتاب عربي')",
                    (book_id,),
                )
        with Catalogue(self.path, "a") as catalogue:
            result = catalogue.search("كتاب عربي")
        self.assertEqual(len(result), 5)
        self.assertEqual([row["rank"] for row in result], [1, 2, 3, 4, 5])
        self.assertEqual([row["book_id"] for row in result], [1, 4, 5, 6, 7])

    def test_cli_json_csv_failure_and_no_overwrite(self) -> None:
        target = self.path.parent / "cover.png"
        target.write_bytes(image_bytes())
        result = extract(image_bytes(), FakeRunner())
        for fmt in ("json", "csv"):
            output = self.path.parent / f"result.{fmt}"
            args = [
                str(target),
                "--db",
                str(self.path),
                "--library-id",
                "a",
                "--output",
                str(output),
                "--format",
                fmt,
            ]
            with (
                patch("cli.extract", return_value=result),
                contextlib.redirect_stdout(io.StringIO()),
            ):
                self.assertEqual(main(args), 0)
            if fmt == "json":
                payload = json.loads(output.read_text())
                self.assertEqual(payload["results"][0]["candidates"][0]["book_id"], 1)
            else:
                self.assertIn("baseline_candidates", output.read_text())
            before = output.read_bytes()
            with (
                patch("cli.extract", return_value=result),
                contextlib.redirect_stderr(io.StringIO()),
            ):
                self.assertEqual(main(args), 2)
            self.assertEqual(output.read_bytes(), before)
        args = [
            str(target),
            "--db",
            str(self.path),
            "--library-id",
            "a",
            "--output",
            str(self.path.parent / "failure.json"),
        ]
        with (
            patch("cli.extract", side_effect=OCRError("OCR_TIMEOUT", "timeout")),
            contextlib.redirect_stdout(io.StringIO()),
        ):
            self.assertEqual(main(args), 1)
        self.assertIn("OCR_TIMEOUT", (self.path.parent / "failure.json").read_text())


if __name__ == "__main__":
    unittest.main()

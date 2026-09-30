"""Existing SQLite catalogue read-only, scoped before matching."""

from __future__ import annotations

import sqlite3
import unicodedata
from pathlib import Path


def normalize(text: str) -> str:
    out = []
    for char in text:
        if char == "\u0640" or unicodedata.category(char) == "Mn":
            continue
        out.append(char if char.isalnum() else " ")
    return " ".join("".join(out).split())


def query_tokens(text: str) -> list[str]:
    # Inspect the whole text before applying a bound, unlike the legacy first 12.
    words = normalize(text).split()
    useful = dict.fromkeys(
        word for word in words if len(word) >= 2 and any(c.isalpha() for c in word)
    )
    return [word[:64] for word in list(useful)[:64]]


class Catalogue:
    def __init__(self, path: Path, library_id: str):
        if not library_id.strip():
            raise ValueError("library_id is required")
        self.library_id = library_id
        self.connection = sqlite3.connect(
            path.resolve().as_uri() + "?mode=ro", uri=True
        )
        self.connection.row_factory = sqlite3.Row
        try:
            self.connection.execute("PRAGMA query_only=ON")
            row = self.connection.execute(
                "SELECT status FROM libraries WHERE id=?",
                (library_id,),
            ).fetchone()
            if row is None or row["status"] != "ACTIVE":
                raise ValueError("library_id must identify an active library")
        except Exception:
            self.connection.close()
            raise

    def __enter__(self) -> Catalogue:
        return self

    def __exit__(self, *_: object) -> None:
        self.connection.close()

    def search(self, text: str) -> list[dict[str, object]]:
        tokens = query_tokens(text)
        if not tokens:
            return []
        # Quoted literal tokens cannot introduce FTS operators.
        query = " OR ".join('"' + token.replace('"', "") + '"' for token in tokens)
        rows = self.connection.execute(
            """SELECT d.id AS book_id,d.title,bm25(defta_fts,5.0,1.0,2.0,0.5,0.5) AS fts_score
               FROM defta_fts JOIN defta d ON d.id=defta_fts.rowid
               WHERE defta_fts MATCH ? AND d.library_id=? AND d.deleted_at IS NULL
               ORDER BY fts_score ASC,d.id ASC LIMIT 5""",
            (query, self.library_id),
        ).fetchall()
        return [dict(row) | {"rank": rank} for rank, row in enumerate(rows, 1)]

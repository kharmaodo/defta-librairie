CREATE TABLE book_tags (
    book_id INTEGER NOT NULL,
    tag_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (book_id, tag_id),
    FOREIGN KEY (book_id) REFERENCES defta(id),
    FOREIGN KEY (tag_id) REFERENCES library_tags(id)
);

CREATE INDEX idx_book_tags_tag ON book_tags(tag_id, book_id);

WITH RECURSIVE split(book_id, library_id, value, remaining, created_at) AS (
    SELECT id, library_id, '', trim(COALESCE(tags, '')) || ',',
           COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
    FROM defta
    WHERE trim(COALESCE(tags, '')) <> ''
    UNION ALL
    SELECT book_id, library_id,
           trim(substr(remaining, 1, instr(remaining, ',') - 1)),
           substr(remaining, instr(remaining, ',') + 1),
           created_at
    FROM split
    WHERE remaining <> ''
),
normalized AS (
    SELECT DISTINCT library_id, value AS name, lower(value) AS normalized_name, created_at
    FROM split
    WHERE value <> ''
)
INSERT OR IGNORE INTO library_tags(id, library_id, name, normalized_name, created_at, updated_at)
SELECT 'legacy-' || library_id || '-' || normalized_name,
       library_id, name, normalized_name, created_at, created_at
FROM normalized;

WITH RECURSIVE split(book_id, library_id, value, remaining, created_at) AS (
    SELECT id, library_id, '', trim(COALESCE(tags, '')) || ',',
           COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
    FROM defta
    WHERE trim(COALESCE(tags, '')) <> ''
    UNION ALL
    SELECT book_id, library_id,
           trim(substr(remaining, 1, instr(remaining, ',') - 1)),
           substr(remaining, instr(remaining, ',') + 1),
           created_at
    FROM split
    WHERE remaining <> ''
)
INSERT OR IGNORE INTO book_tags(book_id, tag_id, created_at)
SELECT split.book_id, tags.id, split.created_at
FROM split
JOIN library_tags tags
  ON tags.library_id = split.library_id
 AND tags.normalized_name = lower(split.value)
WHERE split.value <> '';

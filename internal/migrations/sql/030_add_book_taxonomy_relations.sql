ALTER TABLE defta ADD COLUMN publisher_id INTEGER REFERENCES publishers(id);

CREATE TABLE book_categories (
    book_id INTEGER NOT NULL,
    category_id INTEGER NOT NULL,
    is_primary INTEGER NOT NULL DEFAULT 0 CHECK (is_primary IN (0, 1)),
    created_at TEXT NOT NULL,
    PRIMARY KEY (book_id, category_id),
    FOREIGN KEY (book_id) REFERENCES defta(id),
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

CREATE UNIQUE INDEX idx_book_categories_primary
    ON book_categories(book_id) WHERE is_primary = 1;

CREATE INDEX idx_book_categories_category
    ON book_categories(category_id, book_id);

INSERT OR IGNORE INTO book_categories(book_id, category_id, is_primary, created_at)
SELECT d.id, c.id, 1, COALESCE(d.updated_at, d.created_at)
FROM defta d
JOIN categories c ON trim(COALESCE(d.categorie, '')) <> ''
 AND (
    trim(d.categorie) = CAST(c.id AS TEXT)
    OR lower(trim(d.categorie)) = c.code
    OR lower(trim(d.categorie)) = lower(c.categoriename)
 );

UPDATE defta
SET publisher_id = (
    SELECT p.id FROM publishers p
    WHERE lower(trim(p.fullname)) = lower(trim(defta.editeur))
       OR p.code = lower(trim(defta.editeur))
    LIMIT 1
)
WHERE publisher_id IS NULL AND trim(COALESCE(editeur, '')) <> '';

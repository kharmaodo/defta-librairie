CREATE TABLE IF NOT EXISTS defta (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    price INTEGER,
    title TEXT NOT NULL,
    editeur TEXT,
    tags TEXT,
    categorie TEXT,
    status TEXT,
    volume INTEGER DEFAULT 1,
    auteur TEXT,
    coverUrl TEXT
);

CREATE VIRTUAL TABLE IF NOT EXISTS defta_fts USING fts5(
    title,
    editeur,
    auteur,
    tags,
    categorie,
    content='defta',
    content_rowid='id'
);

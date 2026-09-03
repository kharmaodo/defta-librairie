CREATE TABLE library_tags (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON UPDATE CASCADE ON DELETE CASCADE,
    UNIQUE (library_id, normalized_name)
);

CREATE INDEX idx_library_tags_library_name
ON library_tags(library_id, normalized_name);

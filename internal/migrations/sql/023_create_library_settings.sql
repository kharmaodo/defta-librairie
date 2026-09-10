CREATE TABLE library_settings (
 library_id TEXT PRIMARY KEY REFERENCES libraries(id) ON DELETE CASCADE,
 currency TEXT NOT NULL DEFAULT 'XOF' CHECK(currency='XOF'),
 address TEXT NOT NULL DEFAULT '',
 phone TEXT NOT NULL DEFAULT '',
 email TEXT NOT NULL DEFAULT '',
 logo_data TEXT NOT NULL DEFAULT '',
 default_low_stock_threshold INTEGER NOT NULL DEFAULT 5 CHECK(default_low_stock_threshold BETWEEN 0 AND 1000000),
 print_footer TEXT NOT NULL DEFAULT '',
 version INTEGER NOT NULL DEFAULT 1 CHECK(version>=1),
 updated_at TEXT NOT NULL
);
INSERT INTO library_settings(library_id,updated_at) SELECT id,updated_at FROM libraries;
CREATE TRIGGER initialize_library_settings AFTER INSERT ON libraries BEGIN
 INSERT INTO library_settings(library_id,updated_at) VALUES(NEW.id,NEW.updated_at);
END;

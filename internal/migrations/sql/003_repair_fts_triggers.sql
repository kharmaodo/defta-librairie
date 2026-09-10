DROP TRIGGER IF EXISTS defta_ai;
DROP TRIGGER IF EXISTS defta_ad;
DROP TRIGGER IF EXISTS defta_au;

CREATE TRIGGER defta_ai AFTER INSERT ON defta BEGIN
    INSERT INTO defta_fts(rowid, title, editeur, auteur, tags, categorie)
    VALUES (new.id, new.title, new.editeur, new.auteur, new.tags, new.categorie);
END;

CREATE TRIGGER defta_ad AFTER DELETE ON defta BEGIN
    INSERT INTO defta_fts(defta_fts, rowid, title, editeur, auteur, tags, categorie)
    VALUES ('delete', old.id, old.title, old.editeur, old.auteur, old.tags, old.categorie);
END;

CREATE TRIGGER defta_au AFTER UPDATE ON defta BEGIN
    INSERT INTO defta_fts(defta_fts, rowid, title, editeur, auteur, tags, categorie)
    VALUES ('delete', old.id, old.title, old.editeur, old.auteur, old.tags, old.categorie);
    INSERT INTO defta_fts(rowid, title, editeur, auteur, tags, categorie)
    VALUES (new.id, new.title, new.editeur, new.auteur, new.tags, new.categorie);
END;

INSERT INTO defta_fts(defta_fts) VALUES ('rebuild');

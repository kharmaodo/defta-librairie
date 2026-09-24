CREATE TABLE IF NOT EXISTS categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    categoriename TEXT NOT NULL UNIQUE,
    ar TEXT,
    fr TEXT,
    en TEXT
);
ALTER TABLE categories ADD COLUMN code TEXT;
ALTER TABLE categories ADD COLUMN active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1));
ALTER TABLE categories ADD COLUMN created_at TEXT;
ALTER TABLE categories ADD COLUMN updated_at TEXT;
CREATE UNIQUE INDEX idx_categories_code ON categories(code);

-- Compatibility source for installations using the historical singular table.
CREATE TABLE IF NOT EXISTS editeur (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    fullname TEXT NOT NULL UNIQUE,
    ar TEXT,
    fr TEXT,
    en TEXT
);
CREATE TABLE IF NOT EXISTS publishers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    fullname TEXT NOT NULL UNIQUE,
    ar TEXT,
    fr TEXT,
    en TEXT
);
ALTER TABLE publishers ADD COLUMN code TEXT;
ALTER TABLE publishers ADD COLUMN active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1));
ALTER TABLE publishers ADD COLUMN created_at TEXT;
ALTER TABLE publishers ADD COLUMN updated_at TEXT;
CREATE UNIQUE INDEX idx_publishers_code ON publishers(code);
INSERT OR IGNORE INTO publishers(fullname, ar, fr, en)
SELECT fullname, COALESCE(ar, ''), COALESCE(fr, ''), COALESCE(en, '') FROM editeur;

INSERT INTO categories(code, categoriename, ar, fr, en) VALUES
('sira','sira','السيرة','Sîra (Biographie prophétique)','Sira (Prophetic Biography)'),
('fiqh','fqh','الفقه','Fiqh (Jurisprudence)','Fiqh (Jurisprudence)'),
('nahw','nahw','النحو','Nahw (Syntaxe)','Nahw (Syntax/Grammar)'),
('tasawwuf','tassawuf','التصوف','Tasawwuf (Soufisme)','Tasawwuf (Sufism)'),
('lugha','lugha','اللغة','Lugha (Langue/Linguistique)','Lugha (Language/Linguistics)'),
('mantiq','mantiq','المنطق','Mantiq (Logique)','Mantiq (Logic)'),
('arud','arud','العروض','Arud (Prosodie)','Arud (Prosody)'),
('handassa','handassa','الهندسة','Handassa (Géométrie)','Handassa (Geometry)'),
('hadith','hadiss','الحديث','Hadith (Traditions prophétiques)','Hadith (Prophetic Traditions)'),
('radd','radd','الرد','Radd (Réfutation)','Radd (Refutation)'),
('sarf','sarf','الصرف','Sarf (Morphologie)','Sarf (Morphology)'),
('mirath','mirass','الميراث','Mirath (Droit des successions)','Mirath (Inheritance Law)'),
('ulum-al-quran','ulum al qur''an','علوم القرآن','Sciences du Coran','Sciences of the Qur''an'),
('ilm-al-hadith','ilm al-hadith','علم الحديث','Science du Hadith','Science of Hadith'),
('asbab-al-nuzul','asbab al nuzul','أسباب النزول','Circonstances de la révélation','Occasions of revelation'),
('nasikh-wa-al-mansukh','nasik wa al-mansukh','الناسخ والمنسوخ','L''abrogé et l''abrogeant','Abrogation (Nasikh and Mansukh)'),
('asbab-wurud-al-hadith','asbab wurul al-hadith','أسباب ورود الحديث','Circonstances de l''énonciation du Hadith','Occasions of Hadith narration'),
('ilm-sharh-al-hadith','ilm sharh al-hadith','علم شرح الحديث','Science du commentaire du Hadith','Science of Hadith commentary'),
('mustalah-al-hadith','mustalah al-hadith','مصطلح الحديث','Terminologie du Hadith','Hadith terminology'),
('usul-al-fiqh','usul al-fiqh','أصول الفقه','Fondements du droit','Principles of jurisprudence'),
('balagha','balagha','البلاغة','Balagha (Rhétorique)','Balagha (Rhetoric)'),
('tajwid','tajwid','التجويد','Tajwid (Règles de récitation)','Tajwid (Recitation rules)'),
('ilm-al-rijal','ilm al-rijal','علم الرجال','Science des hommes (Biographies)','Science of Men (Biographical evaluation)'),
('tarikh','tarikh','التاريخ','Histoire','History'),
('aqidah','aqidah','العقيدة','Aqida (Dogme/Croyance)','Aqidah (Creed/Theology)'),
('adab-al-muluk','adab al-muluk','أدب الملوك','Éthique des dirigeants','Ethics of rulers'),
('adab','adab','الأدب','Littérature / Éthique','Literature / Etiquette'),
('mawsuaat','mawsu''aat','موسوعات','Encyclopédies','Encyclopedias'),
('nawazil','nawazil','نوازل','Nawazil (Cas juridiques contemporains)','Nawazil (Contemporary legal cases)')
ON CONFLICT(categoriename) DO UPDATE SET code=excluded.code, ar=excluded.ar, fr=excluded.fr, en=excluded.en;

INSERT INTO publishers(code, fullname, ar, fr, en) VALUES
('dar-ibn-hazm','dar ibn hazm','دار ابن حزم','dar ibn hazm','dar ibn hazm'),
('dar-al-asriyya','dar al asriyya','دار العصرية','dar al asriyya','dar al asriyya'),
('dar-al-fikr','dar al fikr','دار الفكر','dar al fikr','dar al fikr'),
('dar-al-hadith','dar al hadith','دار الحديث','dar al hadith','dar al hadith'),
('dar-al-haramayn','dar al haramayn','دار الحرمين','dar al haramayn','dar al haramayn'),
('orientica','orientica','أورينتيكا','orientica','orientica'),
('dar-at-tawhid','dar at-tawhid','دار التوحيد','dar at-tawhid','dar at-tawhid'),
('dar-at-tawbah','dar at-tawbah','دار التوبة','dar at-tawbah','dar at-tawbah'),
('sana','sana','سناء','sana','sana'),
('dar-ibn-badiss','dar ibn badiss','دار ابن باديس','dar ibn badiss','dar ibn badiss'),
('dar-ibn-imam','dar ibn imam','دار ابن إمام','dar ibn imam','dar ibn imam'),
('dar-al-bayyina','dar al bayyina','دار البينة','dar al bayyina','dar al bayyina'),
('al-bouraq','al bouraq','البراق','al bouraq','al bouraq'),
('al-madina','al madina','المدينة','al madina','al madina')
ON CONFLICT(fullname) DO UPDATE SET code=excluded.code, ar=excluded.ar, fr=excluded.fr, en=excluded.en;
